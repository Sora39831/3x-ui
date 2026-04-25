# Trojan-Go Style MariaDB Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a minimal multi-VPS `master` / `worker` sync model on top of the existing MariaDB support, with install-time role selection, runtime role switching, MariaDB as the default for fresh installs, and SQLite compatibility preserved.

**Architecture:** Keep MariaDB as the shared source of truth and keep local Xray config generation unchanged. Add node-role and sync settings into the JSON config, expose them through CLI and shell scripts, enforce write restrictions in backend services, and add a polling plus traffic-delta sync loop that workers use against MariaDB-backed shared state.

**Tech Stack:** Go, GORM, Gin, shell scripts (`install.sh`, `x-ui.sh`), MariaDB, SQLite, Go tests, `bash -n`

---

## File Map

**Modify**

- `config/config.go`
- `config/config_test.go`
- `web/service/setting.go`
- `web/service/setting_test.go`
- `web/entity/entity.go`
- `main.go`
- `x-ui.sh`
- `install.sh`
- `database/model/model.go`
- `database/db.go`
- `web/service/inbound.go`
- `web/service/server.go`
- `web/service/xray.go`

**Create**

- `web/service/node_sync.go`
- `web/service/node_sync_test.go`

**Reference**

- `docs/superpowers/specs/2026-04-09-trojan-go-style-mariadb-sync-design.md`

### Task 1: Add node-role settings and MariaDB-first defaults

**Files:**

- Modify: `web/service/setting.go`
- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Modify: `web/service/setting_test.go`

- [ ] **Step 1: Write the failing config tests**

```go
func TestGetNodeConfigFromJSONSupportsModulePurposeLayout(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	settings := map[string]any{
		"_meta": map[string]any{"layout": "按模块-用途来归类"},
		"databaseConnection": map[string]any{
			"dbType": "mariadb",
		},
		"systemIntegration": map[string]any{
			"nodeRole":             "worker",
			"nodeId":               "vps-01",
			"syncInterval":         "30",
			"trafficFlushInterval": "60",
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(GetSettingPath(), data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	cfg := GetNodeConfigFromJSON()
	if cfg.Role != "worker" || cfg.ID != "vps-01" || cfg.SyncInterval != "30" || cfg.TrafficFlushInterval != "60" {
		t.Fatalf("unexpected node config: %+v", cfg)
	}
}

func TestLoadSettingsUsesMariaDBAsFreshInstallDefault(t *testing.T) {
	setupTestSettings(t)

	settings, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings() error: %v", err)
	}
	if settings["dbType"] != "mariadb" {
		t.Fatalf("expected default dbType=mariadb, got %s", settings["dbType"])
	}
	if settings["nodeRole"] != "master" {
		t.Fatalf("expected default nodeRole=master, got %s", settings["nodeRole"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./config ./web/service -run 'Test(GetNodeConfigFromJSONSupportsModulePurposeLayout|LoadSettingsUsesMariaDBAsFreshInstallDefault)' -v`

Expected: FAIL because `GetNodeConfigFromJSON`, `nodeRole`, and the new defaults do not exist yet.

- [ ] **Step 3: Add defaults and JSON readers**

```go
type NodeConfig struct {
	Role                 string
	ID                   string
	SyncInterval         string
	TrafficFlushInterval string
}

func GetNodeConfigFromJSON() NodeConfig {
	data, err := os.ReadFile(GetSettingPath())
	if err != nil {
		return NodeConfig{Role: "master", ID: "", SyncInterval: "30", TrafficFlushInterval: "60"}
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return NodeConfig{Role: "master", ID: "", SyncInterval: "30", TrafficFlushInterval: "60"}
	}

	return NodeConfig{
		Role:                 readGroupedString(settings, "nodeRole"),
		ID:                   readGroupedString(settings, "nodeId"),
		SyncInterval:         readGroupedString(settings, "syncInterval"),
		TrafficFlushInterval: readGroupedString(settings, "trafficFlushInterval"),
	}
}
```

