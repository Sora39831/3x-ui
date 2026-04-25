# Fix: Clash proxy entries missing reality-opts, client-fingerprint, network

## Date: 2026-04-25

## Changes

### Bug Fixes
- `sub/subClashService.go` — Fixed REALITY settings extraction: `publicKey` is at `realitySettings.settings.publicKey`, not `realitySettings.publicKey`. `shortIds` is an array (use first element). `fingerprint` is at `realitySettings.settings.fingerprint`.
- `sub/subClashService.go` — Added `network` field to all proxy entries (default "tcp")
- `sub/subClashService.go` — Moved non-REALITY fingerprint code inside the `else` branch to avoid duplication

### Version
- `config/version` — Bumped to v1.7.2.1
