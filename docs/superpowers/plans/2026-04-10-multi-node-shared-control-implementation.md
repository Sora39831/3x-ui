# Multi-Node Shared Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a minimal multi-node shared-control mode to `3x-ui` where `master` owns shared-account writes, `worker` nodes rebuild local Xray config from synchronized snapshots, and all nodes flush traffic deltas back to MariaDB without counter loss.

**Architecture:** Keep the current `controller -> service -> database -> local Xray` flow intact. Add node-role config, shared metadata tables, cache-backed worker sync, and a durable traffic delta flush path around the existing services instead of changing Xray protocol behavior. In shared mode, workers never write shared account definitions directly, and traffic reconciliation that mutates enable/expiry state runs only on `master`.

**Tech Stack:** Go, GORM, MariaDB, SQLite compatibility mode for legacy single-node installs, existing `web/service`, `web/job`, `web/controller`, `xray` models, shell installers `install.sh` and `x-ui.sh`.

---

## File Map

### Existing files to modify

- `config/config.go` — add typed node-role config readers, validation, and runtime file path helpers.
- `config/config_test.go` — cover node config defaults, validation, and shared runtime file paths.
- `main.go` — validate node config at startup, extend `setting` CLI flags, and print node settings in `setting -show`.
- `database/db.go` — migrate shared metadata models and seed the shared version row.
- `database/db_test.go` — verify metadata tables, version helpers, and node-state upsert behavior.
- `web/service/inbound.go` — enforce master-only shared writes, bump shared version on successful shared mutations, and extract shared traffic reconciliation from `AddTraffic`.
- `web/service/xray.go` — split config building from DB reads so worker sync can rebuild from cached snapshots.
- `web/web.go` — start worker sync / master heartbeat loops and the traffic flush loop with server lifecycle context.
- `web/job/xray_traffic_job.go` — branch shared-mode traffic collection away from direct DB writes.
- `xray/client_traffic.go` — add a composite unique key for `(inbound_id, email)` so atomic delta upserts are safe.
- `x-ui.sh` — show node config and add minimal node-management actions.
- `install.sh` — add fresh-install prompts for MariaDB and node role while preserving upgrade behavior.
- `README.md` — add high-level multi-node shared-control documentation.
- `README.zh_CN.md` — add Chinese operator guidance.

### New files to create

- `database/model/shared_state.go` — `shared_accounts_version` metadata model.
- `database/model/node_state.go` — node heartbeat / sync status model.
- `database/shared_state.go` — shared version and node-state repository helpers.
- `web/service/node_guard.go` — node-role helpers and `RequireMaster`.
- `web/service/node_guard_test.go` — node-role guard and transactional version-bump tests.
- `web/service/node_cache.go` — shared snapshot load/save helpers.
- `web/service/node_sync.go` — worker snapshot polling, cache refresh, heartbeat, and node-state updates.
- `web/service/node_sync_test.go` — snapshot persistence and sync-loop unit tests.
- `web/service/traffic_pending.go` — durable pending traffic delta store.
- `web/service/traffic_flush.go` — shared-mode delta collection and batch flush service.
- `web/service/traffic_flush_test.go` — pending delta merge and flush success/failure coverage.
- `docs/multi-node-sync.md` — operator runbook and manual verification checklist.
- `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md` — task-by-task execution tracker with mode and commit checkpoints.

### Runtime files created by the feature

- `/etc/x-ui/x-ui.json` — now also stores `nodeRole`, `nodeId`, `syncInterval`, `trafficFlushInterval`.
- `/etc/x-ui/shared-cache.json` — last good shared account snapshot used by workers.
- `/etc/x-ui/traffic-pending.json` — durable queue of unflushed traffic deltas.

---

### Task 1: Add node config, runtime file paths, and startup validation

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Modify: `main.go`
**Execution Mode:** Inline

- [ ] **Step 1: Write the failing config tests**

Add these tests to `config/config_test.go`:

```go
func writeTestSettingsFile(t *testing.T, settings map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	if err := os.WriteFile(GetSettingPath(), data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func TestGetNodeConfigFromJSONDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	writeTestSettingsFile(t, map[string]any{})

	cfg := GetNodeConfigFromJSON()
	if cfg.Role != NodeRoleMaster {
		t.Fatalf("expected default role %q, got %q", NodeRoleMaster, cfg.Role)
	}
	if cfg.NodeID != "" {
		t.Fatalf("expected empty default node id, got %q", cfg.NodeID)
	}
	if cfg.SyncIntervalSeconds != 30 {
		t.Fatalf("expected default sync interval 30, got %d", cfg.SyncIntervalSeconds)
	}
	if cfg.TrafficFlushSeconds != 10 {
		t.Fatalf("expected default traffic flush interval 10, got %d", cfg.TrafficFlushSeconds)
	}
}

func TestValidateNodeConfigWorkerRequiresNodeID(t *testing.T) {
	err := ValidateNodeConfig(NodeConfig{
		Role:                NodeRoleWorker,
		SyncIntervalSeconds: 30,
		TrafficFlushSeconds: 10,
	}, DBConfig{Type: "mariadb"})
	if err == nil {
		t.Fatal("expected worker without node id to fail validation")
	}
}

func TestValidateNodeConfigWorkerRequiresMariaDB(t *testing.T) {
	err := ValidateNodeConfig(NodeConfig{
		Role:                NodeRoleWorker,
		NodeID:              "worker-1",
		SyncIntervalSeconds: 30,
		TrafficFlushSeconds: 10,
	}, DBConfig{Type: "sqlite"})
	if err == nil {
		t.Fatal("expected worker on sqlite to fail validation")
	}
}

func TestSharedRuntimeFilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	if got := GetSharedCachePath(); got != filepath.Join(tmpDir, "shared-cache.json") {
		t.Fatalf("unexpected shared cache path: %s", got)
	}
	if got := GetTrafficPendingPath(); got != filepath.Join(tmpDir, "traffic-pending.json") {
		t.Fatalf("unexpected traffic pending path: %s", got)
	}
}
```

- [ ] **Step 2: Run the config test subset and confirm it fails**

Run:

```bash
go test ./config -run 'Test(GetNodeConfigFromJSONDefaults|ValidateNodeConfigWorkerRequiresNodeID|ValidateNodeConfigWorkerRequiresMariaDB|SharedRuntimeFilePaths)' -v
```

Expected: FAIL with undefined `NodeConfig`, `NodeRoleMaster`, `GetNodeConfigFromJSON`, `ValidateNodeConfig`, `GetSharedCachePath`, or `GetTrafficPendingPath`.

- [ ] **Step 3: Implement node config and runtime path helpers**

Add these types and helpers in `config/config.go`:

