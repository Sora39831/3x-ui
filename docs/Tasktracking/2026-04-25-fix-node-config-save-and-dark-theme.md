# 2026-04-25 Fix node config save, dbType mismatch, and dark theme

## Problem

### 1. Node config save always fails
The `saveConfig` endpoint in `node.go` used `c.ShouldBindJSON()` which expects
`Content-Type: application/json`. But the global axios interceptor in `axios-init.js`
converts all POST data via `Qs.stringify()` and sends it as
`application/x-www-form-urlencoded`. The backend rejected every save with:
`invalid request (invalid character 's' looking for beginning of value)`.

### 2. dbType dropdown value mismatch
The frontend `<a-select>` used `value="mysql"` for the MySQL/MariaDB option, but the
backend checks for `"mariadb"` everywhere (database init, node list query, validation).
Saving through the UI would write `"mysql"`, which the backend would treat as SQLite.

### 3. Worker node info table invisible in dark theme
The HTML table for the worker's master node info used hardcoded inline styles
(`background:#fafafa`, `border-color:#e8e8e8`). In dark theme, inherited white text
on `#fafafa` background made label cells nearly invisible.

## Fix

### Config save
- Changed `ShouldBindJSON` to `ShouldBind` (matches all other controllers in the project)
- Added `form` struct tags to `updateConfigRequest` fields

### dbType mismatch
- Changed dropdown value from `"mysql"` to `"mariadb"` to match the backend constant

### Dark theme
- Extracted inline styles into CSS classes (`.node-info-wrap`, `.node-info-table`)
- Added `.dark` theme overrides using existing CSS custom properties

## Files Changed

- `web/controller/node.go`: `ShouldBindJSON` → `ShouldBind`, added `form` tags
- `web/html/nodes.html`: Fixed dbType value, replaced inline styles with theme-aware CSS
