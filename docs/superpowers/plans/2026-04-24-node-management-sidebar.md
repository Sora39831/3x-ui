# Node Management Sidebar — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "节点管理" sidebar item and page that shows connected nodes (master→workers, worker→master) with detailed status and node configuration editing.

**Architecture:** New NodeController serves API endpoints under `/panel/api/nodes/`. New `nodes.html` page follows existing patterns (Vue 2 + Ant Design Vue). Database layer adds `GetNodeStates()` query. Sidebar gets a new menu item gated by `{{if .is_admin}}`.

**Tech Stack:** Go, Gin, GORM, Vue.js 2, Ant Design Vue 1.x, Go html/template

---

### Task 1: Add GetNodeStates database query

**Files:**
- Modify: `database/shared_state.go`

- [ ] **Step 1: Add GetNodeStates function**

Add this function to `database/shared_state.go`, after the existing `UpsertNodeState` function:

```go
// GetNodeStates returns all node_state records ordered by node_id.
func GetNodeStates() ([]model.NodeState, error) {
	var states []model.NodeState
	err := GetDB().Order("node_id").Find(&states).Error
	return states, err
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./...`
Expected: PASS (no output)

- [ ] **Step 3: Commit**

```bash
git add database/shared_state.go
git commit -m "feat: add GetNodeStates query for node management"
```

---

### Task 2: Create NodeController with API endpoints

**Files:**
- Create: `web/controller/node.go`

- [ ] **Step 1: Create node.go with full controller**

Create `web/controller/node.go`:

```go
package controller

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"

	"github.com/gin-gonic/gin"
)

// NodeController handles node management API endpoints.
type NodeController struct {
	BaseController
}

// NewNodeController creates a new NodeController and initializes its routes.
func NewNodeController(g *gin.RouterGroup) *NodeController {
	a := &NodeController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for node management.
func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/nodes")
	g.Use(a.checkAdmin)

	g.GET("/list", a.list)
	g.GET("/config", a.getConfig)
	g.POST("/config", a.updateConfig)
}

// NodeView is the JSON shape returned to the frontend for each node.
type NodeView struct {
	NodeID          string `json:"nodeId"`
	NodeRole        string `json:"nodeRole"`
	Online          bool   `json:"online"`
	LastHeartbeatAt int64  `json:"lastHeartbeatAt"`
	LastSyncAt      int64  `json:"lastSyncAt"`
	LastSeenVersion int64  `json:"lastSeenVersion"`
	LastError       string `json:"lastError"`
}

// list returns connected nodes. Master sees all workers; worker sees the master.
func (a *NodeController) list(c *gin.Context) {
	nodeCfg := config.GetNodeConfigFromJSON()
	states, err := database.GetNodeStates()
	if err != nil {
		jsonMsg(c, "get node states", err)
		return
	}

	syncInterval := nodeCfg.SyncIntervalSeconds
	if syncInterval <= 0 {
		syncInterval = 30
	}
	offlineThreshold := int64(syncInterval * 2)
	now := time.Now().Unix()

	var nodes []NodeView
	for _, s := range states {
		// Master shows workers; worker shows master
		if nodeCfg.Role == config.NodeRoleMaster && s.NodeRole != string(config.NodeRoleWorker) {
			continue
		}
		if nodeCfg.Role == config.NodeRoleWorker && s.NodeRole != string(config.NodeRoleMaster) {
			continue
		}
		online := (now - s.LastHeartbeatAt) < offlineThreshold
		nodes = append(nodes, NodeView{
			NodeID:          s.NodeID,
			NodeRole:        s.NodeRole,
			Online:          online,
			LastHeartbeatAt: s.LastHeartbeatAt,
			LastSyncAt:      s.LastSyncAt,
			LastSeenVersion: s.LastSeenVersion,
			LastError:       s.LastError,
		})
	}
	if nodes == nil {
		nodes = []NodeView{}
	}

	jsonObj(c, nodes, nil)
}

// NodeConfigView is the JSON shape for node configuration.
type NodeConfigView struct {
	Role                string `json:"role"`
	NodeID              string `json:"nodeId"`
	SyncInterval        int    `json:"syncInterval"`
	TrafficFlushInterval int   `json:"trafficFlushInterval"`
	DBType              string `json:"dbType"`
	DBHost              string `json:"dbHost"`
	DBPort              string `json:"dbPort"`
	DBUser              string `json:"dbUser"`
	DBPass              string `json:"dbPass"`
	DBName              string `json:"dbName"`
}

// getConfig returns the current node configuration.
func (a *NodeController) getConfig(c *gin.Context) {
	nodeCfg := config.GetNodeConfigFromJSON()
	dbCfg := config.GetDBConfigFromJSON()

	jsonObj(c, NodeConfigView{
		Role:                 string(nodeCfg.Role),
		NodeID:               nodeCfg.NodeID,
		SyncInterval:         nodeCfg.SyncIntervalSeconds,
		TrafficFlushInterval: nodeCfg.TrafficFlushSeconds,
		DBType:               dbCfg.Type,
		DBHost:               dbCfg.Host,
		DBPort:               dbCfg.Port,
		DBUser:               dbCfg.User,
		DBPass:               dbCfg.Password,
		DBName:               dbCfg.Name,
	}, nil)
}

// updateConfigRequest is the JSON body for updating node config.
type updateConfigRequest struct {
	SyncInterval        int    `json:"syncInterval"`
	TrafficFlushInterval int   `json:"trafficFlushInterval"`
	DBType              string `json:"dbType"`
	DBHost              string `json:"dbHost"`
	DBPort              string `json:"dbPort"`
	DBUser              string `json:"dbUser"`
	DBPass              string `json:"dbPass"`
	DBName              string `json:"dbName"`
}

// updateConfig updates the node configuration in x-ui.json.
func (a *NodeController) updateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid request", err)
		return
	}

	// Validate
	if req.SyncInterval <= 0 {
		jsonMsg(c, "syncInterval must be positive", os.ErrInvalid)
		return
	}
	if req.TrafficFlushInterval <= 0 {
		jsonMsg(c, "trafficFlushInterval must be positive", os.ErrInvalid)
		return
	}

	// Write each setting to JSON config
	settings := map[string]string{
		"syncInterval":         json.NumberString(req.SyncInterval),
		"trafficFlushInterval": json.NumberString(req.TrafficFlushInterval),
		"dbType":               req.DBType,
		"dbHost":               req.DBHost,
		"dbPort":               req.DBPort,
		"dbUser":               req.DBUser,
		"dbPassword":           req.DBPass,
		"dbName":               req.DBName,
	}

	for key, value := range settings {
		if err := config.WriteSettingToJSON(key, value); err != nil {
			jsonMsg(c, "save "+key, err)
			return
		}
	}

	jsonMsg(c, I18nWeb(c, "pages.nodes.saveSuccess"), nil)
}
```

Wait — I need to check how `json.NumberString` works. Let me use `strconv.Itoa` instead.

- [ ] **Step 2: Fix the import — use strconv instead of json.NumberString**

The `updateConfig` function should use `strconv.Itoa` for integer-to-string conversion. Replace the settings map in the `updateConfig` function:

```go
	"strconv"
```

Add `"strconv"` to the import block, and change the settings map to:

```go
	settings := map[string]string{
		"syncInterval":         strconv.Itoa(req.SyncInterval),
		"trafficFlushInterval": strconv.Itoa(req.TrafficFlushInterval),
		"dbType":               req.DBType,
		"dbHost":               req.DBHost,
		"dbPort":               req.DBPort,
		"dbUser":               req.DBUser,
		"dbPassword":           req.DBPass,
		"dbName":               req.DBName,
	}
```

Remove the `"encoding/json"` and `"os"` imports (no longer needed).