```go
type NodeRole string

const (
	NodeRoleMaster NodeRole = "master"
	NodeRoleWorker NodeRole = "worker"
)

type NodeConfig struct {
	Role                NodeRole
	NodeID              string
	SyncIntervalSeconds int
	TrafficFlushSeconds int
}

func GetSharedCachePath() string {
	return filepath.Join(GetDBFolderPath(), "shared-cache.json")
}

func GetTrafficPendingPath() string {
	return filepath.Join(GetDBFolderPath(), "traffic-pending.json")
}

func readGroupedInt(settings map[string]any, key string, fallback int) int {
	readInt := func(value any) (int, bool) {
		switch v := value.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case string:
			i, err := strconv.Atoi(v)
			if err == nil {
				return i, true
			}
		}
		return 0, false
	}
	if groups, ok := settingGroupAliases[key]; ok {
		for _, groupName := range groups {
			if group, ok := settings[groupName].(map[string]any); ok {
				if value, ok := readInt(group[key]); ok {
					return value
				}
			}
		}
	}
	if value, ok := readInt(settings[key]); ok {
		return value
	}
	return fallback
}

func GetNodeConfigFromJSON() NodeConfig {
	data, err := os.ReadFile(GetSettingPath())
	if err != nil {
		return NodeConfig{Role: NodeRoleMaster, SyncIntervalSeconds: 30, TrafficFlushSeconds: 10}
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return NodeConfig{Role: NodeRoleMaster, SyncIntervalSeconds: 30, TrafficFlushSeconds: 10}
	}
	role := readGroupedString(settings, "nodeRole")
	if role == "" {
		role = string(NodeRoleMaster)
	}
	return NodeConfig{
		Role:                NodeRole(role),
		NodeID:              readGroupedString(settings, "nodeId"),
		SyncIntervalSeconds: readGroupedInt(settings, "syncInterval", 30),
		TrafficFlushSeconds: readGroupedInt(settings, "trafficFlushInterval", 10),
	}
}

func ValidateNodeConfig(nodeCfg NodeConfig, dbCfg DBConfig) error {
	switch nodeCfg.Role {
	case NodeRoleMaster, NodeRoleWorker:
	default:
		return fmt.Errorf("invalid nodeRole %q", nodeCfg.Role)
	}
	if nodeCfg.Role == NodeRoleWorker && nodeCfg.NodeID == "" {
		return fmt.Errorf("worker mode requires nodeId")
	}
	if nodeCfg.Role == NodeRoleWorker && dbCfg.Type != "mariadb" {
		return fmt.Errorf("worker mode requires mariadb")
	}
	if nodeCfg.SyncIntervalSeconds <= 0 {
		return fmt.Errorf("syncInterval must be positive")
	}
	if nodeCfg.TrafficFlushSeconds <= 0 {
		return fmt.Errorf("trafficFlushInterval must be positive")
	}
	return nil
}
```

Also extend `settingGroupAliases` so these keys can be read from both top-level JSON and the legacy `other` group:

```go
"nodeRole":             {"other"},
"nodeId":               {"other"},
"syncInterval":         {"other"},
"trafficFlushInterval": {"other"},
```

- [ ] **Step 4: Validate node config at startup and expose CLI setters**

Patch `main.go` in two places.

At startup, validate config before DB init:

```go
func runWebServer() {
	log.Printf("Starting %v %v", config.GetName(), config.GetVersion())

	dbCfg := config.GetDBConfigFromJSON()
	nodeCfg := config.GetNodeConfigFromJSON()
	if err := config.ValidateNodeConfig(nodeCfg, dbCfg); err != nil {
		log.Fatalf("invalid node configuration: %v", err)
	}

	switch config.GetLogLevel() {
```

Extend `setting` flags and `setting -show` output:

```go
var nodeRoleFlag string
var nodeIDFlag string
var syncIntervalFlag int
var trafficFlushIntervalFlag int

settingCmd.StringVar(&nodeRoleFlag, "nodeRole", "", "Set node role (master or worker)")
settingCmd.StringVar(&nodeIDFlag, "nodeId", "", "Set node identifier")
settingCmd.IntVar(&syncIntervalFlag, "syncInterval", 0, "Set shared sync interval in seconds")
settingCmd.IntVar(&trafficFlushIntervalFlag, "trafficFlushInterval", 0, "Set traffic flush interval in seconds")
```

```go
func showSetting(show bool) {
	if show {
		nodeCfg := config.GetNodeConfigFromJSON()
		fmt.Println("nodeRole:", nodeCfg.Role)
		fmt.Println("nodeId:", nodeCfg.NodeID)
		fmt.Println("syncInterval:", nodeCfg.SyncIntervalSeconds)
		fmt.Println("trafficFlushInterval:", nodeCfg.TrafficFlushSeconds)
	}
}
```

When setters are used, validate before writing:

```go
candidate := config.GetNodeConfigFromJSON()
if nodeRoleFlag != "" {
	candidate.Role = config.NodeRole(nodeRoleFlag)
}
if nodeIDFlag != "" {
	candidate.NodeID = nodeIDFlag
}
if syncIntervalFlag > 0 {
	candidate.SyncIntervalSeconds = syncIntervalFlag
}
if trafficFlushIntervalFlag > 0 {
	candidate.TrafficFlushSeconds = trafficFlushIntervalFlag
}
if err := config.ValidateNodeConfig(candidate, config.GetDBConfigFromJSON()); err != nil {
	fmt.Println("Invalid node settings:", err)
	return
}
```

Then persist with `WriteSettingToJSON`:

```go
if nodeRoleFlag != "" {
	_ = config.WriteSettingToJSON("nodeRole", nodeRoleFlag)
}
if nodeIDFlag != "" {
	_ = config.WriteSettingToJSON("nodeId", nodeIDFlag)
}
if syncIntervalFlag > 0 {
	_ = config.WriteSettingToJSON("syncInterval", strconv.Itoa(syncIntervalFlag))
}
if trafficFlushIntervalFlag > 0 {
	_ = config.WriteSettingToJSON("trafficFlushInterval", strconv.Itoa(trafficFlushIntervalFlag))
}
```

- [ ] **Step 5: Run config tests and package discovery**

Run:

```bash
go test ./config -run 'Test(GetNodeConfigFromJSONDefaults|ValidateNodeConfigWorkerRequiresNodeID|ValidateNodeConfigWorkerRequiresMariaDB|SharedRuntimeFilePaths|GetDBConfigFromJSONSupportsModulePurposeLayout|WriteSettingToJSONUsesModulePurposeGroup)' -v
go test ./... -run TestNonExistent -count=0
```

Expected:

- the focused `./config` tests PASS
- package discovery succeeds without running unrelated tests

- [ ] **Step 6: Checkpoint Commit the config work**

Run:

```bash
git add config/config.go config/config_test.go main.go
git commit -m "feat: add node config and startup validation"
```

After commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`:

- mark Task 1 complete
- record the short commit hash beside Task 1

---

### Task 2: Add shared metadata models and repository helpers

**Files:**
- Create: `database/model/shared_state.go`
- Create: `database/model/node_state.go`
- Create: `database/shared_state.go`
- Modify: `database/db.go`
- Modify: `database/db_test.go`
**Execution Mode:** Inline

- [ ] **Step 1: Write the failing database tests**

Add these tests to `database/db_test.go`:

```go
func TestInitDB_CreatesSharedMetadataTables(t *testing.T) {
	setupTestDB(t)

	for _, table := range []string{"shared_states", "node_states"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("table %s should exist: %v", table, err)
		}
	}
}

func TestBumpSharedAccountsVersion(t *testing.T) {
	setupTestDB(t)

	version, err := GetSharedAccountsVersion(GetDB())
	if err != nil {
		t.Fatalf("GetSharedAccountsVersion error: %v", err)
	}
	if version != 0 {
		t.Fatalf("expected seeded version 0, got %d", version)
	}

	tx := GetDB().Begin()
	if err := BumpSharedAccountsVersion(tx); err != nil {
		t.Fatalf("BumpSharedAccountsVersion error: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("Commit error: %v", err)
	}

	version, err = GetSharedAccountsVersion(GetDB())
	if err != nil {
		t.Fatalf("GetSharedAccountsVersion error: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected bumped version 1, got %d", version)
	}
}

func TestUpsertNodeState(t *testing.T) {
	setupTestDB(t)

	state := &model.NodeState{
		NodeID:          "worker-1",
		NodeRole:        "worker",
		LastSeenVersion: 7,
		LastError:       "dial tcp timeout",
	}
	if err := UpsertNodeState(GetDB(), state); err != nil {
		t.Fatalf("UpsertNodeState error: %v", err)
	}

	var stored model.NodeState
	if err := GetDB().First(&stored, "node_id = ?", "worker-1").Error; err != nil {
		t.Fatalf("lookup node state failed: %v", err)
	}
	if stored.LastSeenVersion != 7 {
		t.Fatalf("expected last seen version 7, got %d", stored.LastSeenVersion)
	}
	if stored.LastError != "dial tcp timeout" {
		t.Fatalf("expected last error to round-trip, got %q", stored.LastError)
	}
}
```

- [ ] **Step 2: Run the database test subset and confirm it fails**

Run:

```bash
go test ./database -run 'Test(InitDB_CreatesSharedMetadataTables|BumpSharedAccountsVersion|UpsertNodeState)' -v
```

Expected: FAIL with missing `model.NodeState`, `GetSharedAccountsVersion`, `BumpSharedAccountsVersion`, or `UpsertNodeState`.

- [ ] **Step 3: Add metadata models and DB helpers**

Create `database/model/shared_state.go`:

```go
package model

type SharedState struct {
	Key       string `json:"key" gorm:"primaryKey"`
	Version   int64  `json:"version" gorm:"not null;default:0"`
	UpdatedAt int64  `json:"updatedAt"`
}
```

Create `database/model/node_state.go`:

```go
package model

type NodeState struct {
	NodeID          string `json:"nodeId" gorm:"primaryKey"`
	NodeRole        string `json:"nodeRole" gorm:"not null"`
	LastSyncAt      int64  `json:"lastSyncAt"`
	LastHeartbeatAt int64  `json:"lastHeartbeatAt"`
	LastSeenVersion int64  `json:"lastSeenVersion"`
	LastError       string `json:"lastError"`
	UpdatedAt       int64  `json:"updatedAt"`
}
```

Create `database/shared_state.go`:

```go
package database

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"gorm.io/gorm"
)

const SharedAccountsVersionKey = "shared_accounts_version"

func txOrDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return GetDB()
}

func seedSharedAccountsVersion(tx *gorm.DB) error {
	return txOrDB(tx).FirstOrCreate(&model.SharedState{
		Key:       SharedAccountsVersionKey,
		Version:   0,
		UpdatedAt: time.Now().Unix(),
	}, &model.SharedState{Key: SharedAccountsVersionKey}).Error
}

func GetSharedAccountsVersion(tx *gorm.DB) (int64, error) {
	state := &model.SharedState{}
	err := txOrDB(tx).First(state, "key = ?", SharedAccountsVersionKey).Error
	if err != nil {
		return 0, err
	}
	return state.Version, nil
}

func BumpSharedAccountsVersion(tx *gorm.DB) error {
	now := time.Now().Unix()
	return txOrDB(tx).Model(&model.SharedState{}).
		Where("key = ?", SharedAccountsVersionKey).
		Updates(map[string]any{
			"version":    gorm.Expr("version + 1"),
			"updated_at": now,
		}).Error
}

func UpsertNodeState(tx *gorm.DB, state *model.NodeState) error {
	state.UpdatedAt = time.Now().Unix()
	return txOrDB(tx).Save(state).Error
}
```

- [ ] **Step 4: Register metadata models and seed the shared version row**

Patch `database/db.go`:

```go
func initModels() error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&model.SharedState{},
		&model.NodeState{},
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return err
		}
	}
	if err := seedSharedAccountsVersion(db); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 5: Run the database tests**

Run:

```bash
go test ./database -run 'Test(InitDB_CreatesSharedMetadataTables|BumpSharedAccountsVersion|UpsertNodeState|InitDB_CreatesTables|InitDB_Idempotent)' -v
```

Expected: PASS

- [ ] **Step 6: Checkpoint Commit the metadata work**

Run:

```bash
git add database/model/shared_state.go database/model/node_state.go database/shared_state.go database/db.go database/db_test.go
git commit -m "feat: add shared metadata models and helpers"
```

After commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`:

- mark Task 2 complete
- record the short commit hash beside Task 2

---

### Task 3: Enforce master-only shared writes and transactional version bumping

**Files:**
- Create: `web/service/node_guard.go`
- Create: `web/service/node_guard_test.go`
- Modify: `web/service/inbound.go`
**Execution Mode:** Inline

- [ ] **Step 1: Write the failing node-guard tests**

Create `web/service/node_guard_test.go` with:

```go
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
)

