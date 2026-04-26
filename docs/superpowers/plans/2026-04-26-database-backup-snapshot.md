# Database Backup & Snapshot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backup, scheduled snapshot, export, and restore functionality for both SQLite and MariaDB, operable via panel UI and x-ui.sh CLI.

**Architecture:** New `BackupService` in web/service handles mysqldump/sqlite3 dump execution, tar.gz packaging, and file management. `BackupController` exposes REST API endpoints. `BackupJob` is a cron-scheduled job that delegates to BackupService. Settings stored via existing `SettingService`/`AllSetting` reflection-based persistence. CLI subcommands in main.go call the same backup/restore logic directly. x-ui.sh wraps CLI commands.

**Tech Stack:** Go (Gin, GORM, exec.Command), bash (x-ui.sh), Vue.js + Ant Design Vue (panel UI), robfig/cron v3 (scheduling)

---

### Task 1: Add backup settings to entity and setting service

**Files:**
- Modify: `web/entity/entity.go:113-124`
- Modify: `web/service/setting.go:113-130` (add defaults)
- Modify: `web/service/setting.go:229-250` (add to `systemIntegration` group or new `backup` group)

- [ ] **Step 1: Add backup fields to AllSetting struct**

In `web/entity/entity.go`, add before the closing of the struct (line 124, after `TurnstileSecretKey`):

```go
	// Backup settings
	BackupEnabled   bool   `json:"backupEnabled" form:"backupEnabled"`     // Enable scheduled backups
	BackupFrequency string `json:"backupFrequency" form:"backupFrequency"` // hourly, every12h, daily, weekly
	BackupHour      int    `json:"backupHour" form:"backupHour"`           // hour of day (0-23)
	BackupMaxCount  int    `json:"backupMaxCount" form:"backupMaxCount"`   // max backups to retain (1-100)
```

- [ ] **Step 2: Add default values in setting.go**

In `web/service/setting.go`, add inside the `defaultValueMap` block (after line 129 `"trafficFlushInterval": "10",`):

```go
	// Backup settings
	"backupEnabled":   "false",
	"backupFrequency": "daily",
	"backupHour":      "3",
	"backupMaxCount":  "10",
```

- [ ] **Step 3: Add backup group to settingGroups**

In `web/service/setting.go`, add a new `"backup"` group to `settingGroups` (after the `"node"` group, line 248):

```go
	"backup": {
		"enabled":   "backupEnabled",
		"frequency": "backupFrequency",
		"hour":      "backupHour",
		"maxCount":  "backupMaxCount",
	},
```

- [ ] **Step 4: Add getter/setter methods to SettingService**

In `web/service/setting.go`, add after the existing setting methods (e.g., after line 1089 `GetLdapSyncCron`):

```go
func (s *SettingService) GetBackupEnabled() (bool, error)   { return s.getBool("backupEnabled") }
func (s *SettingService) GetBackupFrequency() (string, error) { return s.getString("backupFrequency") }
func (s *SettingService) GetBackupHour() (int, error)         { return s.getInt("backupHour") }
func (s *SettingService) GetBackupMaxCount() (int, error)     { return s.getInt("backupMaxCount") }
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/entity/entity.go web/service/setting.go
git commit -m "feat: add backup config fields to entity and setting service"
```

---

### Task 2: Create BackupService

**Files:**
- Create: `web/service/backup.go`

- [ ] **Step 1: Create `web/service/backup.go`**

```go
package service

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

const (
	backupDir           = "/etc/x-ui/backups"
	minDiskFreeMB       = 100
	maxBackupDirSizeMB  = 500
)

type BackupMeta struct {
	DBType    string `json:"dbType"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

type BackupEntry struct {
	Filename  string `json:"filename"`
	Timestamp string `json:"timestamp"`
	Size      int64  `json:"size"`
}

// BackupService handles database backup, restore, and file management operations.
type BackupService struct {
}

// ensureBackupDir creates the backup directory if it does not exist.
func ensureBackupDir() error {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	return nil
}

// checkNodeRole verifies the current node is a master node.
func checkNodeRole() error {
	nodeCfg := config.GetNodeConfigFromJSON()
	if nodeCfg.Role == config.NodeRoleWorker {
		return fmt.Errorf("backup and restore can only be performed on the master node")
	}
	return nil
}

// CreateBackup creates an immediate backup of the current database.
func (s *BackupService) CreateBackup() (string, error) {
	if err := checkNodeRole(); err != nil {
		return "", err
	}
	if err := ensureBackupDir(); err != nil {
		return "", err
	}

	dbCfg := config.GetDBConfigFromJSON()
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("backup-%s.tar.gz", timestamp)
	filePath := filepath.Join(backupDir, filename)

	var dumpSQL string
	var err error

	switch dbCfg.Type {
	case "mariadb":
		dumpSQL, err = dumpMariaDB(dbCfg)
	case "sqlite", "":
		dumpSQL, err = dumpSQLite(config.GetDBPath())
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbCfg.Type)
	}
	if err != nil {
		return "", fmt.Errorf("database dump failed: %w", err)
	}

	meta := BackupMeta{
		DBType:    defaultDBType(dbCfg.Type),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   config.GetVersion(),
	}

	if err := createTarGz(filePath, meta, dumpSQL); err != nil {
		return "", fmt.Errorf("create archive failed: %w", err)
	}

	return filePath, nil
}

