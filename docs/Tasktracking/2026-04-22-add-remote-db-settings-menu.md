Task Record

Date: 2026-04-22
Related Module: x-ui.sh
Change Type: Fix

Background

The database management menu exposed node role and node ID settings, but it did not provide a standalone way to maintain remote MariaDB connection fields after multi-node setup. Operators had no menu entry to update host, port, username, password, or database name without re-running the database switch flow or editing config files manually.

Changes

Added a reusable database-setting reader in `x-ui.sh` for MariaDB connection values stored in `/etc/x-ui/x-ui.json`.
Added a new database management menu entry to update remote MariaDB host, port, username, password, and database name independently from the database migration flow.
The new flow shows current values, preserves the stored password when the operator leaves the password prompt empty, validates required fields and port range, and tests the MariaDB business connection before saving.
Updated the existing database status output to reuse the new setting reader instead of duplicating JSON parsing logic.

Impact

Affected files:
- `x-ui.sh`
- `docs/Tasktracking/2026-04-22-add-remote-db-settings-menu.md`

Runtime behavior is affected in the shell management menu only.
No API, database schema, or build pipeline changes were made.
The new menu path updates MariaDB connection values in `/etc/x-ui/x-ui.json` through the existing `x-ui setting` command and does not automatically switch the active database type.

Verification

Command:
- `bash -n x-ui.sh`

Result:
- Passed syntax validation with exit code 0.

Not verified:
- Interactive execution of the new menu path against a real remote MariaDB instance was not run in this session.

Risks And Follow-Up

The new flow depends on the MariaDB client being available or installable at runtime, consistent with the existing database switch flow.
Password preservation relies on the currently stored JSON value; if the stored password is stale, the connection test will fail until the operator enters the correct password explicitly.
Recommended follow-up work is a manual end-to-end check of menu option `9` on a host with an actual remote MariaDB target.
