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
	backupDir = "/etc/x-ui/backups"
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