- [ ] **Step 3: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/controller/node.go
git commit -m "feat: add NodeController with list/config API endpoints"
```

---

### Task 3: Register NodeController routes

**Files:**
- Modify: `web/controller/api.go:39-63`
- Modify: `web/controller/xui.go:27-39`

- [ ] **Step 1: Add NodeController to APIController**

In `web/controller/api.go`, add a field to the `APIController` struct (after `userController`):

```go
	nodeController *NodeController
```

In the `initRouter` method, add node routes after the users API group (after line 59):

```go
	// Nodes API
	nodes := api.Group("/nodes")
	nodes.Use(a.checkAdmin)
	a.nodeController = NewNodeController(nodes)
```

Wait — the NodeController already calls `a.checkAdmin` in its own `initRouter`. Looking at the pattern: `ServerController` and `UserController` don't call `checkAdmin` themselves — the `api.go` applies it at the group level. But my NodeController's `initRouter` applies `a.checkAdmin` on its own sub-group. This would double-apply the middleware.

Let me fix: the NodeController's `initRouter` should NOT call `a.checkAdmin` since the parent group in `api.go` already applies it. But actually, looking at the code more carefully:

- `api.go` line 51-52: `server := api.Group("/server")` then `server.Use(a.checkAdmin)` then `a.serverController = NewServerController(server)`
- The ServerController's `initRouter` receives the group and doesn't add `checkAdmin` again

So I need to remove the `g.Use(a.checkAdmin)` from NodeController's `initRouter`. Let me update the NodeController code:

In `web/controller/node.go`, remove the line `g.Use(a.checkAdmin)` from `initRouter`:

```go
func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/config", a.getConfig)
	g.POST("/config", a.updateConfig)
}
```

But wait — for the page route in `xui.go`, the `checkAdmin` is applied at the route level, not via middleware on the group. Let me check the XUI controller again...

Looking at `xui.go:27-39`:
```go
g = g.Group("/panel")
g.Use(a.checkLogin)
// ...
g.GET("/settings", a.checkAdmin, a.settings)
```

So for page routes, `checkAdmin` is per-route. For API routes, it's per-group. The NodeController's API routes will be under the `/panel/api/nodes` group which already has `checkAdmin` applied.

OK, the approach:
1. In `api.go`, add the node controller registration with `checkAdmin` on the group
2. In `xui.go`, add the page route with `checkAdmin` per-route
3. In `node.go`, remove the `checkAdmin` from `initRouter` since it's applied by the parent

- [ ] **Step 1 (corrected): Update api.go — add NodeController**

In `web/controller/api.go`:

1. Add field to struct (after `userController`):
```go
	nodeController *NodeController
```

2. Add route registration in `initRouter` (after the users block, before the "Extra routes" comment):
```go
	// Nodes API
	nodes := api.Group("/nodes")
	nodes.Use(a.checkAdmin)
	a.nodeController = NewNodeController(nodes)
```

- [ ] **Step 2: Update node.go — remove checkAdmin from initRouter**

In `web/controller/node.go`, change `initRouter` to remove `g.Use(a.checkAdmin)`:

```go
func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/config", a.getConfig)
	g.POST("/config", a.updateConfig)
}
```

- [ ] **Step 3: Update xui.go — add page route**

In `web/controller/xui.go`, add the nodes page route in `initRouter` (after the `xray` line, before the `users` line):

```go
	g.GET("/nodes", a.checkAdmin, a.nodes)
