# Task Record

Date: 2026-04-26
Related Module: web/controller
Change Type: Add

## Background
Need BackupController to expose backup/restore API endpoints for the web panel. This is part of the backup feature implementation.

## Changes
- Created `web/controller/backup.go` with BackupController struct embedding BaseController
- Added 5 API endpoints: createBackup, restoreBackup, deleteBackup, listBackups, downloadBackup
- Uses service.BackupService for business logic

## Impact
- New file only, no existing files modified
- Routes will be wired in a later task

## Verification
- `go build ./...` passed with no errors
- `gofmt -l -w .` produced no changes (already formatted)

## Risks And Follow-Up
- Routes not yet wired to gin router (Task 5)
- BackupService must be initialized before controller wiring
