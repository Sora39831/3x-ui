# Panel Settings JSON Migration Design

## Overview

Extract panel settings from the SQLite `settings` table into a standalone JSON file (`x-ui.json`) located in the same directory as the database (`/etc/x-ui/` by default). The `xrayTemplateConfig` remains in the database.

## Requirements

- Panel settings (webPort, tgBot*, sub*, ldap*, etc.) stored in a flat key-value JSON file
- `xrayTemplateConfig` stays in the database `settings` table
- All new installations (no migration from existing DB)
- JSON file path: `<DB_FOLDER>/x-ui.json` (same directory as `x-ui.db`)
- JSON file auto-created on first run with default values

## Architecture

### File Layout

```
/etc/x-ui/
  x-ui.db       # SQLite: users, inbounds, client_traffics, xrayTemplateConfig
  x-ui.json     # Panel settings (flat key-value JSON)
```

### JSON Format

```json
{
  "webListen": "",
  "webPort": "2053",
  "webCertFile": "",
  "webKeyFile": "",
  "secret": "random32chars...",
  "webBasePath": "/",
  "sessionMaxAge": "360",
  "tgBotEnable": "false",
  "tgBotToken": "",
  "subEnable": "true",
  "ldapEnable": "false",
  ...
}
```

All values are strings (consistent with current DB storage). No `xrayTemplateConfig` key.

## Changes

### 1. `config/config.go`

Add `GetSettingPath()` function:

```go
func GetSettingPath() string {
    return fmt.Sprintf("%s/%s.json", GetDBFolderPath(), GetName())
}
```

### 2. `web/service/setting.go`

Replace database-backed `getSetting`/`saveSetting` with JSON file operations:

- **`loadSettings()`** — reads JSON file into `map[string]string`; creates file from `defaultValueMap` if not exists
- **`saveSettings(settings)`** — writes `map[string]string` to JSON file
- **`getSetting(key)`** → read from JSON map
- **`saveSetting(key, value)`** → update key in JSON map, write back
- **`getString(key)`** → `getSetting(key)` with fallback to `defaultValueMap`
- **`GetAllSetting()`** → load JSON map, populate `AllSetting` struct via reflection (same as current, data source changes)
- **`UpdateAllSetting()`** → reflect fields into map, save to JSON
- **`ResetSettings()`** → delete JSON file + clear users table

Remove `import "github.com/mhsanaei/3x-ui/v2/database"` and `model` imports (no longer needed for settings operations).

### 3. `web/service/xray_setting.go`

`XraySettingService.SaveXraySetting()` and related methods continue using the database directly for `xrayTemplateConfig`:

- Replace `s.SettingService.saveSetting("xrayTemplateConfig", ...)` with direct DB operation via `database.GetDB()`
- Add a private helper `saveXraySettingToDB()` / `getXraySettingFromDB()` for direct DB access

### 4. `database/db.go`

Keep `model.Setting{}` in `initModels()` — the `settings` table still stores `xrayTemplateConfig`.

### 5. `main.go`

No changes needed. CLI commands use `SettingService` which handles JSON internally.

The only change: `resetSetting()` calls `settingService.ResetSettings()` which now deletes the JSON file instead of DB rows. The `users` table clearing logic is preserved.

## Data Flow

### Reading

```
Controller/CLI → SettingService.GetString("webPort")
  → loadSettings() [reads x-ui.json]
  → returns "2053" (or default if missing)
```

### Writing

```
Controller/CLI → SettingService.SetPort(8080)
  → setInt("webPort", 8080)
  → setString("webPort", "8080")
  → saveSetting("webPort", "8080")
  → loadSettings() → update map["webPort"] = "8080" → saveSettings()
```

### Xray Config (unchanged path)

```
XraySettingService.SaveXraySetting(config)
  → validate config
  → database.GetDB().Where("key = ?", "xrayTemplateConfig").Save(...)
```

## Error Handling

- JSON file read failure: return error (panel cannot start without settings)
- JSON file write failure: return error (settings update fails, no silent data loss)
- JSON file not found: auto-create from defaults (first run)
- Malformed JSON: return error with clear message
- Concurrent access: Go's single-goroutine web server model means no concurrent write issues for settings

## Testing

- Verify first run creates `x-ui.json` with correct defaults
- Verify `GetAllSetting()` returns correct values from JSON
- Verify `UpdateAllSetting()` writes all fields to JSON
- Verify CLI `x-ui setting -port 8080` updates JSON file
- Verify CLI `x-ui setting -reset` deletes JSON file and recreates on next access
- Verify `xrayTemplateConfig` still works via database
- Verify `x-ui setting -show` reads from JSON file correctly
