# MariaDB Remote IP Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add local MariaDB port customization plus x-ui.sh menu actions to manage remote access by explicit allowed IPs.

**Architecture:** Keep the existing MariaDB install/switch flow, but add reusable shell helpers to update MariaDB server network settings and per-IP grants. Store no new backend state; read active DB settings from `/etc/x-ui/x-ui.json` and query MariaDB for remote hosts when needed.

**Tech Stack:** Bash, MariaDB CLI, existing `install.sh`/`x-ui.sh`, shell prompt assertions in `tests/mariadb_install_switch_test.sh`

---

### Task 1: Extend prompt coverage first

**Files:**
- Modify: `tests/mariadb_install_switch_test.sh`

- [ ] Add assertions for the new local MariaDB port prompt and remote access menu labels.
- [ ] Run `bash tests/mariadb_install_switch_test.sh` and confirm it fails before implementation.

### Task 2: Add MariaDB network config helpers

**Files:**
- Modify: `install.sh`
- Modify: `x-ui.sh`

- [ ] Add shared shell helpers to detect a MariaDB server config file, update `port`, update `bind-address`, and restart MariaDB.
- [ ] Keep defaults local-only (`127.0.0.1`) unless remote access is explicitly enabled.

### Task 3: Support local MariaDB custom port

**Files:**
- Modify: `install.sh`
- Modify: `x-ui.sh`

- [ ] Prompt for local MariaDB port in local install / switch flows.
- [ ] Validate `1-65535`.
- [ ] Apply the chosen server port before business DB/user creation.
- [ ] Persist the selected port through the existing `x-ui setting -dbPort`.

### Task 4: Add remote IP allowlist management

**Files:**
- Modify: `x-ui.sh`

- [ ] Add menu actions to view status, enable remote access, disable remote access, list allowed IPs, add allowed IP, and remove allowed IP.
- [ ] Query MariaDB for existing non-local hosts of the current DB user.
- [ ] Add and remove per-IP grants for the current DB user against the current DB name.
- [ ] Enable remote access by switching bind address to `0.0.0.0`; disable by restoring `127.0.0.1` and removing remote grants.

### Task 5: Verify and record

**Files:**
- Modify: `tests/mariadb_install_switch_test.sh`
- Create: `docs/Tasktracking/2026-04-22-add-mariadb-remote-ip-access.md`

- [ ] Run `bash tests/mariadb_install_switch_test.sh`.
- [ ] Run `bash -n install.sh`.
- [ ] Run `bash -n x-ui.sh`.
- [ ] Write the Tasktracking record with exact verification and residual runtime risks.