func writeNodeGuardSettings(t *testing.T, settings map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	if err := os.WriteFile(config.GetSettingPath(), data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func setupNodeGuardDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	if err := database.InitDBWithPath(filepath.Join(tmpDir, "service.db")); err != nil {
		t.Fatalf("InitDBWithPath error: %v", err)
	}
	t.Cleanup(func() { database.CloseDB() })
}

func TestRequireMasterRejectsWorker(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	writeNodeGuardSettings(t, map[string]any{
		"dbType":   "mariadb",
		"nodeRole": "worker",
		"nodeId":   "worker-1",
	})

	if err := RequireMaster(); err == nil {
		t.Fatal("expected worker mode to be rejected")
	}
}

func TestRequireMasterAllowsMaster(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	writeNodeGuardSettings(t, map[string]any{
		"dbType":   "mariadb",
		"nodeRole": "master",
	})

	if err := RequireMaster(); err != nil {
		t.Fatalf("expected master mode to pass: %v", err)
	}
}

func TestBumpSharedAccountsVersionRollsBackWithTransaction(t *testing.T) {
	setupNodeGuardDB(t)

	tx := database.GetDB().Begin()
	if err := database.BumpSharedAccountsVersion(tx); err != nil {
		t.Fatalf("BumpSharedAccountsVersion error: %v", err)
	}
	tx.Rollback()

	version, err := database.GetSharedAccountsVersion(database.GetDB())
	if err != nil {
		t.Fatalf("GetSharedAccountsVersion error: %v", err)
	}
	if version != 0 {
		t.Fatalf("expected rolled-back version to remain 0, got %d", version)
	}
}
```

- [ ] **Step 2: Run the guard tests and confirm they fail**

Run:

```bash
go test ./web/service -run 'Test(RequireMasterRejectsWorker|RequireMasterAllowsMaster|BumpSharedAccountsVersionRollsBackWithTransaction)' -v
```

Expected: FAIL with undefined `RequireMaster`.

- [ ] **Step 3: Add shared-write guard helpers**

Create `web/service/node_guard.go`:

```go
package service

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v2/config"
)

var ErrSharedWriteRequiresMaster = errors.New("shared-account writes are only allowed on master nodes")

func IsWorker() bool {
	return config.GetNodeConfigFromJSON().Role == config.NodeRoleWorker
}

func IsMaster() bool {
	return !IsWorker()
}

func RequireMaster() error {
	if IsWorker() {
		return ErrSharedWriteRequiresMaster
	}
	return nil
}

func IsSharedModeEnabled() bool {
	return config.GetDBConfigFromJSON().Type == "mariadb"
}
```

- [ ] **Step 4: Guard shared writes and bump shared version inside successful transactions**

Patch `web/service/inbound.go` with one reusable helper:

```go
func ensureSharedWriteAllowed() error {
	return RequireMaster()
}

func bumpSharedVersion(tx *gorm.DB) error {
	return database.BumpSharedAccountsVersion(tx)
}
```

Apply this prologue to shared mutators:

```go
if err := ensureSharedWriteAllowed(); err != nil {
	return nil, false, err
}
```

Apply the version bump immediately before commit in:

- `AddInbound`
- `UpdateInbound`
- `DelInbound`
- `AddInboundClient`
- `DelInboundClient`
- `UpdateInboundClient`
- `ResetClientTraffic`
- `ResetAllTraffics`
- `DelDepletedClients`
- `DelInboundClientByEmail`

Use the same pattern in each method’s existing transaction, preserving that method’s current return type:

```go
if err := tx.Save(oldInbound).Error; err != nil {
	return false, err
}
if err := bumpSharedVersion(tx); err != nil {
	return false, err
}
return needRestart, nil
```

Do not change controller behavior in this task. Let the existing controller paths surface the service-layer error message.

- [ ] **Step 5: Run the node-guard tests**

Run:

```bash
go test ./web/service -run 'Test(RequireMasterRejectsWorker|RequireMasterAllowsMaster|BumpSharedAccountsVersionRollsBackWithTransaction)' -v
```

Expected: PASS

- [ ] **Step 6: Checkpoint Commit the guard work**

Run:

```bash
git add web/service/node_guard.go web/service/node_guard_test.go web/service/inbound.go
git commit -m "feat: guard shared writes and bump version transactionally"
```

After commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`:

- mark Task 3 complete
- record the short commit hash beside Task 3

---

### Task 4: Add shared snapshot cache, worker sync loop, and snapshot-driven Xray rebuild

**Files:**
- Create: `web/service/node_cache.go`
- Create: `web/service/node_sync.go`
- Create: `web/service/node_sync_test.go`
- Modify: `web/service/xray.go`
- Modify: `web/web.go`
**Execution Mode:** Subagent-Driven

- [ ] **Step 1: Write the failing snapshot and sync tests**

Create `web/service/node_sync_test.go`:

```go
package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

func TestLoadAndSaveSharedAccountsSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared-cache.json")
	snapshot := &SharedAccountsSnapshot{
		Version: 2,
		Inbounds: []*model.Inbound{
			{Id: 1, Tag: "inbound-443", Enable: true, Port: 443},
		},
	}

	if err := SaveSharedAccountsSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSharedAccountsSnapshot error: %v", err)
	}

	loaded, err := LoadSharedAccountsSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSharedAccountsSnapshot error: %v", err)
	}
	if loaded.Version != 2 || len(loaded.Inbounds) != 1 {
		t.Fatalf("unexpected snapshot round-trip: %+v", loaded)
	}
}

func TestSyncOnceSkipsApplyWhenVersionUnchanged(t *testing.T) {
	applyCalls := 0
	svc := &NodeSyncService{
		cachePath: filepath.Join(t.TempDir(), "shared-cache.json"),
		loadVersion: func() (int64, error) { return 3, nil },
		loadSnapshot: func(int64) (*SharedAccountsSnapshot, error) {
			t.Fatal("loadSnapshot should not run when version is unchanged")
			return nil, nil
		},
		applySnapshot: func(*SharedAccountsSnapshot) error {
			applyCalls++
			return nil
		},
		lastSeenVersion: 3,
	}

	if err := svc.SyncOnce(); err != nil {
		t.Fatalf("SyncOnce error: %v", err)
	}
	if applyCalls != 0 {
		t.Fatalf("expected applySnapshot to be skipped, got %d calls", applyCalls)
	}
}

func TestSyncOnceRefreshesCacheAndAppliesSnapshot(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "shared-cache.json")
	applyCalls := 0
	svc := &NodeSyncService{
		cachePath: cachePath,
		loadVersion: func() (int64, error) { return 4, nil },
		loadSnapshot: func(version int64) (*SharedAccountsSnapshot, error) {
			return &SharedAccountsSnapshot{
				Version: version,
				Inbounds: []*model.Inbound{
					{Id: 7, Tag: "worker-8443", Enable: true, Port: 8443},
				},
			}, nil
		},
		applySnapshot: func(snapshot *SharedAccountsSnapshot) error {
			applyCalls++
			if snapshot.Version != 4 {
				t.Fatalf("expected snapshot version 4, got %d", snapshot.Version)
			}
			return nil
		},
	}

	if err := svc.SyncOnce(); err != nil {
		t.Fatalf("SyncOnce error: %v", err)
	}
	if applyCalls != 1 {
		t.Fatalf("expected one apply call, got %d", applyCalls)
	}

	loaded, err := LoadSharedAccountsSnapshot(cachePath)
	if err != nil {
		t.Fatalf("LoadSharedAccountsSnapshot error: %v", err)
	}
	if loaded.Version != 4 {
		t.Fatalf("expected cached version 4, got %d", loaded.Version)
	}
}

func TestBootstrapFromCacheAppliesCachedSnapshot(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "shared-cache.json")
	if err := SaveSharedAccountsSnapshot(cachePath, &SharedAccountsSnapshot{
		Version: 5,
		Inbounds: []*model.Inbound{
			{Id: 9, Tag: "cached-9443", Enable: true, Port: 9443},
		},
	}); err != nil {
		t.Fatalf("SaveSharedAccountsSnapshot error: %v", err)
	}

	applyCalls := 0
	svc := &NodeSyncService{
		cachePath: cachePath,
		applySnapshot: func(snapshot *SharedAccountsSnapshot) error {
			applyCalls++
			if snapshot.Version != 5 {
				t.Fatalf("expected cached version 5, got %d", snapshot.Version)
			}
			return nil
		},
	}

	if err := svc.BootstrapFromCache(); err != nil {
		t.Fatalf("BootstrapFromCache error: %v", err)
	}
	if applyCalls != 1 {
		t.Fatalf("expected one cached apply, got %d", applyCalls)
	}
}
```

