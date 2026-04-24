# Tasktracking: Node Management Sidebar

**Date:** 2026-04-24
**Branch:** fix
**Status:** In Progress

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
| 6 | Create `nodes.html` template page | PENDING | — |
| 7 | Build and verify | PENDING | — |

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
