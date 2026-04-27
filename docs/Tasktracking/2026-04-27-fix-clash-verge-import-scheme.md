# Task Record

Date: 2026-04-27
Related Module: web/html/user.html
Change Type: Fix

## Background
The "Desktop" quick import button on `/panel/user` uses the `clash-verge://install-config` URL scheme to trigger subscription import in Clash Verge Rev. However, the Clash Verge Rev `extract_subscription_url` function in `scheme.rs` performs a raw string search for `url=` and takes everything to the end of the query string as the subscription URL. When `name` parameter is placed after `url`, the extracted URL becomes corrupted (e.g., `ENCODED_URL&name=ENCODED_NAME`), causing import to fail.

## Changes
- `web/html/user.html`: Swapped parameter order in `quickImportDesktop` method from `url=...&name=...` to `name=...&url=...`, ensuring the `url` parameter is the last query parameter so Clash Verge Rev correctly extracts only the subscription URL.

## Impact
- Only affects the Desktop (Clash Verge Rev) quick import button behavior
- No API, database, or build impact
- Android and iOS buttons unchanged

## Verification
- `gofmt -l -w .` passed with no changes to Go files
- `go vet ./...` passed
- HTML change is a one-line JS string change, no build needed

## Risks And Follow-Up
- Clash Verge Rev may fix their URL parsing in a future version; this parameter order remains compatible either way
