# Multi-Node Shared Control Progress

## Execution Strategy

- Tasks 1–3: Inline
- Tasks 4–7: Subagent-Driven
- Every task is a checkpoint and must end with its own git commit.
- After each task commit, record the short hash here before moving on.

## Update Rules

- Change `Status: [ ] Pending` to `Status: [x] Done` only after the task’s checkpoint commit succeeds.
- Replace `Commit: pending` with the short commit hash for that task.
- If a task is blocked, update the `Blockers:` line on that task instead of opening a separate scratch section.
- Keep this file current before handing work from Inline execution to Subagent-Driven execution.

## Task Tracker

### Task 1: Add node config, runtime file paths, and startup validation

- Status: [x] Done
- Mode: Inline
- Commit: 36826706
- Depends on: none
- Scope: add `NodeConfig`, role validation, runtime file path helpers, CLI setters, and startup validation before DB init
- Primary files: `config/config.go`, `config/config_test.go`, `main.go`
- Checkpoints:
  - add focused failing tests for defaults, validation, and runtime file paths
  - implement `GetNodeConfigFromJSON`, `ValidateNodeConfig`, `GetSharedCachePath`, and `GetTrafficPendingPath`
  - wire startup validation and `setting` CLI flags into `main.go`
  - run focused `./config` tests and package discovery
- Done when:
  - worker mode rejects missing `nodeId` and `sqlite`
  - `setting -show` prints node settings
  - Task 1 checkpoint commit is recorded
- Blockers: none

### Task 2: Add shared metadata models and repository helpers

- Status: [x] Done
- Mode: Inline
- Commit: fd0af148
- Depends on: Task 1 committed
- Scope: add shared version and node heartbeat metadata tables plus repository helpers for version bump and node-state upsert
- Primary files: `database/model/shared_state.go`, `database/model/node_state.go`, `database/shared_state.go`, `database/db.go`, `database/db_test.go`
- Checkpoints:
  - add failing tests for metadata tables, version seeding, and node-state upsert
  - create `SharedState` and `NodeState` models
  - add `GetSharedAccountsVersion`, `BumpSharedAccountsVersion`, and `UpsertNodeState`
  - register models in DB init and seed `shared_accounts_version`
  - run focused `./database` tests
- Done when:
  - metadata tables are auto-migrated by `InitDB`
  - shared version starts at `0` and bumps transactionally
  - Task 2 checkpoint commit is recorded
- Blockers: none

### Task 3: Enforce master-only shared writes and transactional version bumping

- Status: [x] Done
- Mode: Inline
- Commit: `34b9f01d`
- Depends on: Task 2 committed
- Scope: establish the shared write boundary in service code before any worker sync or shared traffic work lands
- Primary files: `web/service/node_guard.go`, `web/service/node_guard_test.go`, `web/service/inbound.go`
- Checkpoints:
  - [x] add failing tests for `RequireMaster` and version rollback behavior
  - [x] implement `IsWorker`, `IsMaster`, `RequireMaster`, and shared-mode detection
  - [x] guard shared inbound/client mutation paths in `InboundService`
  - [x] bump shared version only inside successful write transactions
  - [x] run focused `./web/service` guard tests
- Done when:
  - worker-side shared write attempts fail in service layer
  - successful shared writes advance `shared_accounts_version`
  - Task 3 checkpoint commit is recorded
- Blockers: none

### Task 4: Add shared snapshot cache, worker sync loop, and snapshot-driven Xray rebuild

- Status: [x] Done
- Mode: Subagent-Driven
- Commit: `3cfa5547`
- Depends on: Tasks 1–3 committed
- Scope: make workers survive from cached shared snapshots and rebuild local Xray from synchronized inbounds instead of live DB reads only
- Primary files: `web/service/node_cache.go`, `web/service/node_sync.go`, `web/service/node_sync_test.go`, `web/service/xray.go`, `web/web.go`
- Checkpoints:
  - [x] add failing tests for snapshot round-trip, no-op sync, changed-version sync, and cache bootstrap
  - [x] add `SharedAccountsSnapshot` load/save helpers
  - [x] refactor `XrayService` to build config from an explicit inbound list
  - [x] implement `NodeSyncService` with cache bootstrap, version polling, and node-state updates
  - [x] start worker sync or master heartbeat loops from server startup
  - [x] run focused sync tests
- Done when:
  - worker startup can apply `shared-cache.json` before the first DB poll
  - changed shared version leads to cache refresh plus local Xray rebuild
  - Task 4 checkpoint commit is recorded
- Blockers: none

### Task 5: Add durable traffic delta persistence and safe shared-mode flushes

- Status: [x] Done
- Mode: Subagent-Driven
- Commit: `87282dde`
- Depends on: Task 3 committed, Task 4 wiring available
- Scope: stop worker shared-mode traffic from mutating shared state directly; collect local deltas, flush atomically, and keep reconciliation master-only
- Primary files: `web/service/traffic_pending.go`, `web/service/traffic_flush.go`, `web/service/traffic_flush_test.go`, `web/job/xray_traffic_job.go`, `web/service/inbound.go`, `web/web.go`, `xray/client_traffic.go`
- Checkpoints:
  - [x] add failing tests for delta merge, successful flush, and failed flush retention
  - [x] implement `TrafficPendingStore` and `TrafficDelta`
  - [x] add composite unique key on `(inbound_id, email)` for safe upserts
  - [x] implement `TrafficFlushService` with atomic DB updates
  - [x] extract `ReconcileSharedTrafficState` so only master performs enable/expiry mutations
  - [x] route shared-mode traffic collection through pending-store accumulation and start flush loop
  - [x] run focused flush tests and package discovery
- Done when:
  - worker shared-mode traffic no longer calls `InboundService.AddTraffic()`
  - pending deltas survive failures and clear only after successful flush
  - Task 5 checkpoint commit is recorded
- Blockers: none

### Task 6: Expose node management in shell tools and the installer

- Status: [x] Done
- Mode: Subagent-Driven
- Commit: `6b58044e`
- Depends on: Task 1 committed
- Scope: expose the new node settings to operators through `x-ui.sh` and `install.sh` without destabilizing upgrades
- Primary files: `x-ui.sh`, `install.sh`
- Checkpoints:
  - [x] add node-setting readers and status display to `x-ui.sh`
  - [x] add minimal node-role and node-id mutation actions via `./x-ui setting`
  - [x] prompt for MariaDB and node role during fresh installs only
  - [x] run `bash -n` on both scripts and smoke-check `./x-ui setting -show true`
- Done when:
  - shell scripts pass syntax checks
  - fresh-install path can capture MariaDB plus node role settings
  - Task 6 checkpoint commit is recorded
- Blockers: none

### Task 7: Document the feature and run focused verification

- Status: [x] Done
- Mode: Subagent-Driven
- Commit: `85cd0b60`
- Depends on: Tasks 4–6 committed
- Scope: publish operator-facing docs, verification steps, and final package-level checks after implementation is stable
- Primary files: `docs/multi-node-sync.md`, `README.md`, `README.zh_CN.md`
- Checkpoints:
  - [x] write the operator runbook with role model, requirements, runtime loops, and failure behavior
  - [x] add concise README sections in English and Chinese
  - [x] append the manual verification checklist
  - [x] run focused verification across `./config`, `./database`, and `./web/service`, then package discovery
- Done when:
  - docs match the shipped runtime behavior
  - verification commands pass
  - Task 7 checkpoint commit is recorded
- Blockers: none