// ListBackups returns all backup files sorted by time (newest first).
func (s *BackupService) ListBackups() ([]BackupEntry, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupEntry{}, nil
		}
		return nil, fmt.Errorf("read backup directory: %w", err)
	}

	var result []BackupEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "backup-") || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		ts := extractTimestamp(entry.Name())
		result = append(result, BackupEntry{
			Filename:  entry.Name(),
			Timestamp: ts,
			Size:      info.Size(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp > result[j].Timestamp
	})

	return result, nil
}

// RestoreBackup restores the database from a backup file.
func (s *BackupService) RestoreBackup(filename string) error {
	if err := checkNodeRole(); err != nil {
		return err
	}

	filePath := filepath.Join(backupDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", filePath)
	}

	meta, dumpSQL, err := extractTarGz(filePath)
	if err != nil {
		return fmt.Errorf("invalid backup file: %w", err)
	}

	dbCfg := config.GetDBConfigFromJSON()
	currentDBType := defaultDBType(dbCfg.Type)

	if meta.DBType != currentDBType {
		return fmt.Errorf("backup type (%s) does not match current database (%s)", meta.DBType, currentDBType)
	}

	// Create safety backup before restoring
	safetyFile, err := s.createSafetyBackup(dbCfg)
	if err != nil {
		logger.Warning("failed to create safety backup before restore:", err)
	} else {
		logger.Info("safety backup created:", safetyFile)
	}

	if err := restoreDB(dbCfg, dumpSQL); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	return nil
}

// DeleteBackup deletes a backup file.
func (s *BackupService) DeleteBackup(filename string) error {
	filePath := filepath.Join(backupDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", filePath)
	}
	return os.Remove(filePath)
}

// GetBackupPath returns the full path to a backup file.
func (s *BackupService) GetBackupPath(filename string) string {
	return filepath.Join(backupDir, filename)
}

// ApplyRetention applies retention policy, keeping maxCount newest backups.
func (s *BackupService) ApplyRetention(maxCount int) error {
	entries, err := s.ListBackups()
	if err != nil {
		return err
	}

	// Also count pre-restore safety backups for total size check, but exclude from retention deletion
	if len(entries) <= maxCount {
		return nil
	}

	for i := maxCount; i < len(entries); i++ {
		filePath := s.GetBackupPath(entries[i].Filename)
		if err := os.Remove(filePath); err != nil {
			logger.Warning("failed to delete old backup:", entries[i].Filename, err)
		} else {
			logger.Info("deleted old backup:", entries[i].Filename)
		}
	}

	return nil
}

// CreateSnapshot creates a scheduled backup and applies retention.
func (s *BackupService) CreateSnapshot(settingService SettingService) error {
	enabled, err := settingService.GetBackupEnabled()
	if err != nil || !enabled {
		return nil
	}

	filePath, err := s.CreateBackup()
	if err != nil {
		return fmt.Errorf("snapshot backup failed: %w", err)
	}
	logger.Info("scheduled backup created:", filePath)

	maxCount := 10
	if mc, err := settingService.GetBackupMaxCount(); err == nil && mc > 0 {
		maxCount = mc
	}
	if err := s.ApplyRetention(maxCount); err != nil {
		logger.Warning("retention apply failed:", err)
	}

	return nil
}

// createSafetyBackup creates an emergency backup before restore.
func (s *BackupService) createSafetyBackup(dbCfg config.DBConfig) (string, error) {
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("pre-restore-%s.tar.gz", timestamp)
	filePath := filepath.Join(backupDir, filename)

	var dumpSQL string
	var err error
	switch dbCfg.Type {
	case "mariadb":
		dumpSQL, err = dumpMariaDB(dbCfg)
	default:
		dumpSQL, err = dumpSQLite(config.GetDBPath())
	}
	if err != nil {
		return "", err
	}

	meta := BackupMeta{
		DBType:    defaultDBType(dbCfg.Type),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   config.GetVersion(),
	}

	if err := createTarGz(filePath, meta, dumpSQL); err != nil {
		return "", err
	}

	return filePath, nil
}

// defaultDBType normalizes empty string to "sqlite".
func defaultDBType(t string) string {
	if t == "" {
		return "sqlite"
	}
	return t
}

// extractTimestamp extracts the timestamp string from backup filename.
func extractTimestamp(filename string) string {
	// backup-2026-04-26-030000.tar.gz
	name := strings.TrimPrefix(filename, "backup-")
	name = strings.TrimSuffix(name, ".tar.gz")
	return name
}

