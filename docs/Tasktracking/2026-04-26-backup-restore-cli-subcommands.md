# Task Record

Date: 2026-04-26
Related Module: main
Change Type: Add

## Background
The BackupService in `web/service/backup.go` only works when the panel is running. CLI users need standalone `backup` and `restore` subcommands that use the same SQLite/MariaDB CLI tools without starting the web server.

## Changes
- Added `backupCmd` and `restoreCmd` flag sets in `main()`
- Added "backup" and "restore" entries to `flag.Usage`
- Added `case "backup":` and `case "restore":` to the switch statement
- Added helper functions: `runBackup()`, `runRestore()`, `dumpMariaDBCLI()`, `dumpSQLiteCLI()`, `createTarGzCLI()`
- Added imports: `archive/tar`, `compress/gzip`, `encoding/json`, `io`, `os/exec`, `path/filepath`, `strings`, `time`

## Impact
- main.go only; no API, database schema, or config changes
- Uses existing config package exported functions

## Verification
- `go build -ldflags "-w -s" -o /usr/local/x-ui/x-ui ./main.go` passed
- `gofmt -l -w . && go vet ./...` passed
- Runtime verification not done (requires sqlite3/mysqldump/mysql CLI tools on a live server)

## Risks And Follow-Up
- Runtime depends on `sqlite3`, `mysqldump`, and `mysql` CLI tools being installed on the server
- Restore is restricted to master nodes only
- No automatic MariaDB table drop before restore; manual table cleanup may be needed