- [ ] **Step 2: Run the sync test subset and confirm it fails**

Run:

```bash
go test ./web/service -run 'Test(LoadAndSaveSharedAccountsSnapshot|SyncOnceSkipsApplyWhenVersionUnchanged|SyncOnceRefreshesCacheAndAppliesSnapshot|BootstrapFromCacheAppliesCachedSnapshot)' -v
```

Expected: FAIL with undefined `SharedAccountsSnapshot`, `SaveSharedAccountsSnapshot`, `LoadSharedAccountsSnapshot`, or `NodeSyncService`.

- [ ] **Step 3: Add cache helpers and refactor Xray config building away from DB reads**

Create `web/service/node_cache.go`:

```go
package service

import (
	"encoding/json"
	"os"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type SharedAccountsSnapshot struct {
	Version  int64             `json:"version"`
	Inbounds []*model.Inbound  `json:"inbounds"`
}

func LoadSharedAccountsSnapshot(path string) (*SharedAccountsSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	snapshot := &SharedAccountsSnapshot{}
	if err := json.Unmarshal(data, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func SaveSharedAccountsSnapshot(path string, snapshot *SharedAccountsSnapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

Refactor `web/service/xray.go` so worker sync can build from cached inbounds:

```go
func (s *XrayService) BuildConfigFromInbounds(inbounds []*model.Inbound) (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	if err := json.Unmarshal([]byte(templateConfig), xrayConfig); err != nil {
		return nil, err
	}

	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		// move the existing settings/streamSettings normalization logic here
		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

func (s *XrayService) RestartXrayWithConfig(xrayConfig *xray.Config, isForce bool) error {
	lock.Lock()
	defer lock.Unlock()
	isManuallyStopped.Store(false)

	if s.IsXrayRunning() {
		if !isForce && p.GetConfig().Equals(xrayConfig) && !isNeedXrayRestart.Load() {
			return nil
		}
		p.Stop()
	}

	p = xray.NewProcess(xrayConfig)
	result = ""
	return p.Start()
}

func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	return s.BuildConfigFromInbounds(inbounds)
}

func (s *XrayService) ApplySharedSnapshot(snapshot *SharedAccountsSnapshot) error {
	xrayConfig, err := s.BuildConfigFromInbounds(snapshot.Inbounds)
	if err != nil {
		return err
	}
	return s.RestartXrayWithConfig(xrayConfig, false)
}
```

- [ ] **Step 4: Implement the node sync service**

Create `web/service/node_sync.go`:

```go
package service

import (
	"context"
	"os"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type NodeSyncService struct {
	xrayService     XrayService
	cachePath       string
	lastSeenVersion int64
	loadVersion     func() (int64, error)
	loadSnapshot    func(int64) (*SharedAccountsSnapshot, error)
	applySnapshot   func(*SharedAccountsSnapshot) error
}

func NewNodeSyncService() *NodeSyncService {
	svc := &NodeSyncService{
		cachePath: config.GetSharedCachePath(),
	}
	svc.loadVersion = func() (int64, error) {
		return database.GetSharedAccountsVersion(database.GetDB())
	}
	svc.loadSnapshot = func(version int64) (*SharedAccountsSnapshot, error) {
		inbounds, err := svc.xrayService.inboundService.GetAllInbounds()
		if err != nil {
			return nil, err
		}
		return &SharedAccountsSnapshot{Version: version, Inbounds: inbounds}, nil
	}
	svc.applySnapshot = svc.xrayService.ApplySharedSnapshot
	return svc
}

func (s *NodeSyncService) updateNodeState(version int64, syncErr error, didSync bool) {
	nodeCfg := config.GetNodeConfigFromJSON()
	now := time.Now().Unix()
	state := &model.NodeState{
		NodeID:          nodeCfg.NodeID,
		NodeRole:        string(nodeCfg.Role),
		LastHeartbeatAt: now,
		LastSeenVersion: version,
	}
	if didSync {
		state.LastSyncAt = now
	}
	if syncErr != nil {
		state.LastError = syncErr.Error()
	}
	_ = database.UpsertNodeState(database.GetDB(), state)
}

func (s *NodeSyncService) BootstrapFromCache() error {
	snapshot, err := LoadSharedAccountsSnapshot(s.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	s.lastSeenVersion = snapshot.Version
	return s.applySnapshot(snapshot)
}

func (s *NodeSyncService) SyncOnce() error {
	version, err := s.loadVersion()
	if err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return err
	}
	if version == s.lastSeenVersion {
		s.updateNodeState(version, nil, false)
		return nil
	}

	snapshot, err := s.loadSnapshot(version)
	if err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return err
	}
	if err := SaveSharedAccountsSnapshot(s.cachePath, snapshot); err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return err
	}
	if err := s.applySnapshot(snapshot); err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return err
	}

	s.lastSeenVersion = version
	s.updateNodeState(version, nil, true)
	return nil
}

func (s *NodeSyncService) Run(ctx context.Context, interval time.Duration) {
	_ = s.BootstrapFromCache()
	_ = s.SyncOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.SyncOnce()
		}
	}
}

func (s *NodeSyncService) RunHeartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			version, _ := database.GetSharedAccountsVersion(database.GetDB())
			s.updateNodeState(version, nil, false)
		}
	}
}
```

- [ ] **Step 5: Start worker sync or master heartbeat on server startup**

Patch `web/web.go`:

```go
func (s *Server) startNodeLoops() {
	nodeCfg := config.GetNodeConfigFromJSON()
	nodeSyncService := service.NewNodeSyncService()
	interval := time.Duration(nodeCfg.SyncIntervalSeconds) * time.Second

	if nodeCfg.Role == config.NodeRoleWorker {
		go nodeSyncService.Run(s.ctx, interval)
		return
	}
	if nodeCfg.NodeID != "" {
		go nodeSyncService.RunHeartbeatLoop(s.ctx, interval)
	}
}
```

Call it from `Start()` after `s.startTask()`:

```go
s.startTask()
s.startNodeLoops()
```

- [ ] **Step 6: Run the sync tests**

Run:

```bash
go test ./web/service -run 'Test(LoadAndSaveSharedAccountsSnapshot|SyncOnceSkipsApplyWhenVersionUnchanged|SyncOnceRefreshesCacheAndAppliesSnapshot|BootstrapFromCacheAppliesCachedSnapshot)' -v
```

Expected: PASS

- [ ] **Step 7: Checkpoint Commit the sync work**

Run:

```bash
git add web/service/node_cache.go web/service/node_sync.go web/service/node_sync_test.go web/service/xray.go web/web.go
git commit -m "feat: add cache-backed worker sync and heartbeat loops"
```

After commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`:

- mark Task 4 complete
- record the short commit hash beside Task 4

---

### Task 5: Add durable traffic delta persistence and safe shared-mode flushes

**Files:**
- Create: `web/service/traffic_pending.go`
- Create: `web/service/traffic_flush.go`
- Create: `web/service/traffic_flush_test.go`
- Modify: `web/job/xray_traffic_job.go`
- Modify: `web/service/inbound.go`
- Modify: `web/web.go`
- Modify: `xray/client_traffic.go`
**Execution Mode:** Subagent-Driven

- [ ] **Step 1: Write the failing pending-delta and flush tests**

Create `web/service/traffic_flush_test.go`:

```go
package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

func TestTrafficPendingStoreMerge(t *testing.T) {
	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))

	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", UpDelta: 7}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", DownDelta: 9}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected one merged delta, got %d", len(deltas))
	}
	if deltas[0].UpDelta != 7 || deltas[0].DownDelta != 9 {
		t.Fatalf("unexpected merged delta: %+v", deltas[0])
	}
}

func TestFlushOnceClearsPendingOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	if err := database.InitDBWithPath(filepath.Join(tmpDir, "flush.db")); err != nil {
		t.Fatalf("InitDBWithPath error: %v", err)
	}
	defer database.CloseDB()

	if err := database.GetDB().Create(&model.Inbound{Id: 1, Tag: "inbound-443", Enable: true}).Error; err != nil {
		t.Fatalf("seed inbound failed: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: 1, Email: "alice@example.com"}).Error; err != nil {
		t.Fatalf("seed client traffic failed: %v", err)
	}

	store := NewTrafficPendingStore(filepath.Join(tmpDir, "traffic-pending.json"))
	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", UpDelta: 7, DownDelta: 9}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	svc := NewTrafficFlushService(store)
	if err := svc.FlushOnce(); err != nil {
		t.Fatalf("FlushOnce error: %v", err)
	}

	var clientTraffic xray.ClientTraffic
	if err := database.GetDB().First(&clientTraffic, "inbound_id = ? AND email = ?", 1, "alice@example.com").Error; err != nil {
		t.Fatalf("lookup client traffic failed: %v", err)
	}
	if clientTraffic.Up != 7 || clientTraffic.Down != 9 {
		t.Fatalf("unexpected flushed traffic: %+v", clientTraffic)
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 0 {
		t.Fatalf("expected pending deltas to be cleared, got %+v", deltas)
	}
}

func TestFlushOnceKeepsPendingOnFailure(t *testing.T) {
	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))
	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", UpDelta: 3}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	svc := NewTrafficFlushService(store)
	svc.flushFn = func([]TrafficDelta) error { return errors.New("boom") }

	if err := svc.FlushOnce(); err == nil {
		t.Fatal("expected flush failure")
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected pending delta to remain, got %+v", deltas)
	}
}
```

- [ ] **Step 2: Run the flush test subset and confirm it fails**

Run:

```bash
go test ./web/service -run 'Test(TrafficPendingStoreMerge|FlushOnceClearsPendingOnSuccess|FlushOnceKeepsPendingOnFailure)' -v
```

Expected: FAIL with undefined `TrafficDelta`, `NewTrafficPendingStore`, or `NewTrafficFlushService`.

- [ ] **Step 3: Implement the pending-delta store and add the composite unique key**

Create `web/service/traffic_pending.go`:

```go
package service

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type TrafficDelta struct {
	InboundID int    `json:"inboundId"`
	Email     string `json:"email"`
	UpDelta   int64  `json:"upDelta"`
	DownDelta int64  `json:"downDelta"`
}

type TrafficPendingStore struct {
	path string
	mu   sync.Mutex
}

func NewTrafficPendingStore(path string) *TrafficPendingStore {
	return &TrafficPendingStore{path: path}
}

func (s *TrafficPendingStore) Load() ([]TrafficDelta, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []TrafficDelta{}, nil
	}
	if err != nil {
		return nil, err
	}
	var deltas []TrafficDelta
	if err := json.Unmarshal(data, &deltas); err != nil {
		return nil, err
	}
	return deltas, nil
}

func (s *TrafficPendingStore) Save(deltas []TrafficDelta) error {
	data, err := json.MarshalIndent(deltas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *TrafficPendingStore) Merge(newDeltas []TrafficDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.Load()
	if err != nil {
		return err
	}
	index := map[string]int{}
	for i, delta := range current {
		index[deltaKey(delta.InboundID, delta.Email)] = i
	}
	for _, delta := range newDeltas {
		key := deltaKey(delta.InboundID, delta.Email)
		if idx, ok := index[key]; ok {
			current[idx].UpDelta += delta.UpDelta
			current[idx].DownDelta += delta.DownDelta
			continue
		}
		index[key] = len(current)
		current = append(current, delta)
	}
	return s.Save(current)
}

func deltaKey(inboundID int, email string) string {
	return fmt.Sprintf("%d:%s", inboundID, email)
}
```

Patch `xray/client_traffic.go` so shared flushes can use deterministic upserts:

```go
type ClientTraffic struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" form:"inboundId" gorm:"uniqueIndex:idx_client_traffics_inbound_email"`
	Enable     bool   `json:"enable" form:"enable"`
	Email      string `json:"email" form:"email" gorm:"uniqueIndex:idx_client_traffics_inbound_email"`
```

- [ ] **Step 4: Implement shared-mode flushes and master-only traffic reconciliation**

Create `web/service/traffic_flush.go`:

```go
package service

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TrafficFlushService struct {
	store    *TrafficPendingStore
	inbounds InboundService
	flushFn  func([]TrafficDelta) error
}

func NewTrafficFlushService(store *TrafficPendingStore) *TrafficFlushService {
	svc := &TrafficFlushService{store: store}
	svc.flushFn = svc.flushToDatabase
	return svc
}

func (s *TrafficFlushService) Collect(clientTraffics []*xray.ClientTraffic) error {
	deltas := make([]TrafficDelta, 0, len(clientTraffics))
	for _, traffic := range clientTraffics {
		if traffic.Up == 0 && traffic.Down == 0 {
			continue
		}
		deltas = append(deltas, TrafficDelta{
			InboundID: traffic.InboundId,
			Email:     traffic.Email,
			UpDelta:   traffic.Up,
			DownDelta: traffic.Down,
		})
	}
	return s.store.Merge(deltas)
}