```go
// web/service/setting.go
"dbType":               "mariadb",
"nodeRole":             "master",
"nodeId":               "",
"syncInterval":         "30",
"trafficFlushInterval": "60",
```

- [ ] **Step 4: Add setting groups and getters/setters**

```go
"systemIntegration": {
	"nodeRole":             "nodeRole",
	"nodeId":               "nodeId",
	"syncInterval":         "syncInterval",
	"trafficFlushInterval": "trafficFlushInterval",
},
```

```go
func (s *SettingService) GetNodeRole() (string, error) { return s.getString("nodeRole") }
func (s *SettingService) SetNodeRole(value string) error { return s.setString("nodeRole", value) }
func (s *SettingService) GetNodeID() (string, error) { return s.getString("nodeId") }
func (s *SettingService) SetNodeID(value string) error { return s.setString("nodeId", value) }
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./config ./web/service -run 'Test(GetNodeConfigFromJSONSupportsModulePurposeLayout|LoadSettingsUsesMariaDBAsFreshInstallDefault)' -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add config/config.go config/config_test.go web/service/setting.go web/service/setting_test.go
git commit -m "feat: add node sync settings and mariadb defaults"
```

### Task 2: Expose node-role configuration through the Go CLI

**Files:**

- Modify: `main.go`
- Modify: `web/entity/entity.go`
- Modify: `web/service/setting_test.go`

- [ ] **Step 1: Write the failing validation and CLI tests**

```go
func TestSettingEntityAcceptsNodeRoleValues(t *testing.T) {
	s := AllSetting{DBType: "mariadb", NodeRole: "worker"}
	if err := s.Check(); err != nil {
		t.Fatalf("expected worker nodeRole to be accepted: %v", err)
	}
}

func TestSettingServiceSetAndGetNodeRole(t *testing.T) {
	setupTestSettings(t)
	svc := &SettingService{}
	if err := svc.SetNodeRole("worker"); err != nil {
		t.Fatalf("SetNodeRole error: %v", err)
	}
	role, err := svc.GetNodeRole()
	if err != nil {
		t.Fatalf("GetNodeRole error: %v", err)
	}
	if role != "worker" {
		t.Fatalf("expected worker, got %s", role)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/service ./web/entity -run 'Test(SettingEntityAcceptsNodeRoleValues|SettingServiceSetAndGetNodeRole)' -v`

Expected: FAIL because `NodeRole` is not part of the validated setting payload and service API yet.

- [ ] **Step 3: Extend the CLI flags and JSON write path**

```go
var nodeRole string
var nodeID string
var syncInterval string
var trafficFlushInterval string

settingCmd.StringVar(&nodeRole, "nodeRole", "", "Set node role (master or worker)")
settingCmd.StringVar(&nodeID, "nodeId", "", "Set node identifier")
settingCmd.StringVar(&syncInterval, "syncInterval", "", "Set account sync interval in seconds")
settingCmd.StringVar(&trafficFlushInterval, "trafficFlushInterval", "", "Set traffic flush interval in seconds")
```

```go
if nodeRole != "" {
	if err := config.WriteSettingToJSON("nodeRole", nodeRole); err != nil {
		fmt.Println("Failed to set nodeRole:", err)
	} else {
		fmt.Println("nodeRole set to:", nodeRole)
	}
}
```

- [ ] **Step 4: Validate `nodeRole` in the setting entity**

```go
if s.NodeRole != "" && s.NodeRole != "master" && s.NodeRole != "worker" {
	return common.NewError("node role must be master or worker, got:", s.NodeRole)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./web/service ./web/entity -run 'Test(SettingEntityAcceptsNodeRoleValues|SettingServiceSetAndGetNodeRole)' -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add main.go web/entity/entity.go web/service/setting_test.go
git commit -m "feat: expose node role settings in cli"
```

### Task 3: Add node management to `x-ui.sh`

**Files:**

