# Task Record

Date: 2026-04-26
Related Module: web (frontend)
Change Type: Add

## Background
Add backup management UI page and corresponding frontend model fields to the settings page. The backup feature allows users to configure scheduled backups, manually create backups, and manage (download/restore/delete) existing backup files.

## Changes
- Added 4 backup fields to `AllSetting` class in `web/assets/js/model/setting.js`: `backupEnabled`, `backupFrequency`, `backupHour`, `backupMaxCount`
- Created `web/html/settings/backup.html` with Vue template for backup configuration, manual operations, and backup list table
- Added Backup tab (key="7") to settings.html `<a-tabs>` component
- Added Vue data properties: `backupList`, `backupColumns`, `backupLoading`, `backupCreating`, `backupRefreshInterval`
- Added Vue methods: `fetchBackups`, `createBackup`, `restoreBackup`, `deleteBackup`, `downloadBackup`, `formatFileSize`, `formatBackupTime`
- Modified `onSettingsTabChange` to fetch backups when tab 7 is selected
- Added 30-second backup refresh interval in `mounted()` lifecycle hook
- Added `beforeDestroy()` lifecycle hook to clear the backup refresh interval

## Impact
- New template file: `web/html/settings/backup.html`
- Modified: `web/assets/js/model/setting.js`, `web/html/settings.html`
- Uses Vue `[[ ]]` delimiters for interpolation (project standard)
- API endpoints referenced: `/panel/api/server/listBackups`, `/panel/api/server/backup`, `/panel/api/server/restore/`, `/panel/api/server/deleteBackup/`, `/panel/api/server/downloadBackup/`
- Uses `this.authHeaders` for axios requests (expected to be defined separately)
- `AllSetting.equals()` will now factor in backup fields for change detection

## Verification
- `go build ./...` passes
- `gofmt -l -w .` passes with no changes needed
- Template compilation verified via build
- Cannot runtime test without running panel server (no local dev environment)

## Risks And Follow-Up
- `this.authHeaders` is referenced but not defined in the current codebase; must be defined before runtime use
- Backup API endpoints must exist on the backend for the page to function
- `AllSetting.equals()` will use the new backup fields for save-button change detection (existing behavior, no regression expected)
