# Fix Worker Node Uninstall Deleting Master Database

**Date:** 2026-06-03
**Version:** 1.8.2.5

## Background

User reported: uninstalling the 3x-ui panel on a worker node server with the "delete database" option caused the master node's database to be dropped. Root cause analysis revealed three critical flaws in the uninstall logic.

## Root Cause

1. `uninstall()` in `x-ui.sh` never checked `nodeRole` — worker nodes were treated the same as master nodes
2. `get_node_setting()` in `x-ui.sh` only read from `.other.*` and root config level, missing `.node.*` group (the primary location used by Go's `WriteSettingToJSON`)
3. `install.sh` defaulted remote MariaDB host to `127.0.0.1`, making it easy to accidentally point to localhost with SSH tunnels proxying to the actual master DB

## Changes

### x-ui.sh

- **fix `get_node_setting`**: Added `.node.*` as the first fallback in jq expressions, aligning with Go's `settingGroupAliases` order (`"node"` → `"other"`). Added python3 fallback for proper JSON parsing when jq is unavailable. Fixed grep fallback from `tail -1` (always picks `"other"` group's default due to alphabetical key ordering) to `head -1` (picks `"node"` group).
- **add `is_node_role_configured()`**: New helper function that checks whether `nodeRole` exists in config file (vs. being a default value), checking all three locations: `.node.nodeRole`, `.other.nodeRole`, `.nodeRole`. Supports jq, python3, and grep fallbacks.
- **fix `uninstall()` — worker node guard**: Worker nodes now skip database deletion entirely with a clear message, without even prompting the user. This is the primary fix.
- **fix `uninstall()` — remote host guard**: Remote MariaDB (non-localhost `db_host`) is detected before localhost check and skipped without prompting
- **fix `uninstall()` — unconfigured nodeRole warning**: When `nodeRole` was never explicitly set and `db_host` is localhost, an extra warning is shown before the deletion prompt
- **fix `uninstall()` — secondary confirmation**: When deleting a localhost MariaDB database, a secondary confirmation now shows the exact database name, username, and target address before proceeding
- **cleanup `show_node_status()`**: Removed unnecessary double-quote escaping in default values (was a workaround for old broken grep fallback)

### install.sh

- **fix remote MariaDB default**: Changed `db_host` from defaulting to `127.0.0.1` to requiring explicit user input with a loop that rejects empty values

### tests/test_uninstall_worker_protection.sh (new)

- 9 test cases covering: `.node.*` and `.other.*` config formats, `is_node_role_configured` detection, worker node deletion blocking, remote DB with unset nodeRole, and format precedence

## Decision Matrix (uninstall behavior)

| nodeRole | db_host | Behavior |
|----------|---------|----------|
| worker (explicit) | any | Skip deletion, no prompt |
| any | remote IP | Skip deletion, no prompt |
| master/unset | localhost + unconfigured nodeRole | Warning + prompt + 2nd confirmation |
| master (explicit) | localhost | Prompt + 2nd confirmation |

## Files modified
- `x-ui.sh` — get_node_setting fix, is_node_role_configured, uninstall rewrite
- `install.sh` — remote MariaDB host validation
- `tests/test_uninstall_worker_protection.sh` — new test suite

## Verification
- All 9 bash tests pass
- gofmt returned no changes