- Modify: `x-ui.sh`

- [ ] **Step 1: Write a shell syntax safety checkpoint**

Run: `bash -n x-ui.sh`

Expected: PASS before changes, establishing a clean syntax baseline.

- [ ] **Step 2: Add helpers for reading and writing node settings**

```bash
read_json_noderole() {
    local node_role
    node_role=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep '^nodeRole:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    if [ -z "$node_role" ]; then
        echo "master"
    else
        echo "$node_role"
    fi
}

switch_node_role() {
    local role="$1"
    ${xui_folder}/x-ui setting -nodeRole "$role" >/dev/null 2>&1
}
```

- [ ] **Step 3: Add a node management menu**

```bash
node_menu() {
    local current_role=$(read_json_noderole)

    echo -e "
╔────────────────────────────────────────────────╗
│   ${green}节点管理${plain}                                      │
│────────────────────────────────────────────────│
│   ${green}0.${plain} 返回主菜单                                │
│   ${green}1.${plain} 查看当前节点角色（当前: ${current_role}）    │
│   ${green}2.${plain} 切换到 master                             │
│   ${green}3.${plain} 切换到 worker                             │
│   ${green}4.${plain} 设置 nodeId                               │
╚════════════════════════════════════════════════╝
"
}
```

- [ ] **Step 4: Wire the menu into `show_menu` and preserve existing database menu**

```bash
│  ${green}27.${plain} 数据库管理                                │
│  ${green}28.${plain} 节点管理                                  │
```

```bash
27)
    check_install && db_menu
    ;;
28)
    check_install && node_menu
    ;;
```

- [ ] **Step 5: Re-run shell syntax verification**

Run: `bash -n x-ui.sh`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add x-ui.sh
git commit -m "feat: add node role management to x-ui shell"
```

### Task 4: Add install-time role selection and MariaDB-first bootstrap

**Files:**

- Modify: `install.sh`

- [ ] **Step 1: Write a shell syntax safety checkpoint**

Run: `bash -n install.sh`

Expected: PASS before changes.

- [ ] **Step 2: Add fresh-install prompts for database type and node role**

```bash
read -rp "请选择数据库类型 [默认 mariadb，可选 sqlite/mariadb]：" install_db_type
install_db_type="${install_db_type// /}"
install_db_type="${install_db_type,,}"
if [[ -z "$install_db_type" ]]; then
    install_db_type="mariadb"
fi

read -rp "请选择节点角色 [默认 master，可选 master/worker]：" install_node_role
install_node_role="${install_node_role// /}"
install_node_role="${install_node_role,,}"
if [[ -z "$install_node_role" ]]; then
    install_node_role="master"
fi
```

- [ ] **Step 3: Persist fresh-install settings through the existing CLI**

```bash
${xui_folder}/x-ui setting -dbType "${install_db_type}" -nodeRole "${install_node_role}"
if [[ -n "${install_node_id}" ]]; then
    ${xui_folder}/x-ui setting -nodeId "${install_node_id}"
fi
```

```bash
if [[ "${install_db_type}" == "mariadb" ]]; then
    XUI_DB_PASSWORD="${db_pass}" ${xui_folder}/x-ui setting \
        -dbHost "${db_host}" \
        -dbPort "${db_port}" \
        -dbUser "${db_user}" \
        -dbName "${db_name}"
fi
```

- [ ] **Step 4: Preserve SQLite compatibility for existing installs**

```bash
if [[ "$is_fresh_install" != "true" ]]; then
    echo -e "${green}检测到现有安装，保留当前数据库类型与节点角色。${plain}"
else
    # prompt for install_db_type and install_node_role only on fresh install
fi
```

- [ ] **Step 5: Re-run shell syntax verification**

Run: `bash -n install.sh`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add install.sh
git commit -m "feat: prompt for node role and mariadb on install"
```

### Task 5: Enforce `master` / `worker` write boundaries in Go services

**Files:**

