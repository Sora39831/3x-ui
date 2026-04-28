# Task Record

Date: 2026-04-27
Related Module: database/shared_state, web/controller/server, web/service/node_sync, web/html/index
Change Type: Add

## Background
The user needed the ability to trigger "Update Geofiles" on all worker nodes from the master node's UI. Previously, Geofile updates only operated locally on the node where the button was clicked. Workers only synced inbound configurations via `SharedAccountsVersion`; there was no mechanism for the master to broadcast arbitrary actions to workers.

## Changes
- `database/shared_state.go`: Added `SharedGeoVersionKey` constant and `GetSharedGeoVersion`/`BumpSharedGeoVersion`/`seedSharedGeoVersion` functions, following the same pattern as `SharedAccountsVersion`.
- `web/controller/server.go`: Added `POST /syncUpdateGeofile` route and `syncUpdateGeofile` handler that updates local Geofiles, then bumps `SharedGeoVersion` in the shared database if shared mode is enabled.
- `web/service/node_sync.go`: Extended `NodeSyncService` with `lastGeoVersion`, `loadGeoVersion`, `updateGeofiles` fields and a `syncGeoIfNeeded()` method. Workers check for geo version changes on each sync tick and trigger `UpdateGeofile("")` when a new version is detected.
- `web/html/index.html`: Added "Sync Update" button next to "Update all" in the Geofiles panel, plus the `syncUpdateGeofile()` JavaScript method.
- Translation files: Added i18n keys `geofileSyncUpdate`, `geofileSyncUpdateDialog`, `geofileSyncUpdateDialogDesc` in both zh_CN and en_US.

## Impact
- New API endpoint: `POST /panel/api/server/syncUpdateGeofile` (triggers local + broadcast Geofile update)
- Worker sync loop now includes geo version checking on every tick
- UI: New "同步更新" button in the Geofiles section of the Xray version modal
- Database: New `shared_geo_version` key in the `shared_state` table (on first use)
- No breaking changes to existing API or database schema

## Verification
- `gofmt -l -w .` passed with no changes
- `go vet ./...` passed with no errors

## Risks And Follow-Up
- Geofile sync uses the same polling interval as inbound sync (`SyncIntervalSeconds`); there may be a delay before workers pick up the new version
- If the shared database is unreachable when bumping the geo version, workers will not be notified but the local update will have already succeeded (handled via warning log)
- Future: could add a notification to the master's UI showing which workers have/haven't applied the geo update
