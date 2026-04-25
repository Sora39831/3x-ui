# MariaDB Support for 3x-ui

## Summary

Add MariaDB as an alternative database backend to SQLite. Users switch between SQLite and MariaDB via the `x-ui.sh` management script (option 27). Data is automatically migrated during the switch. MariaDB connection credentials are stored in `/etc/x-ui/x-ui.json`.

## Requirements

- Support both SQLite and MariaDB as database backends
- Switch via `x-ui.sh` with interactive prompts for MariaDB credentials (IP, port, username, password, database name)
- Auto-migrate data when switching between SQLite and MariaDB
- Keep old database as backup after migration
- MariaDB has core feature parity (CRUD, migrations, seeders) but skips SQLite-specific features (WAL checkpoint, file export/import)
- Credentials stored in `/etc/x-ui/x-ui.json`

## Architecture: Approach A — Driver-agnostic `InitDB`

Refactor `database.InitDB()` to read config from the JSON settings file, determine the driver type, and open the appropriate GORM connection. The package-level `var db *gorm.DB` singleton stays unchanged — all callers continue using `database.GetDB()`.

---

## Section 1: Configuration

### New settings in `web/service/setting.go`

Add to `defaultValueMap`:

| Key | Default | Description |
|-----|---------|-------------|
| `dbType` | `"sqlite"` | `"sqlite"` or `"mariadb"` |
| `dbHost` | `"127.0.0.1"` | MariaDB host |
| `dbPort` | `"3306"` | MariaDB port |
| `dbUser` | `""` | MariaDB username |
| `dbPassword` | `""` | MariaDB password |
| `dbName` | `"3xui"` | MariaDB database name |

Add getter/setter methods: `GetDBType()`, `SetDBType()`, `GetDBHost()`, `SetDBHost()`, `GetDBPort()`, `SetDBPort()`, `GetDBUser()`, `SetDBUser()`, `GetDBPassword()`, `SetDBPassword()`, `GetDBName()`, `SetDBName()`.

### Config reading before DB init

Problem: settings are stored IN the database, but we need `dbType` BEFORE opening the DB.

