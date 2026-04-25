# 2026-04-25 — User Panel: Quick Import URL Schemes

## Summary
Wired up the 3 Quick Import dropdown buttons (Android/iOS/Desktop) with deep link URL schemes to launch proxy client apps directly from the user panel.

## Changes

### Backend (`web/controller/inbound.go`)
- Extended `getUserSubscriptions` API to also return `subEnable` and `subUrl` (standard subscription URL)
- Previously only returned `subClashEnable` and `subClashUrl`

### Frontend (`web/html/user.html`)
- Added `subEnable` and `subUrl` data fields
- Updated `loadSubscriptions()` to save the new fields
- Added 3 URL scheme methods:
  - **Android** → `clash://install-config?url=<encoded_url>` (Clash Meta for Android)
  - **iOS** → `shadowrocket://add/sub/<base64_url>?remark=<name>` (Shadowrocket)
  - **Desktop** → `clash-verge://install-config?url=<encoded_url>&name=<name>` (Clash Verge)
- Added `@click` handlers on the 3 dropdown menu items
- Each method validates subscription availability before opening the URL scheme

### URL Scheme Priority
- Android/Desktop: prefers Clash URL (`subClashUrl`), falls back to standard URL (`subUrl`)
- iOS (Shadowrocket): prefers standard URL (`subUrl`), falls back to Clash URL

## Files Modified
- `web/controller/inbound.go` — extended API response with subEnable/subUrl
- `web/html/user.html` — added URL scheme methods and click handlers