```

Add the handler method (after `xraySettings`):

```go
// nodes renders the node management page.
func (a *XUIController) nodes(c *gin.Context) {
	html(c, "nodes.html", "pages.nodes.title", nil)
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/controller/api.go web/controller/node.go web/controller/xui.go
git commit -m "feat: register NodeController routes and nodes page"
```

---

### Task 4: Add i18n translations

**Files:**
- Modify: `web/translation/translate.en_US.toml`
- Modify: `web/translation/translate.zh_CN.toml`

- [ ] **Step 1: Add English translations**

In `web/translation/translate.en_US.toml`, add `nodes` to the `[menu]` section (after `"xray"`):

```toml
"nodes" = "Nodes"
```

Add a new section at the end of the file:

```toml
[pages.nodes]
"title" = "Node Management"
"nodeId" = "Node ID"
"role" = "Role"
"status" = "Status"
"online" = "Online"
"offline" = "Offline"
"lastHeartbeat" = "Last Heartbeat"
"lastSync" = "Last Sync"
"syncVersion" = "Sync Version"
"error" = "Error"
"syncInterval" = "Sync Interval (seconds)"
"trafficFlushInterval" = "Traffic Flush Interval (seconds)"
"dbType" = "Database Type"
"dbHost" = "Database Host"
"dbPort" = "Database Port"
"dbUser" = "Database User"
"dbPass" = "Database Password"
"dbName" = "Database Name"
"save" = "Save"
"saveSuccess" = "Node configuration saved successfully"
"noWorkerNodes" = "No worker nodes connected"
"masterNode" = "Master Node"
"workerNodes" = "Worker Nodes"
"currentNodeConfig" = "Current Node Configuration"
"connectedNodes" = "Connected Nodes"
"refresh" = "Refresh"
```

Also add the page title key. In the `[pages.nodes]` section, make sure `"title"` is present (it's used by `html(c, "nodes.html", "pages.nodes.title", nil)`).

- [ ] **Step 2: Add Chinese translations**

In `web/translation/translate.zh_CN.toml`, add `nodes` to the `[menu]` section (after `"xray"`):

```toml
"nodes" = "节点管理"
```

Add a new section at the end of the file:

```toml
[pages.nodes]
"title" = "节点管理"
"nodeId" = "节点 ID"
"role" = "角色"
"status" = "状态"
"online" = "在线"
"offline" = "离线"
"lastHeartbeat" = "最后心跳"
"lastSync" = "最后同步"
"syncVersion" = "同步版本"
"error" = "错误"
"syncInterval" = "同步间隔（秒）"
"trafficFlushInterval" = "流量刷新间隔（秒）"
"dbType" = "数据库类型"
"dbHost" = "数据库主机"
"dbPort" = "数据库端口"
"dbUser" = "数据库用户"
"dbPass" = "数据库密码"
"dbName" = "数据库名称"
"save" = "保存"
"saveSuccess" = "节点配置保存成功"
"noWorkerNodes" = "暂无 Worker 节点连接"
"masterNode" = "主节点"
"workerNodes" = "Worker 节点"
"currentNodeConfig" = "当前节点配置"
"connectedNodes" = "已连接节点"
"refresh" = "刷新"
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/translation/translate.en_US.toml web/translation/translate.zh_CN.toml
git commit -m "feat: add i18n translations for node management"
```

---

### Task 5: Add sidebar menu item

**Files:**
- Modify: `web/html/component/aSidebar.html:61-66`

- [ ] **Step 1: Add nodes menu item**

In `web/html/component/aSidebar.html`, add the nodes entry in the `tabs` array between the `settings` item and the `xray` item. After the closing `},` of the settings item (line 61), add:

```javascript
                    {{if .is_admin}}
                    {
                        key: '{{ .base_path }}panel/nodes',
                        icon: 'cluster',
                        title: '{{ i18n "menu.nodes"}}'
                    },
                    {{end}}
