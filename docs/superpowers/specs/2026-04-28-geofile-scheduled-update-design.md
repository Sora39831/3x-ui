# Geofiles Scheduled Update Design

## Summary

Add configurable scheduled geofile updates, settable from both the web panel and `x-ui.sh`. Configuration stored in `x-ui.json` as the single source of truth.

## Configuration

New `geofileUpdate` group in `/etc/x-ui/x-ui.json`:

```json
{
  "geofileUpdate": {
    "enabled":   false,
    "frequency": "daily",
    "hour":      4
  }
}
```

- `enabled`: bool, default `false`
- `frequency`: `hourly` | `every12h` | `daily` | `weekly`
- `hour`: int (0-23), used for `daily` and `weekly` frequencies

## Backend Job

New file `web/job/geofile_update_job.go`, following the existing `BackupJob` pattern:

- Registered as `@every 1m` in `web/web.go` `startTask()`
- `Run()` checks `enabled`, then `shouldRun(frequency)` for time matching
- `shouldRun()` logic:
  - `hourly`: minute == 0
  - `every12h`: (hour == 0 || hour == 12) && minute == 0
  - `daily`: hour == configuredHour && minute == 0
  - `weekly`: weekday == Sunday && hour == configuredHour && minute == 0
- On match: if not master node, returns immediately. Otherwise calls `UpdateGeofile("")` then bumps shared geo version in DB to notify workers

### Files touched

| File | Change |
|------|--------|
| `web/job/geofile_update_job.go` | New file |
| `web/web.go` | Add cron registration line |
| `web/service/setting.go` | Add `defaultValueMap` entries, `settingGroups` mapping, getter methods |
| `web/service/server.go` | No changes (UpdateGeofile already exposed) |

## Panel API

No new API endpoints needed. Existing `POST /panel/api/setting/saveSetting` and `GET /panel/api/setting/getSetting` handle arbitrary settings via `settingGroups` mapping.

## Panel UI

In `web/html/index.html`, inside the existing Geofiles collapse panel:

- **Toggle switch**: enabled/disabled
- **Frequency dropdown**: Hourly / Every 12 hours / Daily / Weekly
- **Hour dropdown**: 0-23, visible only when frequency is `daily` or `weekly`

Reuses existing `loadSetting()` / `saveSetting()` Vue methods.

### New Vue data fields

- `geofileUpdateEnabled`: boolean
- `geofileUpdateFrequency`: string
- `geofileUpdateHour`: number

### New i18n keys

- `geofileUpdate` — "Scheduled Update"
- `geofileUpdateEnabled` — "Enable Scheduled Update"
- `geofileUpdateFrequency` — "Update Frequency"
- `geofileUpdateHour` — "Update Hour"
- (Chinese translations in `translate.zh_CN.toml`)

## x-ui.sh

New subcommand `geofile-cron`:

```
x-ui geofile-cron --enable --frequency daily --hour 4
x-ui geofile-cron --disable
x-ui geofile-cron --status
```

- Directly reads/writes `geofileUpdate` section in `x-ui.json`
- `--status` prints current config to stdout
- `--enable` writes config, prints confirmation, then runs `systemctl restart x-ui`
- `--disable` sets `enabled: false`, then runs `systemctl restart x-ui`
- Config file path: `/etc/x-ui/x-ui.json` (same as `config.GetSettingPath()`)

## Multi-Node Behavior

- Job runs on all nodes; `Run()` checks `IsMaster` and returns immediately on workers
- After master updates, calls `database.BumpSharedGeoVersion()` to notify workers
- Worker nodes detect shared version bump via existing `NodeSyncService.syncGeoIfNeeded()` and auto-pull

## Data Flow

```
Panel UI ──saveSetting──→ x-ui.json ──read by──→ GeofileUpdateJob.Run()
                                                    │
x-ui.sh ──write──→ x-ui.json ──restart──→ panel re-reads settings
```

## Error Handling

- Download failures logged, do not block next scheduled run
- `geofile_versions.json` only updated on successful download (existing behavior)
- Settings with invalid values fall back to defaults via `defaultValueMap`
