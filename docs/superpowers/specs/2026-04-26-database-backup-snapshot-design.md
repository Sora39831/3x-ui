# Database Backup & Snapshot Design

Date: 2026-04-26
Related Module: database, web, x-ui.sh
Change Type: Feature

## Overview

Add backup, scheduled snapshot, export (download), and restore functionality for both SQLite and MariaDB databases to the 3X-UI panel. Operable via the web panel UI and x-ui.sh CLI.

## Scope

- MariaDB backup/restore (primary goal)
- SQLite backup/restore (unified with MariaDB under the new system)
- Scheduled automatic snapshots (configurable frequency)
- Manual ad-hoc backups
- Backup export/download via panel
- Restore from backup via panel or CLI
- Backup retention policy (keep last N backups)
- Existing `getDb`/`importDB` endpoints remain unchanged for SQLite raw .db file operations

## Node Role Constraint

In MariaDB multi-node mode, the database is shared. Backup and restore operations are restricted to the **master node only**:

- Worker nodes: backup/restore endpoints and CLI commands return an error: "Backup and restore can only be performed on the master node"
- Panel UI on worker nodes: backup tab is hidden or disabled with the above message
- x-ui.sh on worker nodes: `backup`/`restore`/`list-backups` commands show the restriction message
- SQLite mode: no restriction (SQLite is always single-node)
- Node role is read from the JSON config (`nodeRole`: `"master"` or `"worker"`)

## Architecture

### New Files

| File | Purpose |
|------|---------|
| `web/service/backup.go` | Backup service layer: mysqldump/sqlite3 dump execution, gzip, file management, scheduling |
| `web/controller/backup.go` | Backup API controller (HTTP handlers) |
| `web/html/settings/backup.html` | Backup management page (new tab in settings) |
| `web/job/backup_job.go` | Scheduled backup cron job |

### Modified Files

| File | Change |
|------|--------|
| `web/html/settings.html` | Add "Backup" tab entry |
| `web/job/job.go` | Register new backup cron job |
| `web/service/setting.go` | Add backup config read/write to AllSetting |
| `web/entity/entity.go` | Add backup config fields to AllSetting |
| `x-ui.sh` | Add `backup`/`restore`/`list-backups` subcommands; add backup menu items to `db_menu` |
| `main.go` | Register `backup` and `restore` CLI subcommands |

### Dependency Graph

```
Settings Page (backup.html)
    ↓ POST/GET
BackupController (web/controller/backup.go)
    ↓
BackupService (web/service/backup.go)
    ↓ exec.Command("mysqldump") / exec.Command("sqlite3", ".dump")
MariaDB / SQLite Server
    ↓ file I/O
/etc/x-ui/backups/  (tar.gz files)
```

Scheduling:
```
web/job/backup_job.go → BackupService.CreateSnapshot()
                            ↑ reads schedule config
                         web/service/setting.go (backup config)
```

## API Endpoints

All endpoints under `/panel/api/server`, registered in `web/controller/backup.go`. Admin authentication required for all.

| Method | Route | Purpose |
|--------|-------|---------|
| POST | `/panel/api/server/backup` | Create an immediate manual backup |
| POST | `/panel/api/server/restore/:filename` | Restore from a specified backup file |
| POST | `/panel/api/server/deleteBackup/:filename` | Delete a specified backup file |
| GET | `/panel/api/server/listBackups` | List all backups (filename, size, timestamp) |
| GET | `/panel/api/server/downloadBackup/:filename` | Download a specified backup file |
| POST | `/panel/api/server/backupConfig` | Update backup configuration |

Response format: standard `Msg{Success, Msg, Obj}` JSON.

## Backup File Format

All backups use `.tar.gz` archives with a consistent internal structure regardless of database type.

### Storage

- Directory: `/etc/x-ui/backups/`
- Filename: `backup-YYYY-MM-DD-HHmmss.tar.gz`

### Archive Contents

```
backup-2026-04-26-030000.tar.gz
├── metadata.json     # {"dbType":"mariadb"|"sqlite","timestamp":"RFC3339","version":"v1.7.2.x"}
└── dump.sql          # mysqldump output (MariaDB) or sqlite3 .dump output (SQLite)
```

### SQLite Backup Process
1. `database.Checkpoint()` to flush WAL
2. `exec.Command("sqlite3", dbPath, ".dump")` to generate SQL dump
3. Create metadata.json with `dbType: "sqlite"`
4. Bundle into tar.gz

### MariaDB Backup Process
1. Build DSN from config
2. `exec.Command("mysqldump", "--single-transaction", "--routines", "--triggers", ...)` 
3. Create metadata.json with `dbType: "mariadb"`
4. Bundle into tar.gz

## Restore Logic