```

The full tabs array should now be:
```javascript
tabs: [
    { key: '{{ .base_path }}panel/', icon: 'dashboard', title: '{{ i18n "menu.dashboard"}}' },
    { key: '{{ .base_path }}panel/inbounds', icon: 'user', title: '{{ i18n "menu.inbounds"}}' },
    { key: '{{ .base_path }}panel/settings', icon: 'setting', title: '{{ i18n "menu.settings"}}' },
    {{if .is_admin}}
    { key: '{{ .base_path }}panel/nodes', icon: 'cluster', title: '{{ i18n "menu.nodes"}}' },
    {{end}}
    { key: '{{ .base_path }}panel/xray', icon: 'tool', title: '{{ i18n "menu.xray"}}' },
    {{if .is_admin}}
    { key: '{{ .base_path }}panel/users', icon: 'team', title: '{{ i18n "menu.users"}}' },
    {{end}}
    { key: '{{ .base_path }}logout/', icon: 'logout', title: '{{ i18n "menu.logout"}}' },
],
```

- [ ] **Step 2: Commit**

```bash
git add web/html/component/aSidebar.html
git commit -m "feat: add nodes menu item to sidebar"
```

---

### Task 6: Create nodes.html page

**Files:**
- Create: `web/html/nodes.html`

- [ ] **Step 1: Create the full nodes.html page**

Create `web/html/nodes.html`:

```html
{{ template "page/head_start" .}}
{{ template "page/head_end" .}}

