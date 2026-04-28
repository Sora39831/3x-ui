# Task Record

Date: 2026-04-27
Related Module: web/service/server, web/controller/server, web/html/index
Change Type: Add

## Background
When downloading geoip.dat and geosite.dat from GitHub releases, the version information (GitHub release tag like `202604262232`) was not captured or displayed. The user wanted to track and show the version of geofiles in the UI.

## Changes
- `web/service/server.go`:
  - Changed `downloadFile` closure to return the captured version string alongside the error
  - Modified `http.Client` to use `CheckRedirect` callback that extracts the release tag from the 302 redirect URL path (format: `/releases/download/{version}/{filename}`)
  - Added `GeofileVersion` struct and `GeofileVersions` map type for version metadata storage
  - Added `loadGeofileVersions()` and `saveGeofileVersions()` for reading/writing `geofile_versions.json` in the bin folder
  - Added `GetGeofileVersions()` public method for API access
- `web/controller/server.go`:
  - Added `GET /getGeofileVersions` endpoint returning version metadata
- `web/html/index.html`:
  - Added `geofileVersions` to Vue data
  - Added `loadGeofileVersions()` method, called when the Xray version modal opens
  - Geofiles panel now displays version string (e.g. `202604262232`) next to each file name
  - Added CSS classes for version text in light/dark themes

## Impact
- New file: `bin/geofile_versions.json` stores version metadata per geofile
- New API: `GET /panel/api/server/getGeofileVersions`
- No database schema changes
- Xray binary filename expectations unchanged (files still saved as `geoip.dat`/`geosite.dat`)

## Verification
- `gofmt -l -w .` passed
- `go vet ./...` passed
- Tested redirect URL parsing logic: path `/releases/download/202604262232/geoip.dat` correctly extracts `202604262232`
- Confirmed `http.Client` with `CheckRedirect` does not interfere with `If-Modified-Since`/`Last-Modified` caching

## Risks And Follow-Up
- Version extraction depends on GitHub's redirect URL format; if GitHub changes the URL structure, version will be empty (graceful degradation — shows `-` in UI)
- Worker nodes: version metadata is written locally on each node after their own download via `syncGeoIfNeeded()`, so each worker has its own `geofile_versions.json`
