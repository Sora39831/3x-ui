Task Record:

Date: 2026-04-22
Related Module: install script (`install.sh`)
Change Type: Fix

Background

The panel port setup in fresh install used a `y/n` confirmation before reading the port.
When users directly entered a numeric port like `443` at the confirmation prompt, it was treated as non-`y` and the script generated a random port, causing unexpected behavior.

Changes

Replaced the two-step `y/n` + port input flow with a single direct input flow in `install.sh`.
Now the script asks for panel port directly:
- Empty input: generate random panel port.
- Valid numeric port (`1-65535`): use as panel port.
- Invalid input: show error and prompt again.

Added `tests/panel_port_prompt_test.sh` to verify the expected prompt and validation logic exists and the legacy `y/n` prompt is removed.

Impact

Affected files:
- `install.sh`
- `tests/panel_port_prompt_test.sh`

No API, database schema, or build pipeline changes.
Runtime install interaction is changed for fresh install panel-port setup only.

Verification

Commands:
- `bash tests/panel_port_prompt_test.sh`
- `bash tests/mariadb_install_switch_test.sh`

Result:
- Both scripts passed.

Risks And Follow-Up

Current verification is static prompt/logic assertion, not full interactive E2E install simulation.
If needed, add an automated pty-based interaction test to validate runtime behavior across shells.