Solution: `config/config.go` gets a `GetDBTypeFromJSON()` function that reads `/etc/x-ui/x-ui.json` directly (falls back to `"sqlite"` if file doesn't exist or key is missing). This is called before `database.InitDB()`.

### New CLI flags in `main.go`

Add `-dbType`, `-dbHost`, `-dbPort`, `-dbUser`, `-dbPassword`, `-dbName` flags to the `setting` subcommand. These write directly to the JSON config file (not via the DB) using `config.WriteSettingToJSON(key, value)`.

New `config/config.go` helper: `WriteSettingToJSON(key, value string)` — reads the JSON file, updates the key, writes back.

---

## Section 2: Database Layer (`database/db.go`)

### Refactored `InitDB()`

```go
func InitDB() error {
    dbType := config.GetDBTypeFromJSON()

    switch dbType {
    case "mariadb":
        return initMariaDB()
    default: // "sqlite"
        return initSQLite(config.GetDBPath())
    }
}
```

### `initSQLite(path string) error`

Existing logic unchanged — opens SQLite with `gorm.io/driver/sqlite`, runs `initModels()`, `initUser()`, `runSeeders()`.

### `initMariaDB() error`

1. Read host, port, user, password, dbName from JSON config.
2. Build DSN: `user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local`
3. Open with `gorm.io/driver/mysql`.
4. Run `initModels()`, `initUser()`, `runSeeders()` (same as SQLite).

### Adapted functions

- `Checkpoint()` — if MariaDB, return `nil`. If SQLite, existing WAL logic.
- `IsSQLiteDB()` — unchanged, only called for SQLite.
- `ValidateSQLiteDB()` — unchanged, only called for SQLite.

### New dependency

`gorm.io/driver/mysql` added to `go.mod`.

---

## Section 3: Data Migration (`database/migrate.go`)

New file with two functions:

### `MigrateSQLiteToMariaDB() error`

1. Open SQLite connection from `config.GetDBPath()`.
2. Open MariaDB connection from JSON settings.
3. For each table (users, inbounds, outbound_traffics, settings, inbound_client_ips, client_traffics, history_of_seeders):
   - AutoMigrate the model on MariaDB.
   - `SELECT *` from SQLite → `INSERT` into MariaDB using GORM raw SQL.
4. On success: close connections (SQLite file kept as backup).
5. On failure: return error with context.

### `MigrateMariaDBToSQLite() error`

Reverse of above:
1. Open MariaDB connection from JSON settings.
2. Open/create SQLite connection at `config.GetDBPath()`.
3. For each table: read from MariaDB, write to SQLite.
4. On success: close connections.
5. On failure: return error.

Row transfer approach: Use the existing model structs explicitly. For each table, query all rows from source into a `[]Model` slice, then batch-insert into destination. This avoids raw SQL differences between SQLite and MySQL. Example for users:

```go
var users []model.User
srcDB.Find(&users)
dstDB.CreateInBatches(&users, 100)
```

This pattern repeats for each of the 7 tables.

---

## Section 4: `main.go` Changes

### Updated callers

All `database.InitDB(config.GetDBPath())` calls change to `database.InitDB()`:
- `runWebServer()` (line 49)
- `resetSetting()` (line 134)
- `updateTgbotSetting()` (line 221)
- `updateSetting()` (line 259)
- `updateCert()` (line 318)
- `migrateDb()` (line 395)

### New `migrate-db` subcommand

```go
case "migrate-db":
    migrateDbBetweenDrivers()
```

`migrateDbBetweenDrivers()`:
1. Read `dbType` from JSON config.
2. If `dbType == "mariadb"`: call `database.MigrateSQLiteToMariaDB()`.
3. If `dbType == "sqlite"`: call `database.MigrateMariaDBToSQLite()`.
4. Print success/failure message.

### New CLI flags

Add to `setting` subcommand:
- `-dbType string` — set database type
- `-dbHost string` — set MariaDB host
- `-dbPort string` — set MariaDB port
- `-dbUser string` — set MariaDB username
- `-dbPassword string` — set MariaDB password
- `-dbName string` — set MariaDB database name

These call `config.WriteSettingToJSON()` to write directly to the JSON file. Only the 6 DB-related settings use `WriteSettingToJSON()` — all other settings (port, username, etc.) continue to use the existing `SettingService` methods that write through the database.

---

## Section 5: `web/service/server.go` Changes

### `GetDb()`

Add check at the top:
```go
dbType, _ := s.GetDBType()
if dbType == "mariadb" {
    return nil, common.NewError("Database export is not supported for MariaDB")
}
```
Existing SQLite logic unchanged.

### `ImportDB()`

Add check at the top:
```go
dbType, _ := s.GetDBType()
if dbType == "mariadb" {
    return common.NewError("Database import is not supported for MariaDB")
}
```
Existing SQLite logic unchanged.

---

## Section 6: `x-ui.sh` Changes

### New menu option 27

Add to `show_menu`:
```
│────────────────────────────────────────────────│
│  ${green}27.${plain} 数据库管理                                │
```

Add to the case statement:
```bash
27)
    check_install && db_menu
    ;;
```

Update prompt: `请输入选择 [0-27]`

### `db_menu()` function

```bash
db_menu() {
    # Read current dbType from JSON
    local current_type=$(read_json_dbtype)

    echo -e "
╔────────────────────────────────────────────────╗
│   ${green}数据库管理${plain}                                    │
│   ${green}0.${plain} 返回主菜单                                │
│   ${green}1.${plain} 查看当前数据库类型（当前: ${current_type}）   │
│   ${green}2.${plain} 切换到 MariaDB                             │
│   ${green}3.${plain} 切换到 SQLite                               │
╚────────────────────────────────────────────────╝
"
    read -rp "请输入选择 [0-3]：" num
    case "${num}" in
    0) show_menu ;;
    1) db_show_status && db_menu ;;
    2) db_switch_to_mariadb ;;
    3) db_switch_to_sqlite ;;
    *) echo "无效选项" && db_menu ;;
    esac
}
```

### `db_switch_to_mariadb()`

```bash
db_switch_to_mariadb() {
    echo "请输入 MariaDB 连接信息（直接回车使用默认值）："

    read -rp "MariaDB IP（默认 127.0.0.1）: " db_host
    db_host=${db_host:-127.0.0.1}

    read -rp "MariaDB 端口（默认 3306）: " db_port
    db_port=${db_port:-3306}

    read -rp "MariaDB 用户名: " db_user
    if [ -z "$db_user" ]; then
        echo -e "${red}用户名不能为空${plain}"
        db_menu
        return
    fi

    read -rsp "MariaDB 密码: " db_pass
    echo
    if [ -z "$db_pass" ]; then
        echo -e "${red}密码不能为空${plain}"
        db_menu
        return
    fi

    read -rp "数据库名（默认 3xui）: " db_name
    db_name=${db_name:-3xui}

    # Write settings to JSON config
    /usr/local/x-ui/x-ui setting -dbType mariadb -dbHost "$db_host" -dbPort "$db_port" -dbUser "$db_user" -dbPassword "$db_pass" -dbName "$db_name"

    # Migrate data
    echo "正在迁移数据从 SQLite 到 MariaDB..."
    /usr/local/x-ui/x-ui migrate-db

    if [ $? -eq 0 ]; then
        echo -e "${green}数据库切换成功，正在重启面板...${plain}"
        restart
    else
        echo -e "${red}数据迁移失败，正在回滚到 SQLite...${plain}"
        /usr/local/x-ui/x-ui setting -dbType sqlite
        restart
    fi
}
```

### `db_switch_to_sqlite()`

```bash
db_switch_to_sqlite() {
    /usr/local/x-ui/x-ui setting -dbType sqlite

    echo "正在迁移数据从 MariaDB 到 SQLite..."
    /usr/local/x-ui/x-ui migrate-db

    if [ $? -eq 0 ]; then
        echo -e "${green}数据库切换成功，正在重启面板...${plain}"
        restart
    else
        echo -e "${red}数据迁移失败${plain}"
    fi
}
```

### Helper functions in x-ui.sh

- `read_json_dbtype()` — reads `dbType` from `/etc/x-ui/x-ui.json` using `grep`/`sed` or Python if available.
- `db_show_status()` — displays current DB type and connection info.

---

## Files Changed

| File | Changes |
|------|---------|
| `go.mod` | Add `gorm.io/driver/mysql` |
| `config/config.go` | Add `GetDBTypeFromJSON()`, `WriteSettingToJSON()` |
| `database/db.go` | Refactor `InitDB()` to be driver-agnostic, add `initMariaDB()`, adapt `Checkpoint()` |
| `database/migrate.go` | **New file** — `MigrateSQLiteToMariaDB()`, `MigrateMariaDBToSQLite()` |
| `main.go` | Update all `InitDB` calls, add `migrate-db` subcommand, add setting CLI flags |
| `web/service/setting.go` | Add 6 new settings + getter/setter methods |
| `web/service/server.go` | Guard `GetDb()`/`ImportDB()` for MariaDB |
| `x-ui.sh` | Add option 27, `db_menu()`, `db_switch_to_mariadb()`, `db_switch_to_sqlite()`, helpers |

## Testing

1. Fresh install with SQLite (default) — verify panel works as before
2. Switch to MariaDB via x-ui.sh — verify data migrates and panel starts
3. Switch back to SQLite — verify data migrates back
4. Verify MariaDB CRUD operations (create inbound, modify settings, etc.)
5. Verify GetDb/ImportDB return appropriate errors when using MariaDB
6. Verify invalid MariaDB credentials show error and rollback to SQLite
