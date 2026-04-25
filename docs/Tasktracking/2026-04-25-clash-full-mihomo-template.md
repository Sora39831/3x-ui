# Clash Link: Full Mihomo Template + Multi-Server Support

## Date: 2026-04-25

## Changes

### Backend
- `config/config.go` — Added `GetClashTemplatePath()`, `GetServersPath()`, `ReadClashTemplate()`, `SaveClashTemplate()`, `ReadServers()`, `SaveServers()`. Files stored at `/etc/x-ui/clash_template.yaml` and `/etc/x-ui/servers.yaml`
- `sub/subClashService.go` — Added `splitTemplate()` (from mihomo-gen), modified `GetClash()` to split at `proxies:`/`proxy-groups:` markers instead of `proxies: []` replacement. Added multi-server support: each `ClashServer` × each client generates a proxy entry. Falls back to old approach if split fails.
- `sub/sub.go` — Reads template and servers from files via `config.ReadClashTemplate()`/`config.ReadServers()`. Added `ClashServer` struct and `parseServers()`.
- `sub/subController.go` — Updated `NewSUBController` to accept `clashServers []ClashServer`
- `web/controller/xray_setting.go` — Added 4 API endpoints: `GET/POST /xray/clashTemplate`, `GET/POST /xray/servers`
- `web/service/setting.go` — Cleared `subClashTemplate` default (template now from file)

### Frontend
- `web/html/settings/xray/advanced.html` — Added "Clash" and "Servers" radio buttons in Xray advanced config
- `web/html/xray.html` — Added `clashTemplate`/`servers` data with old-value tracking, load/save methods, YAML CodeMirror mode, smart save button dispatches to correct save handler
- `web/html/settings/panel/subscription/clash.html` — Removed template editor (now in Xray advanced config)
- `web/html/settings.html` — Removed `initClashCodeMirror()` (template editor moved)
- `web/translation/translate.en_US.toml` — Added "Servers" key
- `web/translation/translate.zh_CN.toml` — Added "Servers" key

### Version
- `config/version` — Bumped to v1.7.2.1