{{ template "page/body_start" .}}
<a-layout id="app" v-cloak :class="themeSwitcher.currentTheme + ' nodes-page'">
  <a-sidebar></a-sidebar>
  <a-layout id="content-layout">
    <a-layout-content>
      <a-spin :spinning="loading" :delay="200" tip='{{ i18n "loading"}}'>
        <transition name="list" appear>
          <a-row :gutter="[isMobile ? 8 : 16, isMobile ? 8 : 12]">
            <!-- Connected Nodes Section -->
            <a-col :span="24">
              <a-card hoverable>
                <template #title>
                  <a-row type="flex" justify="space-between" align="middle">
                    <a-col>
                      <a-space>
                        <a-icon type="cluster"></a-icon>
                        <span>{{ i18n "pages.nodes.connectedNodes" }}</span>
                        <a-tag :color="nodeRole === 'master' ? 'blue' : 'green'">[[ nodeRole ]]</a-tag>
                      </a-space>
                    </a-col>
                    <a-col>
                      <a-button icon="reload" size="small" @click="loadNodes">{{ i18n "pages.nodes.refresh" }}</a-button>
                    </a-col>
                  </a-row>
                </template>
                <a-table
                  v-if="nodeRole === 'master'"
                  :columns="nodeColumns"
                  :data-source="nodes"
                  :row-key="record => record.nodeId"
                  :pagination="false"
                  :scroll="isMobile ? { x: 700 } : undefined"
                  size="middle">
                  <template slot="status" slot-scope="text, record">
                    <a-badge :status="record.online ? 'success' : 'error'" :text="record.online ? '{{ i18n "pages.nodes.online" }}' : '{{ i18n "pages.nodes.offline" }}'" />
                  </template>
                  <template slot="role" slot-scope="text, record">
                    <a-tag :color="record.nodeRole === 'master' ? 'blue' : 'green'">[[ record.nodeRole ]]</a-tag>
                  </template>
                  <template slot="lastHeartbeat" slot-scope="text, record">
                    [[ record.lastHeartbeatAt ? formatTime(record.lastHeartbeatAt) : '-' ]]
                  </template>
                  <template slot="lastSync" slot-scope="text, record">
                    [[ record.lastSyncAt ? formatTime(record.lastSyncAt) : '-' ]]
                  </template>
                </a-table>
                <div v-if="nodeRole === 'worker'">
                  <a-empty v-if="nodes.length === 0" description='{{ i18n "pages.nodes.noWorkerNodes" }}' />
                  <a-descriptions v-else bordered size="small" :column="isMobile ? 1 : 2">
                    <a-descriptions-item label='{{ i18n "pages.nodes.nodeId" }}'>[[ nodes[0].nodeId ]]</a-descriptions-item>
                    <a-descriptions-item label='{{ i18n "pages.nodes.status" }}'>
                      <a-badge :status="nodes[0].online ? 'success' : 'error'" :text="nodes[0].online ? '{{ i18n "pages.nodes.online" }}' : '{{ i18n "pages.nodes.offline" }}'" />
                    </a-descriptions-item>
                    <a-descriptions-item label='{{ i18n "pages.nodes.lastHeartbeat" }}'>[[ nodes[0].lastHeartbeatAt ? formatTime(nodes[0].lastHeartbeatAt) : '-' ]]</a-descriptions-item>
                    <a-descriptions-item label='{{ i18n "pages.nodes.lastSync" }}'>[[ nodes[0].lastSyncAt ? formatTime(nodes[0].lastSyncAt) : '-' ]]</a-descriptions-item>
                    <a-descriptions-item label='{{ i18n "pages.nodes.syncVersion" }}'>[[ nodes[0].lastSeenVersion ]]</a-descriptions-item>
                    <a-descriptions-item label='{{ i18n "pages.nodes.error" }}'>[[ nodes[0].lastError || '-' ]]</a-descriptions-item>
                  </a-descriptions>
                </div>
                <a-empty v-if="nodeRole === 'master' && nodes.length === 0" description='{{ i18n "pages.nodes.noWorkerNodes" }}' />
              </a-card>
            </a-col>

            <!-- Current Node Config Section -->
            <a-col :span="24">
              <a-card hoverable>
                <template #title>
                  <a-space>
                    <a-icon type="setting"></a-icon>
                    <span>{{ i18n "pages.nodes.currentNodeConfig" }}</span>
                  </a-space>
                </template>
                <a-form layout="vertical">
                  <a-row :gutter="16">
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.role" }}'>
                        <a-input :value="nodeConfig.role" disabled></a-input>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.nodeId" }}'>
                        <a-input :value="nodeConfig.nodeId" disabled></a-input>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.dbType" }}'>
                        <a-select v-model="nodeConfig.dbType" :disabled="saving">
                          <a-select-option value="sqlite">SQLite</a-select-option>
                          <a-select-option value="mysql">MySQL/MariaDB</a-select-option>
                        </a-select>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.syncInterval" }}'>
                        <a-input-number v-model="nodeConfig.syncInterval" :min="5" :max="3600" style="width: 100%"></a-input-number>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.trafficFlushInterval" }}'>
                        <a-input-number v-model="nodeConfig.trafficFlushInterval" :min="5" :max="3600" style="width: 100%"></a-input-number>
                      </a-form-item>
                    </a-col>
                  </a-row>
                  <a-divider>{{ i18n "pages.nodes.dbHost" }}</a-divider>
                  <a-row :gutter="16">
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.dbHost" }}'>
                        <a-input v-model="nodeConfig.dbHost" :disabled="saving"></a-input>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.dbPort" }}'>
                        <a-input v-model="nodeConfig.dbPort" :disabled="saving"></a-input>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.dbName" }}'>
                        <a-input v-model="nodeConfig.dbName" :disabled="saving"></a-input>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.dbUser" }}'>
                        <a-input v-model="nodeConfig.dbUser" :disabled="saving"></a-input>
                      </a-form-item>
                    </a-col>
                    <a-col :xs="24" :sm="12" :md="8">
                      <a-form-item label='{{ i18n "pages.nodes.dbPass" }}'>
                        <a-input-password v-model="nodeConfig.dbPass" :disabled="saving"></a-input-password>
                      </a-form-item>
                    </a-col>
                  </a-row>
                  <a-form-item>
                    <a-button type="primary" icon="save" :loading="saving" @click="saveConfig">
                      {{ i18n "pages.nodes.save" }}
                    </a-button>
                  </a-form-item>
                </a-form>
              </a-card>
            </a-col>
          </a-row>
        </transition>
      </a-spin>
    </a-layout-content>
  </a-layout>
