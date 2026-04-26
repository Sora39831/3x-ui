# Task Record

Date: 2026-04-26
Related Module: web/service
Change Type: Add

## Background
Need a BackupService to handle database backup, restore, and retention operations. Supports both SQLite (via sqlite3 .dump) and MariaDB (via mysqldump). Backups are stored as tar.gz archives containing dump.sql and metadata.json.

## Changes
- Created web/service/backup.go with BackupService struct
- BackupService.CreateBackup: dumps database and creates tar.gz archive
- BackupService.ListBackups: lists all backup files sorted by time
- BackupService.RestoreBackup: validates dbType match, creates safety backup, restores from dump
- BackupService.DeleteBackup: deletes a backup file
- BackupService.ApplyRetention: keeps max N newest backups, removes older ones
- BackupService.CreateSnapshot: scheduled backup with retention policy
- Internal helper functions: dumpMariaDB, dumpSQLite, restoreDB, createTarGz, extractTarGz
- checkNodeRole enforces master-node-only restriction
- Safety backup (pre-restore-*.tar.gz) created before any restore

## Impact
- New file: web/service/backup.go
- No existing files modified
- No API, database, or config changes

## Verification
- go build ./... PASS
- gofmt -l -w . PASS (no formatting changes needed)

## Risks And Follow-Up
- External dependencies on mysqldump, mysql, and sqlite3 CLI tools must be present on the system
- Not yet wired to any API endpoint or cron scheduler; those will be added in subsequent tasks