- Create: `web/service/node_sync.go`
- Create: `web/service/node_sync_test.go`
- Modify: `web/service/inbound.go`
- Modify: `web/service/server.go`
- Modify: `web/service/user_test.go`

- [ ] **Step 1: Write the failing role-enforcement tests**

```go
func TestRequireMasterAllowsMaster(t *testing.T) {
	setupTestSettings(t)
	if err := config.WriteSettingToJSON("nodeRole", "master"); err != nil {
		t.Fatalf("WriteSettingToJSON error: %v", err)
	}
	if err := RequireMaster(); err != nil {
		t.Fatalf("expected master to pass: %v", err)
	}
}

func TestRequireMasterRejectsWorker(t *testing.T) {
	setupTestSettings(t)
	if err := config.WriteSettingToJSON("nodeRole", "worker"); err != nil {
		t.Fatalf("WriteSettingToJSON error: %v", err)
	}
	if err := RequireMaster(); err == nil {
		t.Fatal("expected worker role to be rejected")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/service -run 'TestRequireMaster(AllowsMaster|RejectsWorker)' -v`

Expected: FAIL because no role gate exists yet.

- [ ] **Step 3: Implement a shared guard helper**

```go
func CurrentNodeRole() string {
	cfg := config.GetNodeConfigFromJSON()
	if cfg.Role == "" {
		return "master"
	}
	return cfg.Role
}

func RequireMaster() error {
	if CurrentNodeRole() != "master" {
		return common.NewError("write operations are only allowed on master nodes")
	}
	return nil
}
```

- [ ] **Step 4: Call the guard in shared-state write paths**

```go
func (s *InboundService) AddInbound(inbound *model.Inbound) error {
	if err := RequireMaster(); err != nil {
		return err
	}
	// existing logic
}
```

```go
func (s *InboundService) DelInbound(id int) error {
	if err := RequireMaster(); err != nil {
		return err
	}
	// existing logic
}
```

- [ ] **Step 5: Re-run the tests and a focused service package pass**

Run: `go test ./web/service -run 'TestRequireMaster(AllowsMaster|RejectsWorker)' -v`

Expected: PASS

Run: `go test ./web/service/...`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/service/node_sync.go web/service/node_sync_test.go web/service/inbound.go web/service/server.go web/service/user_test.go
git commit -m "feat: enforce master-only shared writes"
```

### Task 6: Add account polling and local cache refresh

**Files:**

- Create: `web/service/node_sync.go`
- Create: `web/service/node_sync_test.go`
- Modify: `web/service/xray.go`
- Modify: `main.go`

- [ ] **Step 1: Write the failing sync-interval and version tests**

```go
func TestShouldRefreshAccountsWhenVersionChanges(t *testing.T) {
	state := syncState{lastVersion: 2}
	if !state.shouldRefresh(3) {
		t.Fatal("expected newer version to trigger refresh")
	}
}