</a-layout>
{{template "page/body_scripts" .}}
<script>
  const app = new Vue({
    el: '#app',
    delimiters: ['[[', ']]'],
    data() {
      return {
        loading: false,
        saving: false,
        nodeRole: '{{ if .is_admin }}master{{ else }}worker{{ end }}',
        nodes: [],
        nodeConfig: {
          role: '',
          nodeId: '',
          syncInterval: 30,
          trafficFlushInterval: 10,
          dbType: '',
          dbHost: '',
          dbPort: '',
          dbUser: '',
          dbPass: '',
          dbName: '',
        },
        nodeColumns: [
          { title: '{{ i18n "pages.nodes.nodeId" }}', dataIndex: 'nodeId', width: 150 },
          { title: '{{ i18n "pages.nodes.status" }}', scopedSlots: { customRender: 'status' }, width: 100 },
          { title: '{{ i18n "pages.nodes.role" }}', scopedSlots: { customRender: 'role' }, width: 80 },
          { title: '{{ i18n "pages.nodes.lastHeartbeat" }}', scopedSlots: { customRender: 'lastHeartbeat' }, width: 180 },
          { title: '{{ i18n "pages.nodes.lastSync" }}', scopedSlots: { customRender: 'lastSync' }, width: 180 },
          { title: '{{ i18n "pages.nodes.syncVersion" }}', dataIndex: 'lastSeenVersion', width: 120 },
          { title: '{{ i18n "pages.nodes.error" }}', dataIndex: 'lastError', ellipsis: true },
        ],
        refreshTimer: null,
      }
    },
    computed: {
      isMobile() {
        return window.innerWidth <= 768;
      }
    },
    methods: {
      async loadNodes() {
        try {
          const res = await axios.get('api/nodes/list');
          if (res.data.success) {
            this.nodes = res.data.obj;
            // Determine current role from the first node or config
            if (this.nodes.length > 0) {
              // If we're master, all returned nodes are workers
              // If we're worker, returned nodes are master
              // We can also check from config
            }
          }
        } catch (e) {
          console.error('Failed to load nodes', e);
        }
      },
      async loadConfig() {
        try {
          const res = await axios.get('api/nodes/config');
          if (res.data.success) {
            Object.assign(this.nodeConfig, res.data.obj);
            this.nodeRole = res.data.obj.role;
          }
        } catch (e) {
          console.error('Failed to load node config', e);
        }
      },
      async saveConfig() {
        this.saving = true;
        try {
          const res = await axios.post('api/nodes/config', {
            syncInterval: this.nodeConfig.syncInterval,
            trafficFlushInterval: this.nodeConfig.trafficFlushInterval,
            dbType: this.nodeConfig.dbType,
            dbHost: this.nodeConfig.dbHost,
            dbPort: this.nodeConfig.dbPort,
            dbUser: this.nodeConfig.dbUser,
            dbPass: this.nodeConfig.dbPass,
            dbName: this.nodeConfig.dbName,
          });
          if (res.data.success) {
            this.$message.success(res.data.msg);
          } else {
            this.$message.error(res.data.msg);
          }
        } catch (e) {
          this.$message.error('Save failed');
        } finally {
          this.saving = false;
        }
      },
      formatTime(ts) {
        if (!ts) return '-';
        return moment.unix(ts).format('YYYY-MM-DD HH:mm:ss');
      },
    },
    mounted() {
      this.loadNodes();
      this.loadConfig();
      // Auto-refresh node list every 10 seconds
      this.refreshTimer = setInterval(() => {
        this.loadNodes();
      }, 10000);
    },
    beforeDestroy() {
      if (this.refreshTimer) {
        clearInterval(this.refreshTimer);
      }
    },
  });
</script>
{{ template "page/body_end" }}
</html>
```

- [ ] **Step 2: Commit**

```bash
git add web/html/nodes.html
git commit -m "feat: add nodes.html page with node list and config form"
```

---

### Task 7: Build and verify

- [ ] **Step 1: Full build check**

Run: `cd /usr/x-ui/3x-ui && go build ./...`
Expected: PASS

- [ ] **Step 2: Run vet**

Run: `cd /usr/x-ui/3x-ui && go vet ./...`
Expected: PASS

- [ ] **Step 3: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: address build issues from node management feature"
```
