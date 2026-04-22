Task Record

Date: 2026-04-22
Related Module: install.sh, x-ui.sh, MariaDB runtime configuration
Change Type: Fix

Background

The existing MariaDB setup flow only supported local-only business accounts and default port handling. It did not provide a script path to change the local MariaDB server port or to explicitly authorize selected remote VPS IPs for worker-style access after installation.

Changes

Added reusable shell helpers in `install.sh` and `x-ui.sh` to manage local MariaDB server network settings through an override config file, including `port` and `bind-address`.
Updated the local MariaDB install/switch flow to prompt for `本地 MariaDB port [3306]`, validate the input, apply the port to the local MariaDB server, and keep default local-only binding (`127.0.0.1`).
Extended the `x-ui.sh` database management menu with local MariaDB runtime actions:
- set local MariaDB port
- view MariaDB remote access status
- enable MariaDB remote access
- disable MariaDB remote access
- view allowed remote IPs
- add allowed remote IP
- remove allowed remote IP

Remote access management now uses MariaDB per-host grants for the current business user and current business database. Enabling remote access switches MariaDB bind address to `0.0.0.0` and requires at least one authorized remote IP. Disabling remote access restores `127.0.0.1` and removes non-local grants for the current business user.

Impact

Affected files:
- `install.sh`
- `x-ui.sh`
- `tests/mariadb_install_switch_test.sh`
- `docs/Tasktracking/2026-04-22-add-mariadb-remote-ip-access.md`

This changes shell installer behavior, runtime shell menu behavior, and local MariaDB server configuration on hosts that use the new flow.
No Go API, database schema, or frontend behavior was changed.
Worker-side remote database host and port configuration continues to use the existing `dbHost` and `dbPort` settings flow.

Verification

Commands:
- `bash tests/mariadb_install_switch_test.sh`
- `bash -n install.sh`
- `bash -n x-ui.sh`

Result:
- All commands completed successfully.

Not verified:
- No live MariaDB install/runtime session was executed in this environment.
- No end-to-end validation against an actual remote worker VPS IP was executed in this session.

Risks And Follow-Up

The MariaDB config override path is selected from common distro include directories. On an uncommon MariaDB packaging layout, manual adjustment may still be required.
This implementation restricts remote access by MariaDB host grants, not by firewall source filtering. Unauthorised source IPs should be rejected by MariaDB authentication, but the database service still listens on the configured port while remote access is enabled.
If stricter network-layer isolation is required later, a follow-up can add optional per-IP firewall rules on top of the current MariaDB host-grant model.
