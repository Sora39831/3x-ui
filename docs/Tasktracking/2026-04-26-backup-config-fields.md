# Task Record

Date: 2026-04-26
Related Module: web/entity, web/service
Change Type: Add

## Background
Need to support scheduled database backup configuration via panel settings. This adds the backup-related fields (enabled, frequency, hour, max count) to the AllSetting entity, default value map, setting groups, and getter methods on SettingService.

## Changes
- Added BackupEnabled, BackupFrequency, BackupHour, BackupMaxCount fields to AllSetting struct in web/entity/entity.go
- Added default values for backup settings in defaultValueMap in web/service/setting.go
- Added "backup" group to settingGroups in web/service/setting.go
- Added GetBackupEnabled, GetBackupFrequency, GetBackupHour, GetBackupMaxCount getter methods to SettingService

## Impact
- AllSetting struct gains 4 new fields (no breaking change)
- SettingService gains 4 new getter methods
- New "backup" group available in settings API responses
- Default values: enabled=false, frequency=daily, hour=3, maxCount=10

## Verification
- `go build ./...` passed with no errors
- `gofmt -l -w .` produced no formatting changes
- Manually reviewed diff: fields, defaults, group, and getters all follow existing patterns

## Risks And Follow-Up
- None. These are additive changes only; no existing functionality is modified.
