Task Record:

Date: 2026-04-22
Related Module: install/uninstall scripts (`install.sh`, `x-ui.sh`)
Change Type: Fix

Background

Installation could get stuck or fail at `x-ui migrate` before service installation completed.
When this happened, service files were not installed, but files under `/usr/local/x-ui` or `/etc/x-ui` could already exist.
Then `x-ui.sh uninstall` might reject uninstall with "please install first" because installation detection relied only on service file presence.

Changes

In `install.sh`, changed migration call to non-blocking behavior:
- Use `timeout 30` when available for `${xui_folder}/x-ui migrate`.
- If migration times out or fails, print warning and continue installation.
- Keep manual migration command hint for follow-up.

In `x-ui.sh`, improved install detection in `check_status`:
- If service/init file is missing but residual install artifacts exist (`${xui_folder}/x-ui`, `${xui_folder}`, or `/etc/x-ui`), treat it as installed-but-not-running instead of not installed.
- This allows `x-ui uninstall` to proceed and clean residual files.

Added `tests/install_uninstall_resilience_test.sh` for static regression checks of the new logic.

Impact

Affected files:
- `install.sh`
- `x-ui.sh`
- `tests/install_uninstall_resilience_test.sh`

No API or database schema changes.
Installer runtime behavior is more resilient when migration has connectivity issues.
Uninstall command now works for partial/failed installation residue.

Verification

Commands:
- `bash -n install.sh`
- `bash -n x-ui.sh`
- `bash tests/install_uninstall_resilience_test.sh`
- `bash tests/panel_port_prompt_test.sh`
- `bash tests/mariadb_install_switch_test.sh`

Result:
- All commands passed.

Risks And Follow-Up

Migration failure is now non-blocking, so some environments may finish install while still requiring manual migration.
For complete runtime coverage, a pty-driven install E2E test with unreachable MariaDB simulation can be added later.