func (s *TrafficFlushService) flushToDatabase(deltas []TrafficDelta) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, delta := range deltas {
			if err := tx.Model(&model.Inbound{}).
				Where("id = ?", delta.InboundID).
				Updates(map[string]any{
					"up":       gorm.Expr("up + ?", delta.UpDelta),
					"down":     gorm.Expr("down + ?", delta.DownDelta),
					"all_time": gorm.Expr("COALESCE(all_time, 0) + ?", delta.UpDelta+delta.DownDelta),
				}).Error; err != nil {
				return err
			}

			row := xray.ClientTraffic{
				InboundId: delta.InboundID,
				Email:     delta.Email,
				Up:        delta.UpDelta,
				Down:      delta.DownDelta,
				AllTime:   delta.UpDelta + delta.DownDelta,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "inbound_id"}, {Name: "email"}},
				DoUpdates: clause.Assignments(map[string]any{
					"up":       gorm.Expr("up + ?", delta.UpDelta),
					"down":     gorm.Expr("down + ?", delta.DownDelta),
					"all_time": gorm.Expr("all_time + ?", delta.UpDelta+delta.DownDelta),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}

		if IsMaster() {
			_, err := s.inbounds.ReconcileSharedTrafficState(tx)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *TrafficFlushService) FlushOnce() error {
	deltas, err := s.store.Load()
	if err != nil || len(deltas) == 0 {
		return err
	}
	if err := s.flushFn(deltas); err != nil {
		return err
	}
	return s.store.Save([]TrafficDelta{})
}

func (s *TrafficFlushService) Run(ctx context.Context) {
	interval := time.Duration(config.GetNodeConfigFromJSON().TrafficFlushSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = s.FlushOnce()
			return
		case <-ticker.C:
			_ = s.FlushOnce()
		}
	}
}
```

Extract master-only reconciliation from `web/service/inbound.go`:

```go
func (s *InboundService) ReconcileSharedTrafficState(tx *gorm.DB) (bool, error) {
	needRestart0, _, err := s.autoRenewClients(tx)
	if err != nil {
		return false, err
	}
	needRestart1, _, err := s.disableInvalidClients(tx)
	if err != nil {
		return false, err
	}
	needRestart2, _, err := s.disableInvalidInbounds(tx)
	if err != nil {
		return false, err
	}
	return needRestart0 || needRestart1 || needRestart2, nil
}
```

Do not let worker shared-mode traffic processing call `AddTraffic()`, because that path mutates shared enable/expiry state.

- [ ] **Step 5: Route shared-mode traffic collection through the pending store and start the flush loop**

Patch `web/job/xray_traffic_job.go`:

```go
type XrayTrafficJob struct {
	settingService   service.SettingService
	xrayService      service.XrayService
	inboundService   service.InboundService
	outboundService  service.OutboundService
	trafficFlushSvc  *service.TrafficFlushService
}

func NewXrayTrafficJob() *XrayTrafficJob {
	return &XrayTrafficJob{
		trafficFlushSvc: service.NewTrafficFlushService(
			service.NewTrafficPendingStore(config.GetTrafficPendingPath()),
		),
	}
}
```

In `Run()`, branch on shared mode:

```go
if service.IsSharedModeEnabled() {
	if err := j.trafficFlushSvc.Collect(clientTraffics); err != nil {
		logger.Warning("collect shared traffic failed:", err)
	}
} else {
	err, needRestart0 := j.inboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add inbound traffic failed:", err)
	}
	if needRestart0 {
		j.xrayService.SetToNeedRestart()
	}
}
```

Start the flush loop in `web/web.go`:

```go
func (s *Server) startTrafficFlushLoop() {
	store := service.NewTrafficPendingStore(config.GetTrafficPendingPath())
	flushService := service.NewTrafficFlushService(store)
	go flushService.Run(s.ctx)
}
```

Call it from `Start()` after `s.startNodeLoops()`:

```go
s.startTrafficFlushLoop()
```

- [ ] **Step 6: Run the flush tests and package discovery**

Run:

```bash
go test ./web/service -run 'Test(TrafficPendingStoreMerge|FlushOnceClearsPendingOnSuccess|FlushOnceKeepsPendingOnFailure)' -v
go test ./... -run TestNonExistent -count=0
```

Expected:

- the focused flush tests PASS
- package discovery still succeeds after the new service wiring

- [ ] **Step 7: Checkpoint Commit the shared traffic work**

Run:

```bash
git add web/service/traffic_pending.go web/service/traffic_flush.go web/service/traffic_flush_test.go web/job/xray_traffic_job.go web/service/inbound.go web/web.go xray/client_traffic.go
git commit -m "feat: add durable traffic deltas and shared flush loop"
```

After commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`:

- mark Task 5 complete
- record the short commit hash beside Task 5

---

### Task 6: Expose node management in shell tools and the installer

**Files:**
- Modify: `x-ui.sh`
- Modify: `install.sh`
**Execution Mode:** Subagent-Driven

- [ ] **Step 1: Add read helpers and node status display to `x-ui.sh`**

Add reusable JSON readers near the existing DB helpers:

```bash
get_node_setting() {
    local key="$1"
    local default_value="$2"
    local json_path="/etc/x-ui/x-ui.json"
    if [ ! -f "$json_path" ]; then
        echo "$default_value"
        return
    fi
    jq -r "$key // $default_value" "$json_path" 2>/dev/null
}

show_node_status() {
    local node_role
    local node_id
    local sync_interval
    local flush_interval
    node_role=$(get_node_setting '.nodeRole' '"master"')
    node_id=$(get_node_setting '.nodeId' '""')
    sync_interval=$(get_node_setting '.syncInterval' '30')
    flush_interval=$(get_node_setting '.trafficFlushInterval' '10')

    echo "Node role: ${node_role}"
    echo "Node ID: ${node_id:-<empty>}"
    echo "Sync interval: ${sync_interval}s"
    echo "Traffic flush interval: ${flush_interval}s"
}
```

- [ ] **Step 2: Add minimal node-management actions to `x-ui.sh`**

Add menu actions that call the existing Go binary instead of editing JSON directly:

```bash
set_node_role() {
    read -rp "Enter node role (master/worker): " node_role
    if [ "$node_role" != "master" ] && [ "$node_role" != "worker" ]; then
        echo "Invalid node role"
        return 1
    fi
    ${xui_folder}/x-ui setting -nodeRole "$node_role"
}

set_node_id() {
    read -rp "Enter node ID: " node_id
    ${xui_folder}/x-ui setting -nodeId "$node_id"
}
```

Menu text should stay minimal:

- show current node role
- set `master` / `worker`
- set `nodeId`
- remind the operator to restart after changes

- [ ] **Step 3: Prompt for MariaDB and node role during fresh installs**

Patch the fresh-install branch in `install.sh`:

