# Fix User Panel Traffic Bar Layout & Last Online Date

**Date:** 2026-05-06
**Version:** 1.8.2.4

## Changes

### Fix traffic bar text and progress bar on same line
- Wrapped "used / total" text in `<div>` block element so the progress bar renders below the text instead of inline

### Fix last online date showing wrong value
- Removed `* 1000` from `new Date(traffic.lastOnline * 1000)` — the backend already returns milliseconds, the extra multiplication caused incorrect date display
- Admin panel correctly uses `new Date(ts)` without multiplication

## Files modified
- `web/html/user.html` — div wrapper + remove * 1000
- `config/version` — bump to 1.8.2.4
