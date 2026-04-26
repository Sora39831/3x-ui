# Task Record

Date: 2026-04-26
Related Module: web/job, web/controller, web
Change Type: Add

## Background
Database backups were only possible manually via API. There was no scheduled backup mechanism, so users had to remember to create backups or rely on external cron.

## Changes
- Created `web/job/backup_job.go`: BackupJob struct with Run() and shouldRun() methods that check backup settings and execute CreateSnapshot when the configured schedule matches the current time
- Registered BackupController routes in `web/controller/server.go` initRouter() to expose backup API endpoints under `/panel/api/server/`
- Scheduled BackupJob via cron in `web/web.go` startTask() at `@every 1m` interval (the job internally checks whether it should actually run)

## Impact
- New file: `web/job/backup_job.go`
- Modified: `web/controller/server.go` (3 lines added in initRouter)
- Modified: `web/web.go` (2 lines added in startTask)
- No database schema changes, no API breaking changes, no config changes

## Verification
- `go build ./...` passed with no errors
- `gofmt -l -w .` produced no changes (files already properly formatted)

## Risks And Follow-Up
- BackupJob has zero-value service fields; methods rely on DB queries at runtime, same pattern as other jobs (CheckXrayRunningJob, LdapSyncJob)
- The job uses `logger.Warning` (not `logger.Warn`) — verify this matches the logger API
- Need to verify that Backups setting page in UI has the scheduling options (BackupEnabled, BackupFrequency, BackupHour) to make the job functional