func TestShouldNotRefreshAccountsWhenVersionMatches(t *testing.T) {
	state := syncState{lastVersion: 3}
	if state.shouldRefresh(3) {
		t.Fatal("expected same version to skip refresh")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/service -run 'TestShould(RefreshAccountsWhenVersionChanges|NotRefreshAccountsWhenVersionMatches)' -v`

Expected: FAIL because the sync state object and refresh logic do not exist.

- [ ] **Step 3: Implement the sync state and poller skeleton**

```go
type syncState struct {
	lastVersion int64
}

func (s syncState) shouldRefresh(version int64) bool {
	return version > s.lastVersion
}

func StartNodeSyncLoop() {
	cfg := config.GetNodeConfigFromJSON()
	if config.GetDBTypeFromJSON() != "mariadb" {
		return
	}
	if cfg.Role != "worker" && cfg.Role != "master" {
		return
	}
	go runAccountSyncLoop(cfg)
}
```

- [ ] **Step 4: Hook the poller into process startup without changing local Xray ownership**

```go
func runWebServer() {
	if err := database.InitDB(); err != nil {
		log.Fatal(err)
	}
	service.StartNodeSyncLoop()
	// existing startup
}
```

- [ ] **Step 5: Re-run focused tests**

Run: `go test ./web/service -run 'TestShould(RefreshAccountsWhenVersionChanges|NotRefreshAccountsWhenVersionMatches)' -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/service/node_sync.go web/service/node_sync_test.go web/service/xray.go main.go
git commit -m "feat: add account sync polling skeleton"
```

### Task 7: Add traffic delta writeback and worker-safe accounting

**Files:**

- Modify: `database/model/model.go`
- Modify: `database/db.go`
- Modify: `web/service/node_sync.go`
- Modify: `web/service/node_sync_test.go`

- [ ] **Step 1: Write the failing delta accounting tests**

```go
func TestApplyTrafficDeltaAccumulatesUsage(t *testing.T) {
	current := trafficTotals{Upload: 100, Download: 200}
	next := current.applyDelta(20, 30)
	if next.Upload != 120 || next.Download != 230 {
		t.Fatalf("unexpected totals: %+v", next)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/service -run TestApplyTrafficDeltaAccumulatesUsage -v`

Expected: FAIL because the delta helper does not exist.

- [ ] **Step 3: Implement atomic delta write helpers**

```go
type trafficTotals struct {
	Upload   int64
	Download int64
}

func (t trafficTotals) applyDelta(uploadDelta, downloadDelta int64) trafficTotals {
	t.Upload += uploadDelta
	t.Download += downloadDelta
	return t
}
```

```go
func ApplyTrafficDelta(clientID int64, uploadDelta, downloadDelta int64) error {
	return GetDB().Model(&model.ClientTraffic{}).
		Where("client_id = ?", clientID).
		Updates(map[string]any{
			"up":   gorm.Expr("up + ?", uploadDelta),
			"down": gorm.Expr("down + ?", downloadDelta),
		}).Error
}
```

- [ ] **Step 4: Flush only deltas from worker nodes**

```go
func flushTrafficDeltas(deltas map[int64]trafficTotals) error {
	for clientID, delta := range deltas {
		if err := database.ApplyTrafficDelta(clientID, delta.Upload, delta.Download); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: Re-run focused tests and database package tests**

Run: `go test ./web/service -run TestApplyTrafficDeltaAccumulatesUsage -v`

Expected: PASS

Run: `go test ./database/...`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add database/model/model.go database/db.go web/service/node_sync.go web/service/node_sync_test.go
git commit -m "feat: add worker traffic delta writeback"
```

### Task 8: Final verification and docs alignment

**Files:**

- Modify: `docs/superpowers/specs/2026-04-09-trojan-go-style-mariadb-sync-design.md`
- Modify: `docs/superpowers/plans/2026-04-09-trojan-go-style-mariadb-sync.md`

- [ ] **Step 1: Run the targeted Go test suites**

Run: `go test ./config ./database/... ./web/service/...`

Expected: PASS

- [ ] **Step 2: Run shell syntax checks**

Run: `bash -n x-ui.sh`

Expected: PASS

Run: `bash -n install.sh`

Expected: PASS

- [ ] **Step 3: Manually verify fresh-install and runtime flows**

Run:

```bash
./x-ui setting -showDbType
./x-ui setting -nodeRole worker -nodeId vps-02
./x-ui setting -dbType sqlite
```

Expected:

- first command prints the configured backend
- second command updates the JSON config without touching the DB schema
- third command remains accepted for compatibility

- [ ] **Step 4: Update docs if the final implementation diverges from the spec**

```md
- document the final JSON keys
- document the exact `x-ui.sh` menu entries
- document the fresh-install MariaDB default and SQLite compatibility rule
```

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/specs/2026-04-09-trojan-go-style-mariadb-sync-design.md docs/superpowers/plans/2026-04-09-trojan-go-style-mariadb-sync.md
git commit -m "docs: finalize node sync implementation notes"
```