1. Read `metadata.json` from the archive to extract `dbType`
2. Compare `dbType` with current panel database type
3. If mismatch → reject with error: "Backup type (X) does not match current database (Y)"
4. If match → create a safety backup of current database before proceeding
5. Stop panel → restore → restart panel
6. Restoration:
   - SQLite: drop all tables, import `.dump` via sqlite3
   - MariaDB: drop database, recreate, import via `mysql` client

### Restore Flow

```
Read metadata.json → validate dbType match
  → create safety backup of current DB
  → systemctl stop x-ui (or internal stop)
  → execute restore (sqlite3 or mysql client)
  → systemctl start x-ui (or internal restart)
  → verify panel accessible
```

Safety backup filename pattern: `pre-restore-YYYY-MM-DD-HHmmss.tar.gz`. These are excluded from the retention count and auto-deleted after 7 days.

## Retention Policy

- Configurable via `max_backups` setting (default: 10, range: 1-100)
- After each backup creation, if total count exceeds `max_backups`, delete oldest files first
- Manual and scheduled backups share the same retention pool
- Space protection: refuse new backup if disk free space < 100 MB or backup directory total > 500 MB

## Scheduled Snapshots

Configuration fields in AllSetting:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `backupEnabled` | bool | false | Enable scheduled backups |
| `backupFrequency` | string | "daily" | "hourly", "every12h", "daily", "weekly" |
| `backupHour` | int | 3 | Hour of day (0-23), used by daily and weekly |
| `backupMaxCount` | int | 10 | Max backups to retain |

### Schedule Runtime

| Frequency | When it runs |
|-----------|-------------|
| `hourly` | Minute 0 of every hour |
| `every12h` | 00:00 and 12:00 |
| `daily` | `backupHour:00` each day |
| `weekly` | Sunday at `backupHour:00` |

Scheduling logic in `web/job/backup_job.go`:
- Check `backupEnabled` on each tick
- Execute backup via `BackupService.CreateSnapshot()`
- Apply retention after each successful backup
- Log errors (do not crash the panel)

## Panel UI Design

### Location

New "Backup" tab in the settings page (`settings.html`).

### Layout

**Scheduled Backup Section:**
- Enable toggle switch
- Frequency dropdown: Every Hour / Every 12 Hours / Every Day / Every Week
- Hour picker (visible when daily or weekly selected)
- Max backups input (1-100)
- Save button

**Manual Operations Section:**
- "Create Backup Now" button

**Backup List Section:**
- Table with columns: Filename, Timestamp, Size, Actions
- Action buttons per row: Download, Restore, Delete
- Restore shows a warning confirmation dialog
- Delete requires secondary confirmation
- Auto-refresh every 30 seconds

## x-ui.sh CLI

### New Subcommands

| Command | Purpose |
|---------|---------|
| `x-ui backup` | Create an immediate manual backup |
| `x-ui restore <filename>` | Restore from `/etc/x-ui/backups/<filename>` |
| `x-ui list-backups` | List all backups with size and timestamp |

### db_menu Additions (menu 27)

New items after existing 1-16:

| # | Label | Action |
|---|-------|--------|
| 17 | Create database backup | Call `x-ui backup` |
| 18 | Restore from backup | Interactive file selection, then restore |
| 19 | List all backups | Show backup list |
| 20 | Configure auto-backup | Interactive config (frequency, retention) |

### Implementation

- `backup` and `restore` subcommands delegate to Go binary: `${xui_folder}/x-ui backup` / `${xui_folder}/x-ui restore --file=<name>`
- `list-backups` uses `ls -lh /etc/x-ui/backups/`
- Restore flow in x-ui.sh: validate → confirm → `systemctl stop x-ui` → `${xui_folder}/x-ui restore` → `systemctl start x-ui`

## Error Handling

- Worker node attempts backup/restore → reject: "Backup and restore can only be performed on the master node"
- `mysqldump` or `sqlite3` not found on system → clear error message with install instructions
- Disk full → reject backup, notify user
- Backup file corrupted (invalid tar.gz or missing metadata.json) → reject restore
- Database type mismatch → reject with clear message
- Panel not stopped cleanly before restore → timeout and manual recovery instructions

## Testing

- Unit tests for `BackupService` methods (mock exec.Command where possible)
- Integration test: create backup → verify contents → restore → verify data integrity
- Test both SQLite and MariaDB paths
- Test retention policy (create N+1 backups, verify oldest deleted)
- Test dbType mismatch rejection

## Risks And Follow-Up

- `mysqldump` may not be installed on all systems; x-ui.sh already has `install_mariadb_client` that can install it. Panel UI should detect and surface this.
- Large database backups may take significant time; UI should show progress or timeout gracefully.
- Restore requires panel downtime (stop/start). This is communicated to the user before confirmation.
- Existing `getDb`/`importDB` endpoints remain unchanged for backward compatibility.
