# Task Record

Date: 2026-04-27
Related Module: web/html/user.html
Change Type: Fix

## Background
The Desktop quick import button used `clash-verge://install-config` with a `name` query parameter, but Clash Verge Rev official documentation only specifies `clash://install-config?url=<encoded_url>` with no `name` parameter. Additionally, the Clash Verge Rev `extract_subscription_url` function takes everything from `url=` to end of query string, causing corruption when extra params follow.

## Changes
- `web/html/user.html`: Changed the Desktop quick import URL scheme from `clash-verge://install-config?name=...&url=...` to `clash://install-config?url=...`, matching the official Clash Verge Rev URL Schemes documentation exactly. Removed the undocumented `name` parameter.

## Impact
- Only affects the Desktop (Clash Verge Rev) quick import button
- No API, database, or build impact
- Android (`clash://install-config`) and iOS (`shadowrocket://`) buttons unchanged

## Verification
- `gofmt -l -w .` passed
- `go vet ./...` passed
- Official docs confirmed: https://www.clashverge.dev/guide/url_schemes.html specifies `clash://install-config?url=<URI编码后url>`

## Risks And Follow-Up
- The removed `name` parameter means the imported profile in Clash Verge Rev will use the default name from the subscription response instead of the username; this can be restored if Clash Verge Rev fixes their URL parser
