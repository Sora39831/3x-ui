# Tasktracking: Node Management Sidebar

**Date:** 2026-04-24
**Branch:** fix
**Status:** Done
**Tags:** v1.6.0-beta, v1.6.1, v1.6.3, v1.6.4, v1.6.5

## Overview

Adding a Node Management sidebar page to the 3x-ui web panel for cluster node visibility.

## Tasks

| # | Task | Status | Commit |
|---|------|--------|--------|
| 1 | Add `GetNodeStates` database query | DONE | 85c6b661 |
| 2 | Create `NodeController` with API endpoints | DONE | 16eb179e |
| 3 | Register `NodeController` routes in server | DONE | — |
| 4 | Add i18n translations for node page | DONE | fc77154c |
| 5 | Add sidebar menu item | DONE | c09c6182 |
| 6 | Create `nodes.html` template page | DONE | 7d75d02c |
| 7 | Build and verify | DONE | — |
| 8 | Fix themeSwitcher + API 404 errors | DONE | 07fecdbf |
| 9 | Fix gofmt formatting | DONE | a3d8e9c5 |
| 10 | Fix shared MariaDB query for node states | DONE | d5bf2858 |
| 11 | Fix node settings not auto-created in x-ui.json | DONE | d733ff2a |
| 12 | Fix master heartbeat not visible to workers | DONE | 226bae2b |

## v1.6.3 Fix Details

**Problem:** Node settings (`nodeRole`, `nodeId`, `syncInterval`, `trafficFlushInterval`) were not present in `x-ui.json` on fresh install. Users had to manually configure them via the database settings menu before they appeared.

**Root cause:** These keys were not in `defaultValueMap` in `web/service/setting.go`, so they were never auto-created when the panel initialized settings.

**Fix (commit d733ff2a):**
- Added `nodeRole`, `nodeId`, `syncInterval`, `trafficFlushInterval` to `defaultValueMap`
- Added `"node"` group to `settingGroups`
- Updated `settingGroupAliases` in `config/config.go` to look in `"node"` first, then `"other"` for backward compat
- Updated `ensureDefaultNodeSettings` to write to `"node"` group

## v1.6.4 Fix Details

**Problem:** Worker node page showed no connected master node, even though master had nodeId configured and was running.

**Root cause:** `updateNodeState()` in `node_sync.go` wrote heartbeats to `database.GetDB()` — the local SQLite for a master using SQLite. Workers query the shared MariaDB via `getNodeStatesFromShared()`, so the master's heartbeat was never visible to workers.

**Fix (commit 226bae2b):**
- Master now also writes heartbeat to shared MariaDB via `writeStateToSharedMariaDB()` when MariaDB connection settings are configured
- Check uses `dbUser` (no default value) as the most reliable indicator that MariaDB is configured
- This works even when `dbType` is "sqlite" (master uses SQLite locally but has shared MariaDB settings)

## Task 2 Details

**File created:** `web/controller/node.go`

**Endpoints:**
- `GET /node/list` — returns connected nodes with online/offline status based on heartbeat threshold
- `GET /node/config` — returns current node role, ID, sync intervals, and DB connection settings
- `POST /node/config` — validates and persists node + DB settings to `x-ui.json`

**Bug fixes applied from spec:**
1. Added missing `"time"` import (used by `time.Now().Unix()`)
2. Added missing `"os"` import (used by `os.ErrInvalid`)
3. Removed unused `"net/http"` import
4. Removed unused `"model"` import (return type of `database.GetNodeStates()` is inferred)