// dumpMariaDB runs mysqldump and returns the SQL output.
func dumpMariaDB(dbCfg config.DBConfig) (string, error) {
	args := []string{
		"--single-transaction",
		"--routines",
		"--triggers",
		"--no-tablespaces",
		fmt.Sprintf("-h%s", dbCfg.Host),
		fmt.Sprintf("-P%s", dbCfg.Port),
	}
	if dbCfg.User != "" {
		args = append(args, fmt.Sprintf("-u%s", dbCfg.User))
	}
	if dbCfg.Password != "" {
		args = append(args, fmt.Sprintf("-p%s", dbCfg.Password))
	}
	args = append(args, dbCfg.Name)

	cmd := exec.Command("mysqldump", args...)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("mysqldump: %w (output: %s)", err, out.String())
	}
	return out.String(), nil
}

// dumpSQLite runs sqlite3 .dump and returns the SQL output.
func dumpSQLite(dbPath string) (string, error) {
	database.Checkpoint()
	cmd := exec.Command("sqlite3", dbPath, ".dump")
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("sqlite3 dump: %w (output: %s)", err, out.String())
	}
	return out.String(), nil
}

// restoreDB restores the database from SQL dump.
func restoreDB(dbCfg config.DBConfig, dumpSQL string) error {
	switch dbCfg.Type {
	case "mariadb":
		args := []string{
			fmt.Sprintf("-h%s", dbCfg.Host),
			fmt.Sprintf("-P%s", dbCfg.Port),
		}
		if dbCfg.User != "" {
			args = append(args, fmt.Sprintf("-u%s", dbCfg.User))
		}
		if dbCfg.Password != "" {
			args = append(args, fmt.Sprintf("-p%s", dbCfg.Password))
		}
		args = append(args, dbCfg.Name)

		cmd := exec.Command("mysql", args...)
		cmd.Stdin = strings.NewReader(dumpSQL)
		var stderr strings.Builder
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("mysql restore: %w (stderr: %s)", err, stderr.String())
		}
	default:
		cmd := exec.Command("sqlite3", config.GetDBPath())
		cmd.Stdin = strings.NewReader(dumpSQL)
		var stderr strings.Builder
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sqlite3 restore: %w (stderr: %s)", err, stderr.String())
		}
	}
	return nil
}

// createTarGz creates a tar.gz archive containing metadata.json and dump.sql.
func createTarGz(filePath string, meta BackupMeta, dumpSQL string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// metadata.json
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "metadata.json",
		Size:     int64(len(metaBytes)),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	if _, err := tw.Write(metaBytes); err != nil {
		return err
	}

	// dump.sql
	dumpBytes := []byte(dumpSQL)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "dump.sql",
		Size:     int64(len(dumpBytes)),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	if _, err := tw.Write(dumpBytes); err != nil {
		return err
	}

	return nil
}

// extractTarGz reads a tar.gz archive and returns metadata and SQL dump.
func extractTarGz(filePath string) (*BackupMeta, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, "", err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	var meta BackupMeta
	var dumpSQL string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}

		var buf strings.Builder
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, "", err
		}

		switch hdr.Name {
		case "metadata.json":
			if err := json.Unmarshal([]byte(buf.String()), &meta); err != nil {
				return nil, "", fmt.Errorf("invalid metadata.json: %w", err)
			}
		case "dump.sql":
			dumpSQL = buf.String()
		}
	}

	if dumpSQL == "" {
		return nil, "", fmt.Errorf("dump.sql not found in archive")
	}
	if meta.DBType == "" {
		return nil, "", fmt.Errorf("metadata.json missing dbType")
	}

	return &meta, dumpSQL, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/service/backup.go
git commit -m "feat: add BackupService with dump, archive, restore logic"
```

---

### Task 3: Create BackupController

**Files:**
- Create: `web/controller/backup.go`

- [ ] **Step 1: Create `web/controller/backup.go`**

```go
package controller

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// BackupController handles database backup and restore API endpoints.
type BackupController struct {
	BaseController
	backupService service.BackupService
}

// initRouter registers backup API routes.
func (a *BackupController) initRouter(g *gin.RouterGroup) {
	g.POST("/backup", a.createBackup)
	g.POST("/restore/:filename", a.restoreBackup)
	g.POST("/deleteBackup/:filename", a.deleteBackup)
	g.GET("/listBackups", a.listBackups)
	g.GET("/downloadBackup/:filename", a.downloadBackup)
}

// createBackup creates an immediate manual backup.
func (a *BackupController) createBackup(c *gin.Context) {
	filePath, err := a.backupService.CreateBackup()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.backup.createError"), err)
		return
	}
	jsonObj(c, filePath, nil)
}

// restoreBackup restores the database from a backup file.
func (a *BackupController) restoreBackup(c *gin.Context) {
	filename := c.Param("filename")
	if !isValidFilename(filename) {
		jsonMsg(c, "Invalid filename", fmt.Errorf("invalid filename"))
		return
	}
	if err := a.backupService.RestoreBackup(filename); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.backup.restoreError"), err)
		return
	}
	jsonObj(c, "restore completed", nil)
}

// deleteBackup deletes a backup file.
func (a *BackupController) deleteBackup(c *gin.Context) {
	filename := c.Param("filename")
	if !isValidFilename(filename) {
		jsonMsg(c, "Invalid filename", fmt.Errorf("invalid filename"))
		return
	}
	if err := a.backupService.DeleteBackup(filename); err != nil {
		jsonMsg(c, "delete failed", err)
		return
	}
	jsonObj(c, "deleted", nil)
}

