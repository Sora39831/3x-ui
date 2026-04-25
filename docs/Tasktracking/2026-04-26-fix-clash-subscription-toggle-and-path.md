Task Record

Date: 2026-04-26
Related Module: web settings / subscription assets / setting service
Change Type: Fix

Background

The Clash subscription toggle in `/panel/settings` did not behave correctly in practice.
The page was loading an outdated fingerprinted frontend model from `web/public` that did not include Clash subscription fields, which caused inconsistent binding and update behavior for `subClashEnable` and related properties.
In addition, Clash subscription keys were not mapped in the settings grouping metadata, making their nested config representation inconsistent.

Changes

Regenerated fingerprinted frontend assets via `go run ./cmd/genassets` so the settings page now loads an updated `AllSetting` model containing:
- `subClashEnable`
- `subClashPath`
- `subClashURI`

Updated settings group mappings in `web/service/setting.go`:
- Added Clash keys to `subscriptionNetwork`:
  - `clashEnable -> subClashEnable`
  - `clashPath -> subClashPath`
  - `clashURI -> subClashURI`
- Added corresponding Clash keys to legacy `sub` mapping for compatibility.

Impact

Affected modules or files.
- `web/service/setting.go`
- `web/public/assets-manifest.json`
- `web/public/assets/js/model/setting.*.js` (fingerprinted replacement)
- `web/public/assets/js/subscription.*.js` (fingerprinted replacement)
- `web/public/assets/codemirror/yaml.*.js` (fingerprinted generated asset)

Whether APIs, database, config, build, or compatibility are affected.
- API schema unchanged.
- Database unchanged.
- Settings file structure compatibility improved for Clash keys in nested/legacy mappings.
- Frontend static asset fingerprints updated.

Whether upstream or downstream callers are affected.
- Panel settings frontend now correctly tracks and submits Clash subscription toggle/path fields.
- Subscription page uses refreshed asset bundle.

Verification

List validation commands or checks performed.
- `go run ./cmd/genassets`
- `go test -race ./web/service/...`

State the result.
- Asset generation succeeded.
- Related service tests passed.

If not verified, explain why.
- No live runtime verification against a deployed panel/subscription server was performed in this local environment.

Risks And Follow-Up

Remaining risks.
- Existing browser caches may keep old fingerprint mappings until refresh; after deploy/restart, hard refresh may still be needed in some clients.

Recommended follow-up work.
- Verify in a deployed environment that toggling `Clash Subscription` and editing `subClashPath` immediately reflects expected behavior after restart/reload cycle.
