# fix: MariaDB JSON_EACH compatibility for subscription and traffic queries

## Date: 2026-04-24

## Problem
Subscription endpoint (`/sub/:subid`, `/json/:subid`) returns `Error!` on MariaDB.
Root cause: `JSON_EACH` is SQLite-only; MariaDB requires `JSON_TABLE`.

## Changes

### sub/subService.go
- `getInboundsBySubId`: branch SQL by DB type — `JSON_TABLE` for MariaDB, `JSON_EACH` for SQLite
- `getFallbackMaster`: same branching for fallback query
- Added `config` import for `GetDBTypeFromJSON()`

### web/service/inbound.go
- `GetClientTrafficByID`: branch SQL by DB type
- `MigrationRemoveOrphanedTraffics`: branch SQL by DB type
- Added `config` import

### config/version
- Bump to v1.5.1

## Not in scope
- `getAllEmails()` (TG Bot) — same issue, deferred

## Verification
- `go build ./...` passes
- `go test ./...` all pass
- `go vet ./sub/ ./web/service/` clean
