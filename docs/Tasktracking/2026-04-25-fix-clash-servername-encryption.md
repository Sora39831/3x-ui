# Fix: use servername and add encryption to Clash proxy entries

## Date: 2026-04-25

## Changes
- `sub/subClashService.go` — `sni` → `servername` (correct mihomo/Clash Meta field name)
- `sub/subClashService.go` — Added `encryption` field parsed from `inbound.Settings.encryption`

## Version
- v1.7.2.3
