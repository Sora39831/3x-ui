# Node Management Sidebar — Design Spec

**Date:** 2026-04-24
**Status:** Approved

## Overview

Add a "Node Management" page accessible from the sidebar, visible only to admin users. The page displays connected node status and allows modifying node configuration.

## Behavior by Role

- **Master node:** Shows a table of all connected worker nodes with detailed status
- **Worker node:** Shows a card with the master node's info

## Backend

### New Controller: `NodeController`

File: `web/controller/node.go`

API endpoints (all admin-only via `checkAdmin` middleware):

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/panel/api/nodes/list` | GET | Node list (master: all workers; worker: master) |
| `/panel/api/nodes/config` | GET | Current node config |
| `/panel/api/nodes/config` | POST | Update current node config |

### New Page Route

In `xui.go`, add:
```go
func (x *XUIController) Nodes(c *gin.Context) {
    // render nodes.html
}
```

Route: `GET /panel/nodes` → `XUIController.Nodes` (admin only)

### Data Sources

- **Node list:** Query `node_states` table via `database.GetNodeStates()` (new function in `database/shared_state.go`)
- **Node config:** Read from `x-ui.json` via existing `config.GetNodeConfigFromJSON()`
- **DB config:** Read from `AllSetting` entity (dbType, dbHost, dbPort, dbUser, dbPass, dbName)
- **Online status:** `LastHeartbeatAt` > 2 × `syncInterval` ago → offline

### Config Update Logic

POST `/panel/api/nodes/config` accepts JSON body with:
- `syncInterval` (int, seconds)
- `trafficFlushInterval` (int, seconds)
- `dbType`, `dbHost`, `dbPort`, `dbUser`, `dbPass`, `dbName`

Writes to `x-ui.json` under `"other"` group. Does NOT allow changing `nodeRole` or `nodeId` at runtime (displayed as read-only).

## Frontend

### New Page: `web/html/xui/nodes.html`

Structure (mirrors settings.html pattern):
- Head section: imports, template includes
- Vue app with two sections:
  1. **Node list** — `<a-table>` (master) or `<a-card>` (worker)
  2. **Node config form** — `<a-form>` with save button

### Node List Columns (master view)

| Column | Source |
|--------|--------|
| Node ID | `NodeState.NodeID` |
| Status | Online/Offline (heartbeat check) |
| Last Heartbeat | `NodeState.LastHeartbeatAt` (formatted) |
| Last Sync | `NodeState.LastSyncAt` (formatted) |
| Sync Version | `NodeState.LastSeenVersion` |
| Error | `NodeState.LastError` |

Worker view: same fields in a card layout.

### Config Form Fields

| Field | Type | Editable |
|-------|------|----------|
| Node Role | Text | No (read-only) |
| Node ID | Text | No (read-only) |
| Sync Interval | Number (seconds) | Yes |
| Traffic Flush Interval | Number (seconds) | Yes |
| DB Type | Select (sqlite/mysql) | Yes |
| DB Host | Text | Yes |
| DB Port | Number | Yes |
| DB User | Text | Yes |
| DB Password | Password | Yes |
| DB Name | Text | Yes |

### Auto-Refresh

Node list polls `/panel/api/nodes/list` every 10 seconds via `setInterval`.

### Sidebar Change

In `web/html/component/aSidebar.html`, add between `settings` and `xray`:
```javascript
{{if .is_admin}}
{ key: '{{ .base_path }}panel/nodes', icon: 'cluster', title: '{{ i18n "menu.nodes"}}' },
{{end}}
```

## i18n

Add to `translate.en_US.toml` and `translate.zh_CN.toml`:
```toml
[menu]
"nodes" = "Nodes"  # en
"nodes" = "节点管理"  # zh

[nodes]
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
"syncInterval" = "Sync Interval"
"trafficFlushInterval" = "Traffic Flush Interval"
"dbType" = "Database Type"
"dbHost" = "Database Host"
"dbPort" = "Database Port"
"dbUser" = "Database User"
"dbPass" = "Database Password"
"dbName" = "Database Name"
"save" = "Save"
"saveSuccess" = "Saved successfully"
"noWorkerNodes" = "No worker nodes connected"
"masterNode" = "Master Node"
"workerNodes" = "Worker Nodes"
```

## Files to Create/Modify

| File | Action |
|------|--------|
| `web/controller/node.go` | **Create** — NodeController with list/config APIs |
| `web/html/xui/nodes.html` | **Create** — Node management page |
| `web/html/component/aSidebar.html` | **Modify** — Add nodes menu item |
| `web/web.go` | **Modify** — Register routes and controller |
| `web/controller/xui.go` | **Modify** — Add Nodes() page method |
| `web/translation/translate.en_US.toml` | **Modify** — Add i18n keys |
| `web/translation/translate.zh_CN.toml` | **Modify** — Add i18n keys |
| `database/shared_state.go` | **Modify** — Add GetNodeStates() query function |

## Scope Boundaries

- **In scope:** View node status, modify node config, sidebar entry
- **Out of scope:** Node registration/removal, restart, adding new nodes, real-time WebSocket updates (uses polling instead)
