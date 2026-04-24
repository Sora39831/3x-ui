# Clash YAML Subscription Endpoint

## Date: 2026-04-24

## Changes

### New Files
- `sub/subClashService.go` — Clash YAML subscription service: reads user template, generates proxies from inbound/client data, injects via `proxies: []` placeholder replacement
- `web/html/settings/panel/subscription/clash.html` — Clash subscription settings panel (path, URI, template textarea)

### Backend
- `web/entity/entity.go` — Added `SubClashEnable`, `SubClashPath`, `SubClashURI`, `SubClashTemplate` to `AllSetting`
- `web/service/setting.go` — Added defaults, getter functions, `GetSubSettings()` auto-build URI for Clash
- `sub/sub.go` — Read Clash settings, pass to controller
- `sub/subController.go` — Added Clash fields, route `GET /clash/:subid`, `clashSubs()` handler returning `text/yaml`

### Frontend
- `web/html/settings.html` — Added Clash settings tab (key 6)
- `web/html/settings/panel/subscription/general.html` — Added Clash enable toggle
- `web/html/settings/panel/subscription/subpage.html` — Added Clash QR code, Desktop dropdown with Clash Verge deep link
- `web/assets/js/subscription.js` — Added `subClashUrl`, `clashvergeUrl` prefers Clash URL
- `web/assets/js/model/setting.js` — Added Clash defaults
- `web/html/inbounds.html` — Added `subClashEnable`, `subClashURI` to subscription settings
- `web/html/modals/qrcode_modal.html` — Added Clash QR code + `genSubClashLink()`
- `web/html/modals/inbound_info_modal.html` — Added Clash subscription link
- `web/translation/translate.en_US.toml` — Added `subClashEnable` i18n
- `web/translation/translate.zh_CN.toml` — Added `subClashEnable` i18n

### Version
- `config/version` — Bumped to v1.5.2-beta