// listBackups lists all backup files.
func (a *BackupController) listBackups(c *gin.Context) {
	entries, err := a.backupService.ListBackups()
	if err != nil {
		jsonMsg(c, "list backups failed", err)
		return
	}
	jsonObj(c, entries, nil)
}

// downloadBackup downloads a backup file.
func (a *BackupController) downloadBackup(c *gin.Context) {
	filename := c.Param("filename")
	if !isValidFilename(filename) {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid filename"))
		return
	}
	filePath := a.backupService.GetBackupPath(filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		jsonMsg(c, "read backup file failed", err)
		return
	}
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Writer.Write(data)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/controller/backup.go
git commit -m "feat: add BackupController API endpoints"
```

---

### Task 4: Create BackupJob for scheduled snapshots

**Files:**
- Create: `web/job/backup_job.go`

- [ ] **Step 1: Create `web/job/backup_job.go`**

```go
package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// BackupJob handles scheduled database backups.
type BackupJob struct {
	settingService service.SettingService
	backupService  service.BackupService
	lastRun        string // stores the frequency of the last run to detect change
}

// NewBackupJob creates a new BackupJob instance.
func NewBackupJob() *BackupJob {
	return &BackupJob{}
}

// Run executes the scheduled backup if enabled and time matches the configured frequency.
func (j *BackupJob) Run() {
	enabled, err := j.settingService.GetBackupEnabled()
	if err != nil || !enabled {
		return
	}

	frequency, err := j.settingService.GetBackupFrequency()
	if err != nil {
		return
	}

	if !j.shouldRun(frequency) {
		return
	}

	if err := j.backupService.CreateSnapshot(j.settingService); err != nil {
		logger.Warning("scheduled backup failed:", err)
	}
}

// shouldRun checks if the backup should run based on the configured frequency.
func (j *BackupJob) shouldRun(frequency string) bool {
	now := time.Now()

	switch frequency {
	case "hourly":
		return now.Minute() == 0
	case "every12h":
		return (now.Hour() == 0 || now.Hour() == 12) && now.Minute() == 0
	case "daily":
		hour, err := j.settingService.GetBackupHour()
		if err != nil {
			hour = 3
		}
		return now.Hour() == hour && now.Minute() == 0
	case "weekly":
		if now.Weekday() != time.Sunday {
			return false
		}
		hour, err := j.settingService.GetBackupHour()
		if err != nil {
			hour = 3
		}
		return now.Hour() == hour && now.Minute() == 0
	default:
		return false
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/job/backup_job.go
git commit -m "feat: add BackupJob for scheduled snapshots"
```

---

### Task 5: Wire up routes and job scheduling

**Files:**
- Modify: `web/controller/server.go:41-63` (add backup routes to initRouter)
- Modify: `web/web.go:360-413` (add backup job to cron)

- [ ] **Step 1: Register backup routes in ServerController**

In `web/controller/server.go`, inside `initRouter()` after line 62, add:

```go
	// Backup routes
	backupCtrl := BackupController{}
	backupCtrl.initRouter(g)
```

- [ ] **Step 2: Register backup job in web.go**

In `web/web.go`, after `s.cron.AddJob("@daily", job.NewClearLogsJob())` (around line 365), add:

```go
	// Schedule database backup job (runs every minute, checks schedule internally)
	s.cron.AddJob("@every 1m", job.NewBackupJob())
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Run vet**

Run: `go vet ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add web/controller/server.go web/web.go
git commit -m "feat: wire backup routes and scheduling job"
```

---

### Task 6: Create backup UI page

**Files:**
- Create: `web/html/settings/backup.html`
- Modify: `web/html/settings.html:67-109` (add backup tab)

- [ ] **Step 1: Read current settings.html tabs section**

Read `web/html/settings.html:67-109` to understand the current tab structure before editing.

- [ ] **Step 2: Create `web/html/settings/backup.html`**

```html
<div>
    <a-row :gutter="[16, 16]" :style="{ marginTop: '16px' }">
        <a-col :span="24">
            <a-card :title="'Backup Configuration'" size="small">
                <a-form-model :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }" label-align="left">
                    <a-row :gutter="[16, 16]">
                        <a-col :xs="24" :sm="12">
                            <a-form-model-item :label="'Enable Scheduled Backup'">
                                <a-switch v-model="allSetting.backupEnabled"></a-switch>
                            </a-form-model-item>
                        </a-col>
                        <a-col :xs="24" :sm="12">
                            <a-form-model-item :label="'Frequency'">
                                <a-select v-model="allSetting.backupFrequency" :disabled="!allSetting.backupEnabled">
                                    <a-select-option value="hourly">Every Hour</a-select-option>
                                    <a-select-option value="every12h">Every 12 Hours</a-select-option>
                                    <a-select-option value="daily">Every Day</a-select-option>
                                    <a-select-option value="weekly">Every Week</a-select-option>
                                </a-select>
                            </a-form-model-item>
                        </a-col>
                        <a-col :xs="24" :sm="12">
                            <a-form-model-item :label="'Hour (0-23)'"
                                v-if="allSetting.backupFrequency === 'daily' || allSetting.backupFrequency === 'weekly'">
                                <a-input-number v-model="allSetting.backupHour" :min="0" :max="23"
                                    :disabled="!allSetting.backupEnabled"></a-input-number>
                            </a-form-model-item>
                        </a-col>
                        <a-col :xs="24" :sm="12">
                            <a-form-model-item :label="'Max Backups (1-100)'">
                                <a-input-number v-model="allSetting.backupMaxCount" :min="1" :max="100"
                                    :disabled="!allSetting.backupEnabled"></a-input-number>
                            </a-form-model-item>
                        </a-col>
                    </a-row>
                </a-form-model>
            </a-card>
        </a-col>
        <a-col :span="24">
            <a-card :title="'Manual Operations'" size="small">
                <a-space>
                    <a-button type="primary" icon="plus" @click="createBackup" :loading="backupCreating">
                        Create Backup Now
                    </a-button>
                </a-space>
            </a-card>
        </a-col>
        <a-col :span="24">
            <a-card size="small">
                <span slot="title">Backup List
                    <a-badge :count="backupList.length" :number-style="{ backgroundColor: '#52c41a' }"
                        :style="{ marginLeft: '8px' }" />
                </span>
                <a-table :columns="backupColumns" :data-source="backupList" :pagination="false" :loading="backupLoading"
                    size="small" row-key="filename">
                    <template slot="size" slot-scope="text">
                        [[ formatFileSize(text) ]]
                    </template>
                    <template slot="timestamp" slot-scope="text">
                        [[ formatBackupTime(text) ]]
                    </template>
                    <template slot="action" slot-scope="text, record">
                        <a-space>
                            <a-button size="small" icon="download" @click="downloadBackup(record.filename)">Download</a-button>
                            <a-popconfirm :title="'Restore will stop the panel temporarily. Continue?'" ok-text="Yes"
                                cancel-text="No" @confirm="restoreBackup(record.filename)">
                                <a-button size="small" type="danger" icon="redo">Restore</a-button>
                            </a-popconfirm>
                            <a-popconfirm title="Delete this backup?" ok-text="Yes" cancel-text="No"
                                @confirm="deleteBackup(record.filename)">
                                <a-button size="small" icon="delete">Delete</a-button>
                            </a-popconfirm>
                        </a-space>
                    </template>
                </a-table>
            </a-card>
        </a-col>
    </a-row>
</div>
```

- [ ] **Step 3: Add backup tab to settings.html**

In `web/html/settings.html`, after the clash tab (closing `</a-tab-pane>` on line 109), add:

```html
                  <a-tab-pane key="7" :style="{ paddingTop: '20px' }">
                    <template #tab>
                      <a-icon type="database"></a-icon>
                      <span>Backup</span>
                    </template>
                    {{ template "settings/backup" . }}
                  </a-tab-pane>
```

- [ ] **Step 4: Add backup Vue.js methods and data to settings.html script**

In `web/html/settings.html`, in the `<script>` section (after the Vue data block), add to the `data` object:

```javascript
      backupList: [],
      backupColumns: [
        { title: 'Filename', dataIndex: 'filename', key: 'filename' },
        { title: 'Timestamp', dataIndex: 'timestamp', key: 'timestamp', scopedSlots: { customRender: 'timestamp' } },
        { title: 'Size', dataIndex: 'size', key: 'size', scopedSlots: { customRender: 'size' } },
        { title: 'Actions', key: 'action', scopedSlots: { customRender: 'action' } }
      ],
      backupLoading: false,
      backupCreating: false,
```

And add methods to the Vue instance:

```javascript
    fetchBackups() {
      this.backupLoading = true;
      axios.get(this.entryHost + 'panel/api/server/listBackups', {
        headers: this.authHeaders
      }).then(res => {
        this.backupList = res.data.obj || [];
      }).catch(err => {
        this.$message.error('Failed to load backups: ' + (err.response?.data?.msg || err.message));
      }).finally(() => {
        this.backupLoading = false;
      });
    },
    createBackup() {
      this.backupCreating = true;
      axios.post(this.entryHost + 'panel/api/server/backup', {}, {
        headers: this.authHeaders
      }).then(res => {
        this.$message.success('Backup created successfully');
        this.fetchBackups();
      }).catch(err => {
        this.$message.error('Backup failed: ' + (err.response?.data?.msg || err.message));
      }).finally(() => {
        this.backupCreating = false;
      });
    },
    restoreBackup(filename) {
      axios.post(this.entryHost + 'panel/api/server/restore/' + filename, {}, {
        headers: this.authHeaders
      }).then(res => {
        this.$message.success('Restore completed successfully');
        this.fetchBackups();
      }).catch(err => {
        this.$message.error('Restore failed: ' + (err.response?.data?.msg || err.message));
      });
    },
    deleteBackup(filename) {
      axios.post(this.entryHost + 'panel/api/server/deleteBackup/' + filename, {}, {
        headers: this.authHeaders
      }).then(res => {
        this.$message.success('Backup deleted');
        this.fetchBackups();
      }).catch(err => {
        this.$message.error('Delete failed: ' + (err.response?.data?.msg || err.message));
      });
    },
    downloadBackup(filename) {
      // Open download in new tab
      window.open(this.entryHost + 'panel/api/server/downloadBackup/' + filename, '_blank');
    },
    formatFileSize(bytes) {
      if (bytes === 0) return '0 B';
      const k = 1024;
      const sizes = ['B', 'KB', 'MB', 'GB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    },
    formatBackupTime(ts) {
      if (!ts) return '';
      return ts.replace(/-/g, ':').replace(/(\d{4}):(\d{2}):(\d{2}):(\d{2})(\d{2})(\d{2})/, '$1-$2-$3 $4:$5:$6');
    },
```

Also add a watcher or call `fetchBackups` when the backup tab is selected. Add to `onSettingsTabChange` method or add a `watch`:

```javascript
    onSettingsTabChange(key) {
      if (key === '7') {
        this.fetchBackups();
      }
    },
```

If there's already an `onSettingsTabChange` method, merge the `key === '7'` condition.

Also add a periodic refresh timer. In the `mounted` hook or similar:

```javascript
    startBackupRefresh() {
      this.backupRefreshInterval = setInterval(() => {
        const activeTab = this.$el?.querySelector?.('.ant-tabs-tab-active')?.getAttribute?.('data-node-key');
        if (activeTab === '7') {
          this.fetchBackups();
        }
      }, 30000);
    },
```

And call `startBackupRefresh()` in `mounted()`, and clear it in `beforeDestroy()`:

```javascript
    beforeDestroy() {
      if (this.backupRefreshInterval) {
        clearInterval(this.backupRefreshInterval);
      }
    },
```

- [ ] **Step 5: Verify template parses**

Run: `CGO_ENABLED=1 go build -ldflags "-w -s" ./...`
Expected: compilation passes

- [ ] **Step 6: Commit**

```bash
git add web/html/settings/backup.html web/html/settings.html
git commit -m "feat: add backup UI page and settings tab"
```

---

### Task 7: Add frontend model fields

**Files:**
- Modify: `web/assets/js/model/setting.js`

- [ ] **Step 1: Add backup fields to AllSetting JS class**

In `web/assets/js/model/setting.js`, add before the closing `}` of the AllSetting constructor (after line with `this.turnstileSecretKey` or after the LDAP fields):

```javascript
        this.backupEnabled = false;
        this.backupFrequency = "daily";
        this.backupHour = 3;
        this.backupMaxCount = 10;
```

- [ ] **Step 2: Match order with entity.go for presentKeys**

Verify the field order in `AllSetting` struct matches usage. The `UpdateAllSetting` function uses `json` tags — order doesn't matter as long as tag names match.

- [ ] **Step 3: Commit**

```bash
git add web/assets/js/model/setting.js
git commit -m "feat: add backup config fields to frontend model"
```

---

### Task 8: Add CLI subcommands to main.go

**Files:**
- Modify: `main.go:623-641` (add backup/restore flag sets)

- [ ] **Step 1: Add backup and restore command flag sets in main.go**

After the `migrateDbCmd` definition (around line 625), add:

```go
	backupCmd := flag.NewFlagSet("backup", flag.ExitOnError)

	restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	var restoreFile string
	restoreCmd.StringVar(&restoreFile, "file", "", "Backup file name to restore from")
```

- [ ] **Step 2: Add subcommands to the switch and usage**

Update the `flag.Usage` function (around line 636-641) to include new commands:

```go
		fmt.Println("    backup         create a database backup")
		fmt.Println("    restore        restore database from backup")
```

In the switch statement (after `case "setting":` block, around line 838), add:

```go
	case "backup":
		err := backupCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		runBackup()
	case "restore":
		err := restoreCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		if restoreFile == "" {
			fmt.Println("--file flag is required")
			return
		}
		runRestore(restoreFile)
```

- [ ] **Step 3: Add backup and restore functions in main.go**

Add these functions near other command functions (e.g., near `migrateDbBetweenDrivers`):

```go
// runBackup creates a database backup from the CLI.
func runBackup() {
	checkNodeRoleOrExit()

	dbCfg := config.GetDBConfigFromJSON()

	// Initialize DB to read settings if needed
	backupDir := "/etc/x-ui/backups"
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("backup-%s.tar.gz", timestamp)
	filePath := filepath.Join(backupDir, filename)

	var dumpSQL string
	var err error

	switch dbCfg.Type {
	case "mariadb":
		dumpSQL, err = dumpMariaDBCLI(dbCfg)
	case "sqlite", "":
		dbPath := config.GetDBPath()
		dumpSQL, err = dumpSQLiteCLI(dbPath)
	default:
		fmt.Println("unsupported database type:", dbCfg.Type)
		os.Exit(1)
	}
	if err != nil {
		fmt.Println("dump failed:", err)
		os.Exit(1)
	}

	meta := map[string]string{
		"dbType":    dbCfg.Type,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   config.GetVersion(),
	}
	if meta["dbType"] == "" {
		meta["dbType"] = "sqlite"
	}

	if err := createTarGzCLI(filePath, meta, dumpSQL); err != nil {
		fmt.Println("archive creation failed:", err)
		os.Exit(1)
	}

	fmt.Println("backup created:", filePath)
}

// runRestore restores a database from a backup file via CLI.
func runRestore(filename string) {
	checkNodeRoleOrExit()

	filePath := filepath.Join("/etc/x-ui/backups", filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("backup file not found:", filePath)
		os.Exit(1)
	}

	meta, dumpSQL, err := extractTarGzCLI(filePath)
	if err != nil {
		fmt.Println("invalid backup file:", err)
		os.Exit(1)
	}

	dbCfg := config.GetDBConfigFromJSON()
	currentDBType := dbCfg.Type
	if currentDBType == "" {
		currentDBType = "sqlite"
	}

	if meta["dbType"] != currentDBType {
		fmt.Printf("backup type (%s) does not match current database (%s)\n", meta["dbType"], currentDBType)
		os.Exit(1)
	}

	// Create safety backup
	timestamp := time.Now().Format("2006-01-02-150405")
	safetyFile := filepath.Join("/etc/x-ui/backups", "pre-restore-"+timestamp+".tar.gz")
	safetyMeta := map[string]string{
		"dbType":    currentDBType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   config.GetVersion(),
	}
	var safetySQL string
	switch currentDBType {
	case "mariadb":
		safetySQL, err = dumpMariaDBCLI(dbCfg)
	default:
		safetySQL, err = dumpSQLiteCLI(config.GetDBPath())
	}
	if err == nil {
		if err := createTarGzCLI(safetyFile, safetyMeta, safetySQL); err == nil {
			fmt.Println("safety backup created:", safetyFile)
		}
	}

	if err := restoreDBCLI(dbCfg, dumpSQL); err != nil {
		fmt.Println("restore failed:", err)
		os.Exit(1)
	}

	fmt.Println("restore completed successfully")
}

func checkNodeRoleOrExit() {
	nodeCfg := config.GetNodeConfigFromJSON()
	if nodeCfg.Role == config.NodeRoleWorker {
		fmt.Println("backup and restore can only be performed on the master node")
		os.Exit(1)
	}
}

func dumpMariaDBCLI(dbCfg config.DBConfig) (string, error) {
	args := []string{
		"--single-transaction",
		"--routines",
		"--triggers",
		"--no-tablespaces",
		fmt.Sprintf("-h%s", dbCfg.Host),
		fmt.Sprintf("-P%s", dbCfg.Port),
	}
	if dbCfg.User != "" {
		args = append(args, fmt.Sprintf("-u%s", dbCfg.User))
	}
	if dbCfg.Password != "" {
		args = append(args, fmt.Sprintf("-p%s", dbCfg.Password))
	}
	args = append(args, dbCfg.Name)

	cmd := exec.Command("mysqldump", args...)
	var out strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return out.String(), nil
}

func dumpSQLiteCLI(dbPath string) (string, error) {
	cmd := exec.Command("sqlite3", dbPath, ".dump")
	var out strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return out.String(), nil
}

func restoreDBCLI(dbCfg config.DBConfig, dumpSQL string) error {
	switch dbCfg.Type {
	case "mariadb":
		args := []string{
			fmt.Sprintf("-h%s", dbCfg.Host),
			fmt.Sprintf("-P%s", dbCfg.Port),
		}
		if dbCfg.User != "" {
			args = append(args, fmt.Sprintf("-u%s", dbCfg.User))
		}
		if dbCfg.Password != "" {
			args = append(args, fmt.Sprintf("-p%s", dbCfg.Password))
		}
		args = append(args, dbCfg.Name)

		cmd := exec.Command("mysql", args...)
		cmd.Stdin = strings.NewReader(dumpSQL)
		var stderr strings.Builder
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
	default:
		dbPath := config.GetDBPath()
		cmd := exec.Command("sqlite3", dbPath)
		cmd.Stdin = strings.NewReader(dumpSQL)
		var stderr strings.Builder
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
	}
	return nil
}

func createTarGzCLI(filePath string, meta map[string]string, dumpSQL string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := tw.WriteHeader(&tar.Header{Name: "metadata.json", Size: int64(len(metaBytes)), Mode: 0644, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := tw.Write(metaBytes); err != nil {
		return err
	}

	dumpBytes := []byte(dumpSQL)
	if err := tw.WriteHeader(&tar.Header{Name: "dump.sql", Size: int64(len(dumpBytes)), Mode: 0644, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := tw.Write(dumpBytes); err != nil {
		return err
	}

	return nil
}

func extractTarGzCLI(filePath string) (map[string]string, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, "", err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	meta := make(map[string]string)
	var dumpSQL string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}

		var buf strings.Builder
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, "", err
		}

		switch hdr.Name {
		case "metadata.json":
			json.Unmarshal([]byte(buf.String()), &meta)
		case "dump.sql":
			dumpSQL = buf.String()
		}
	}

	if dumpSQL == "" {
		return nil, "", fmt.Errorf("dump.sql not found")
	}
	if meta["dbType"] == "" {
		return nil, "", fmt.Errorf("metadata.json missing dbType")
	}

	return meta, dumpSQL, nil
}
```

- [ ] **Step 4: Add required imports in main.go**

Ensure these imports are added to main.go's import block (in addition to existing ones):

```go
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
```

- [ ] **Step 5: Verify compilation**

Run: `go build -ldflags "-w -s" -o /usr/local/x-ui/x-ui ./main.go`
Expected: compilation passes

- [ ] **Step 6: Test CLI backup (if SQLite)**

Run: `./x-ui backup`
Expected: "backup created: /etc/x-ui/backups/backup-YYYY-MM-DD-HHmmss.tar.gz"

- [ ] **Step 7: Commit**

```bash
git add main.go
git commit -m "feat: add backup and restore CLI subcommands"
```

---

### Task 9: Modify x-ui.sh

**Files:**
- Modify: `x-ui.sh:3826-3877` (add subcommands)
- Modify: `x-ui.sh:3591-3689` (add db_menu items)

- [ ] **Step 1: Add backup, restore, list-backups to subcommand router**

In `x-ui.sh`, add before the `*) show_usage ;;` line in the subcommand routing section:

```bash
        "backup"        ) check_install 0 && backup_db ;;
        "restore"       ) check_install 0 && restore_db "${2}" ;;
        "list-backups"  ) check_install 0 && list_backups ;;
```

- [ ] **Step 2: Add backup, restore, list-backups functions**

Add these functions in the script, near the db_menu section:

```bash
backup_db() {
    echo -e "${green}Creating database backup...${plain}"
    ${xui_folder}/x-ui backup
}

restore_db() {
    local backup_file="$1"
    if [[ -z "$backup_file" ]]; then
        echo -e "${red}Usage: x-ui restore <backup-filename>${plain}"
        list_backups
        return 1
    fi
    local full_path="/etc/x-ui/backups/${backup_file}"
    if [[ ! -f "$full_path" ]]; then
        echo -e "${red}Backup file not found: $full_path${plain}"
        list_backups
        return 1
    fi
    echo -e "${yellow}WARNING: Restore will stop the panel and replace the database.${plain}"
    read -p "Continue? (y/n) " confirm
    if [[ "$confirm" != "y" ]]; then
        echo "Cancelled."
        return 0
    fi
    echo "Stopping panel..."
    stop
    echo "Restoring from $backup_file..."
    ${xui_folder}/x-ui restore --file="$backup_file"
    echo "Starting panel..."
    start
    echo -e "${green}Restore completed.${plain}"
}

list_backups() {
    local backup_dir="/etc/x-ui/backups"
    if [[ ! -d "$backup_dir" ]]; then
        echo "No backups found."
        return 0
    fi
    echo -e "${green}Backups in ${backup_dir}:${plain}"
    ls -lh "$backup_dir" | grep "backup-" | awk '{print $5, $6, $7, $8, $9}'
}
```

Note: `xui_db_folder` is already defined in x-ui.sh as `/etc/x-ui` (default). The `backup` directory path in the script should use `/etc/x-ui/backups` (same as the Go constant).

- [ ] **Step 3: Add backup menu items to db_menu**

In the `db_menu()` function, at the end before the menu display (before the read/prompt), add these menu items:

```bash
    17) echo -e "  17) ${green}Create database backup${plain}" ;;
    18) echo -e "  18) ${green}Restore from backup${plain}" ;;
    19) echo -e "  19) ${green}List all backups${plain}" ;;
```

And in the case statement after item 16:

```bash
                17) backup_db ;;
                18)
                    list_backups
                    echo ""
                    read -p "Enter backup filename to restore: " restore_filename
                    restore_db "$restore_filename"
                    ;;
                19) list_backups ;;
```

- [ ] **Step 4: Commit**

```bash
git add x-ui.sh
git commit -m "feat: add backup/restore subcommands and menu to x-ui.sh"
```

---

### Task 10: Build, format, and verify

- [ ] **Step 1: Format Go code**

Run: `gofmt -l -w .`
Expected: no output (or only files that were modified)

- [ ] **Step 2: Generate assets**

Run: `go run ./cmd/genassets`
Expected: completes successfully

- [ ] **Step 3: Build binary**

Run: `CGO_ENABLED=1 go build -ldflags "-w -s" -o /usr/local/x-ui/x-ui ./main.go`
Expected: compilation passes with no errors

- [ ] **Step 4: Run vet and staticcheck**

Run: `go vet ./...`
Expected: no errors

- [ ] **Step 5: Run tests (database-related)**

Run: `go test -race ./database/...`
Expected: PASS

Run: `go test -race ./web/service/...`
Expected: PASS

- [ ] **Step 6: Verify backup directory creation**

Run: `ls -la /etc/x-ui/backups/` (if running service)
Expected: directory created

- [ ] **Step 7: Create task tracking record**

Create file `docs/Tasktracking/2026-04-26-database-backup-snapshot.md` with the standard template.

- [ ] **Step 8: Final commit**

```bash
git add -f docs/Tasktracking/2026-04-26-database-backup-snapshot.md
git commit -m "docs: add task tracking record for backup feature"
```
