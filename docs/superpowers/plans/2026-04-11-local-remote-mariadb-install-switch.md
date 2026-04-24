# Local/Remote MariaDB Install and Switch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add consistent local-vs-remote MariaDB handling to first install and runtime database switching, with local setup ensuring the business database and business user exist before x-ui saves business credentials.

**Architecture:** Extend the shell scripts with shared helper patterns for local detection, admin credential acquisition, idempotent database/user provisioning, and business-credential validation. Keep remote MariaDB as a pure validation-and-save path, while local MariaDB performs install/start/provision/validate before saving config and migrating data.

**Tech Stack:** Bash, MariaDB CLI, `bash -n`, shell regression tests, Go tests

---

## File Map

**Modify**

- `install.sh`
- `x-ui.sh`

**Create**

- `tests/mariadb_install_switch_test.sh`

**Reference**

- `docs/superpowers/specs/2026-04-11-local-remote-mariadb-install-design.md`

### Task 1: Add failing regression checks

**Files:**

- Create: `tests/mariadb_install_switch_test.sh`

- [ ] Add assertions that `install.sh` prompts for local vs remote MariaDB and handles local business DB fields.
- [ ] Add assertions that `x-ui.sh` exposes the same local vs remote MariaDB prompt and helper flow.
- [ ] Run `bash tests/mariadb_install_switch_test.sh` and verify it fails before script changes.

### Task 2: Implement install-time local/remote MariaDB flow

**Files:**

- Modify: `install.sh`

- [ ] Add helper functions for local-host detection, admin connection attempts, identifier validation, provisioning SQL, and business-connection validation.
- [ ] Update fresh-install MariaDB prompts to choose local or remote before collecting credentials.
- [ ] For remote, validate direct business connection to the target database and save settings.
- [ ] For local, collect business db name/user/password first, provision MariaDB resources, validate the business connection, and save settings.

### Task 3: Implement switch-time local/remote MariaDB flow

**Files:**

- Modify: `x-ui.sh`

- [ ] Reuse or mirror the same prompt structure and provisioning logic in `db_switch_to_mariadb`.
- [ ] Keep remote switch as validate-and-migrate only.
- [ ] Keep local switch as install/start/provision/validate/migrate.

### Task 4: Verify scripts

**Files:**

- Modify: `install.sh`
- Modify: `x-ui.sh`
- Test: `tests/mariadb_install_switch_test.sh`

- [ ] Run `bash tests/mariadb_install_switch_test.sh`.
- [ ] Run `bash -n install.sh x-ui.sh`.
- [ ] Run `go test ./...`.