```bash
read -rp "Database type [mariadb]: " db_type
db_type=${db_type:-mariadb}
${xui_folder}/x-ui setting -dbType "$db_type"

if [ "$db_type" = "mariadb" ]; then
    read -rp "MariaDB host [127.0.0.1]: " db_host
    read -rp "MariaDB port [3306]: " db_port
    read -rp "MariaDB user: " db_user
    read -rsp "MariaDB password: " db_pass
    echo
    read -rp "MariaDB database [3xui]: " db_name

    ${xui_folder}/x-ui setting -dbHost "${db_host:-127.0.0.1}" -dbPort "${db_port:-3306}" -dbUser "$db_user" -dbPassword "$db_pass" -dbName "${db_name:-3xui}"
fi

read -rp "Node role [master]: " node_role
node_role=${node_role:-master}

if [ "$node_role" = "worker" ]; then
    read -rp "Node ID: " node_id
    ${xui_folder}/x-ui setting -nodeRole worker -nodeId "$node_id"
else
    ${xui_folder}/x-ui setting -nodeRole master
fi
```

Do not add this prompt path to upgrades. Preserve existing SQLite upgrade behavior for old installs.

- [ ] **Step 4: Run shell syntax checks and a CLI smoke check**

Run:

```bash
bash -n x-ui.sh
bash -n install.sh
./x-ui setting -show true
```

Expected:

- both shell scripts pass `bash -n`
- `./x-ui setting -show true` prints `nodeRole`, `nodeId`, `syncInterval`, and `trafficFlushInterval`

- [ ] **Step 5: Checkpoint Commit the operator tooling work**

Run:

```bash
git add x-ui.sh install.sh
git commit -m "feat: add node management shell and installer flows"
```

After commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`:

- mark Task 6 complete
- record the short commit hash beside Task 6

---

### Task 7: Document the feature and run focused verification

**Files:**
- Create: `docs/multi-node-sync.md`
- Modify: `README.md`
- Modify: `README.zh_CN.md`
**Execution Mode:** Subagent-Driven

- [ ] **Step 1: Write the operator runbook**

Create `docs/multi-node-sync.md` with these sections:

```md
# Multi-Node Shared Control

## Roles

- `master`: the only node allowed to change shared account definitions
- `worker`: rebuilds local Xray config from shared snapshots and flushes traffic deltas

## Requirements

- shared mode requires MariaDB
- each worker needs a unique `nodeId`
- workers keep `/etc/x-ui/shared-cache.json` for outage survival

## Runtime Loops

- workers poll `shared_accounts_version` every `syncInterval`
- all nodes flush `/etc/x-ui/traffic-pending.json` every `trafficFlushInterval`
- only `master` runs shared traffic reconciliation that can disable or renew clients
```

- [ ] **Step 2: Add concise README entries in both languages**

Append a short section to `README.md`:

```md
## Multi-Node Shared Control

- use MariaDB as the shared control database
- keep one `master` node for shared-account writes
- configure other nodes as `worker`
- workers rebuild local Xray config from synchronized snapshots
- traffic is flushed back as deltas, not absolute totals
```

Append the matching section to `README.zh_CN.md`:

```md
## 多节点共享控制

- 使用 MariaDB 作为共享控制数据库
- 仅保留一个 `master` 节点负责共享账号写入
- 其他节点配置为 `worker`
- `worker` 通过同步快照重建本地 Xray 配置
- 流量按增量回刷，不覆盖绝对总量
```

- [ ] **Step 3: Add the manual verification checklist to the runbook**

Append this checklist to `docs/multi-node-sync.md`:

```md
## Manual Verification

1. Start a `master` node on MariaDB.
2. Start a `worker` node on the same MariaDB with a unique `nodeId`.
3. Change an inbound or client on `master`.
4. Confirm the worker sees a newer `shared_accounts_version` and rebuilds local Xray.
5. Generate traffic on both nodes.
6. Confirm aggregated MariaDB counters increase without overwriting each other.
7. Stop MariaDB briefly and confirm the worker continues using `shared-cache.json`.
8. Restore MariaDB and confirm pending traffic deltas flush successfully.
```

- [ ] **Step 4: Run focused verification**

Run:

```bash
go test ./config ./database ./web/service -v
go test ./... -run TestNonExistent -count=0
```

Expected:

- focused packages PASS
- package discovery succeeds across the repo

- [ ] **Step 5: Checkpoint Commit the docs**

Run:

```bash
git add docs/multi-node-sync.md README.md README.zh_CN.md
git commit -m "docs: add multi-node shared control guidance"
```

After commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`:

- mark Task 7 complete
- record the short commit hash beside Task 7

---

## Rollout Order

1. Task 1 — config, runtime file paths, CLI setters, startup validation
2. Task 2 — metadata tables and repository helpers
3. Task 3 — master-only guards and version bumping
4. Task 4 — cache-backed worker sync and snapshot-driven Xray rebuild
5. Task 5 — durable traffic delta collection, atomic flush, and master-only reconciliation
6. Task 6 — shell and installer flows
7. Task 7 — docs and focused verification

## Execution Strategy

- Tasks 1–3 execute Inline in the current session to establish the shared foundations before parallel work begins.
- Tasks 4–7 execute Subagent-Driven after Tasks 1–3 are complete and committed.
- Each task is a checkpoint and must end with its own git commit; do not batch adjacent tasks into one commit.
- After each checkpoint commit, update `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md` before moving to the next task.

## Acceptance Criteria

- `master` is the only node that can mutate shared account definitions.
- successful shared-account writes increment `shared_accounts_version` in the same transaction.
- `worker` nodes poll the shared version and rebuild local Xray config from cached snapshots.
- workers keep serving from `shared-cache.json` when MariaDB is temporarily unavailable.
- traffic is stored locally as deltas and flushed back without overwriting aggregate totals.
- shared-mode traffic collection on `worker` no longer calls `InboundService.AddTraffic()` and therefore no longer mutates shared enable / expiry state.
- only `master` performs shared traffic reconciliation that can disable or renew clients and inbounds.
- `x-ui.sh`, `install.sh`, and the README documents expose the node role and shared-control workflow clearly.

## Self-Review

- Spec coverage: the plan covers node config, shared metadata, master-only writes, worker snapshot sync, cache fallback, durable traffic deltas, master-only reconciliation after flush, operator tooling, and docs.
- Placeholder scan: no unfinished placeholder markers remain.
- Type consistency: `NodeConfig`, `SharedAccountsSnapshot`, `TrafficDelta`, `NodeSyncService`, `TrafficFlushService`, `RequireMaster`, and `ReconcileSharedTrafficState` are used consistently across tasks.

## Execution Handoff

Selected execution strategy:

- Inline foundation: Tasks 1–3
- Subagent-Driven expansion: Tasks 4–7

Progress tracker:

- `docs/superpowers/progress/2026-04-10-multi-node-shared-control-progress.md`
