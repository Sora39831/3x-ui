# 2026-04-25 — User Panel: Add Clash Link & Quick Import Button

## Summary
Optimized the user panel (`/panel/user`) to show subscription info and add a one-click import dropdown.

## Changes

### Backend
- Added `settingService` field to `InboundController`
- New endpoint `GET /panel/api/inbounds/userSubscriptions` — returns `subId`, `subClashEnable`, `subClashUrl` for the logged-in user
- Route registered before `checkAdmin` middleware so non-admin users can access

### Frontend (`web/html/user.html`)
- Redesigned page with 3 cards:
  1. **User Info** — traffic stats, expiry, status (polished)
  2. **Clash Link** — shows Clash subscription URL with copy button, or "暂无订阅" if not enabled
  3. **Quick Import** — dropdown button with Android/iOS/Desktop options with icons (visual only, functionality TBD)
- Added copy-to-clipboard via `ClipboardManager`

### i18n
- Added keys to `translate.en_US.toml` and `translate.zh_CN.toml`:
  - `clashUrl`, `quickImport`, `android`, `ios`, `desktop`, `copied`, `noSubscription`

## Files Modified
- `web/controller/inbound.go` — added settingService, getUserSubscriptions method
- `web/controller/api.go` — registered new route
- `web/html/user.html` — redesigned user panel page
- `web/translation/translate.en_US.toml` — new i18n keys
- `web/translation/translate.zh_CN.toml` — new i18n keys
- `config/version` — bumped to v1.7.2.5
