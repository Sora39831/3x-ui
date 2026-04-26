# Task Record

Date: 2026-04-26
Related Module: database, web, x-ui.sh
Change Type: Feature

## Background

MariaDB has no backup or snapshot functionality. Users need to create backups, schedule automatic snapshots, export/download backup files, and restore from backups — all operable from the panel UI and x-ui.sh CLI.

## Changes

- Added backup config fields to `AllSetting` entity and `SettingService` (backupEnabled, backupFrequency, backupHour, backupMaxCount)
- Created `web/service/backup.go` — BackupService with mysqldump/sqlite3 dump, tar.gz archiving, restore with safety backup, retention policy, node role check
- Created `web/controller/backup.go` — BackupController with 5 API endpoints (create, restore, delete, list, download)
- Created `web/job/backup_job.go` — BackupJob for scheduled snapshots (hourly/every12h/daily/weekly)
- Wired up routes in `web/controller/server.go` and job scheduling in `web/web.go`
- Created `web/html/settings/backup.html` — Backup tab UI with config, manual backup, backup list table
- Added backup tab to `web/html/settings.html` with Vue.js data/methods
- Added backup fields to `web/assets/js/model/setting.js`
- Added `backup` and `restore` CLI subcommands to `main.go`
- Added `backup`/`restore`/`list-backups` subcommands and db_menu items 17-19 to `x-ui.sh`
- Added filename validation (regex) to prevent path traversal in backup file operations
- Exported ValidateBackupFilename for consistent validation across service and controller

## Impact

- New files: web/service/backup.go, web/controller/backup.go, web/job/backup_job.go, web/html/settings/backup.html
- Modified: web/entity/entity.go, web/service/setting.go, web/controller/server.go, web/web.go, web/html/settings.html, web/assets/js/model/setting.js, main.go, x-ui.sh
- Database schema: no changes (backup settings stored via existing settings key-value table)
- API: 5 new endpoints under /panel/api/server/
- CLI: 2 new subcommands (backup, restore)
- x-ui.sh: 3 new subcommands, 4 new db_menu items
- Build: requires CGO_ENABLED=1, go run ./cmd/genassets first

## Verification

- go build ./... — PASS
- go vet ./... — PASS
- gofmt -l -w . — PASS
- go test -race ./database/... — PASS
- go test -race ./web/service/... — PASS
- go test -race ./config/... — PASS
- CGO_ENABLED=1 go build -ldflags "-w -s" -o /tmp/x-ui ./main.go — PASS

## Risks And Follow-Up

- mysqldump/sqlite3 must be installed on target system; x-ui.sh install_mariadb_client can install MariaDB client
- Large DB backups may be slow; UI has no progress indicator (uses default HTTP timeout)
- Node role check prevents worker nodes from backup/restore — correct behavior
- Restore requires panel downtime (systemctl stop/start) — communicated via UI confirmation dialog
- Safety backups (pre-restore-*.tar.gz) are now visible in backup list and included in retention count
- exec.Command calls lack timeout; consider adding context.WithTimeout in future iteration
