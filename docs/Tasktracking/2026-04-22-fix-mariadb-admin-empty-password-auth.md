Task Record:

Date: 2026-04-22
Related Module: MariaDB admin auth in installer and menu script
Change Type: Fix

Background

During local MariaDB setup, when socket auth failed the script asked for admin credentials.
On hosts where admin/root password is empty, the previous command construction always appended `-p`, which can cause authentication checks to fail for empty-password accounts.
This led to false "管理员账号连接失败" and blocked install/uninstall flows.

Changes

Updated MariaDB connection execution in both `install.sh` and `x-ui.sh`:
- Build MariaDB client command with optional password flag.
- Append `-p<password>` only when password is non-empty.
- Apply this to server-connection check, database-connection check, and admin SQL execution.
- Prompt text updated to indicate admin password can be empty.
- Added retry logic (up to 10 seconds) before declaring socket auth failure.
- Added automatic fallback to `root@127.0.0.1` with empty password before prompting admin credentials.
- For freshly installed local MariaDB in script flow, set an install marker and prefer non-interactive auto init path.

Added `tests/mariadb_admin_empty_password_test.sh` for static regression checks.

Impact

Affected files:
- `install.sh`
- `x-ui.sh`
- `tests/mariadb_admin_empty_password_test.sh`

No API/database schema change.
Install, DB switching, and uninstall paths are more compatible with empty-password admin setups.

Verification

Commands:
- `bash -n install.sh`
- `bash -n x-ui.sh`
- `bash tests/mariadb_admin_empty_password_test.sh`
- `bash tests/install_uninstall_resilience_test.sh`
- `bash tests/panel_port_prompt_test.sh`
- `bash tests/mariadb_install_switch_test.sh`

Result:
- All checks passed.

Risks And Follow-Up

Current tests are static assertions. Full runtime verification still depends on real MariaDB environment variants (socket auth, TCP auth, empty password, root-password mode).
