# Multi-Node Shared Control Design

## Context

`3x-ui` already supports MariaDB as a database backend, but the runtime model remains single-node in practice: the panel reads persisted data, generates a local Xray configuration, and starts a local Xray process. Xray itself does not read MariaDB directly.

The target capability is a minimal multi-node control model where multiple VPS nodes share account definitions and aggregate traffic through MariaDB, while each node still runs its own local Xray instance.

This design intentionally keeps the existing panel pattern intact. It adds synchronization and role boundaries around the current database and Xray services instead of redesigning the runtime into a distributed cluster.

## Goals

- Support multiple nodes sharing account definitions through MariaDB.
- Keep `master` as the only writer for shared account definitions.
- Keep `worker` nodes running local Xray instances built from synchronized shared data.
- Aggregate traffic from all nodes without last-write-wins corruption.
- Preserve survivability when MariaDB is temporarily unavailable by using local cache files.
- Minimize changes to the existing `3x-ui` architecture and operator workflow.

## Non-Goals

- No direct MariaDB access from Xray.
- No leader election or automatic role failover.
- No active-active shared account writes across nodes.
- No push-based real-time config propagation.
- No synchronization of panel-global settings such as panel port, web domain, TLS files, Telegram bot settings, or panel users.
- No first-pass frontend-only read-only redesign for worker nodes.

## Accepted Scope

The first phase only synchronizes the shared-account surface:

- inbounds and inbound client definitions
- passwords and credential-bearing settings
- quota, expiry, enabled/disabled state
- aggregate upload and download counters

The first phase does not synchronize node-local operational settings or observability data beyond minimal sync status metadata.

## Recommended Approach

Use a single-writer control model:

- `master` is the only node allowed to mutate shared account definitions.
- `worker` nodes keep the existing panel pages, but shared-account write requests are rejected in the backend.
- all nodes, including `master`, continue to run local Xray instances and carry traffic
- workers poll MariaDB for a shared account-set version and rebuild their local Xray state when that version changes
- all nodes accumulate local traffic deltas and periodically flush them back to MariaDB using atomic increments

This is the lowest-risk option because it keeps the current `controller -> service -> database -> local Xray` model intact while adding explicit control-plane boundaries.

## Architecture

### Runtime Model

The system remains node-local at runtime:

- each node runs its own `3x-ui` process
- each node generates its own local Xray configuration
- each node starts and restarts its own local Xray process
- MariaDB acts only as the shared control database

There is no direct node-to-node RPC. Synchronization happens indirectly through MariaDB and local background loops.

### Role Boundaries

`master` responsibilities:

- accept and apply shared-account writes
- bump the shared account-set version after successful writes
- rebuild local Xray state immediately after successful writes
- flush local traffic deltas like any other node

`worker` responsibilities:

- reject shared-account write requests in the backend
- load synchronized shared account snapshots
- rebuild local Xray state from the synchronized snapshot
- flush local traffic deltas

### Shared vs Local Data

Shared through MariaDB:

- shared inbound definitions
- shared client definitions
- shared quota and expiry state
- aggregate traffic totals
- synchronization metadata

Kept local to each node:

- panel port and path
- web domain and TLS files
- Telegram bot settings
- panel users and login state
- local cache and pending traffic files
- local logs and node-specific operational state

## Configuration Model

Store node-control settings in `/etc/x-ui/x-ui.json`:

- `nodeRole`: `master` or `worker`, default `master`
- `nodeId`: unique node identifier, required for `worker`
- `syncInterval`: shared snapshot poll interval in seconds, default `30`
- `trafficFlushInterval`: traffic delta flush interval in seconds, default `10`

Validation rules:

- `nodeRole` must be `master` or `worker`
- `worker` requires non-empty `nodeId`
- `worker` requires `dbType = mariadb`
- `syncInterval` must be positive
- `trafficFlushInterval` must be positive

Compatibility rules:

- legacy configs without these keys must continue to load with defaults
- existing grouped JSON layout rules for other settings must remain compatible
- switching roles is a config change plus restart, not a schema migration

## Data Model

The first phase keeps shared business data in the existing tables and adds minimal synchronization metadata.

### Shared Metadata Tables

`shared_state`

- `key` primary key
- `version` monotonic int64
- `updated_at`

Reserved key:

- `shared_accounts_version`

`node_state`

- `node_id` primary key
- `node_role`
- `last_sync_at`
- `last_heartbeat_at`
- `last_seen_version`
- `last_error`
- `updated_at`

### Local Persistent Files

`/etc/x-ui/shared-cache.json`

- stores the last valid shared account snapshot
- primarily used by workers for startup and MariaDB outage survivability
- is not the system of record

`/etc/x-ui/traffic-pending.json`

- stores traffic deltas that have not yet been flushed successfully
- is used by all nodes
- prevents delta loss across retries or restarts

### Versioning Strategy

The design uses a single account-set version instead of per-record versions.

Rule:

- whenever `master` successfully changes shared account definitions, it increments `shared_accounts_version` in the same transaction

This version answers only one question:

- has the shared account set changed since the node last synchronized

That keeps worker polling cheap and avoids per-row merge logic in the first phase.

## Synchronization and Write Flow

### Shared Account Write Flow

For shared-account writes such as add, update, delete, enable, disable, quota change, expiry change, or client mutation:

1. request enters the existing controller/service path
2. service layer enforces `RequireMaster()`
3. `worker` requests are rejected before any database or Xray mutation
4. `master` applies the write
5. the same transaction increments `shared_accounts_version`
6. after transaction commit, `master` rebuilds local Xray configuration immediately

The `master` node does not wait for polling to apply its own writes.

### Worker Snapshot Sync Flow

Workers synchronize on startup and on a fixed interval:

1. load `shared-cache.json` if present
2. perform an immediate version check against MariaDB
3. if the shared version is newer, fetch the full shared account snapshot
4. persist the snapshot to `shared-cache.json`
5. rebuild local Xray configuration from that snapshot
6. update `node_state` with sync status

During steady state:

- poll `shared_accounts_version` every `syncInterval`
- if unchanged, only update node heartbeat/sync status
- if changed, fetch the full snapshot and rebuild local Xray state

### Traffic Flush Flow

All nodes, including `master`, follow the same traffic accounting pattern:

1. accumulate local per-account upload and download deltas
2. persist pending deltas locally
3. flush pending deltas every `trafficFlushInterval`
4. apply deltas in MariaDB using atomic increments
5. remove flushed deltas from local pending state after success

Required rule:

- nodes must never write absolute aggregate totals back to MariaDB

Forbidden behavior:

- reading the current total, adding locally, then overwriting the row
- allowing concurrent nodes to race with last-write-wins semantics

## Trigger Timing

Synchronization is triggered by control-state changes and fixed intervals, not by direct node-to-node notifications.

Triggers:

- `master` shared-account write success: bump version in-transaction and rebuild local Xray immediately
- worker startup: load local cache, then perform an immediate version check
- worker periodic sync: poll version every `syncInterval`
- all nodes periodic traffic flush: flush deltas every `trafficFlushInterval`
- optional best-effort shutdown flush: attempt one final delta flush on graceful shutdown

There is no direct push from `master` to `worker` in the first phase.

## Failure Handling

If MariaDB is temporarily unreachable:

- workers continue using the last valid shared snapshot
- traffic deltas remain in local pending storage
- shared-account writes cannot proceed

If `shared-cache.json` is missing or invalid and MariaDB is unreachable at worker startup:

- do not construct a speculative snapshot
- preserve the last successfully loaded runtime state if one already exists
- record the error in node status

If snapshot fetch succeeds but local Xray rebuild fails:

- keep the last known-good local runtime configuration active
- record the failure in `node_state.last_error`
- retry on the next synchronization cycle

If `master` writes shared state successfully but its own local rebuild fails:

- MariaDB remains the source of truth with the new version
- other nodes still synchronize from that committed state
- `master` records the local rebuild error for operator action

If a traffic flush fails:

- keep the delta in pending storage
- retry later
- never drop pending deltas on flush failure

## Testing Strategy

### Config Tests

Cover:

- legacy config defaults
- invalid `nodeRole`
- `worker` without `nodeId`
- `worker` with `sqlite`
- non-positive interval validation

### Service Tests

Cover:

- `RequireMaster()` enforcement
- version bumping on successful shared-account writes
- worker no-op behavior when version is unchanged
- worker snapshot fetch and rebuild trigger when version changes
- traffic flush success and retry behavior

### Database Tests

Cover:

- migration of `shared_state` and `node_state`
- seeding of `shared_accounts_version`
- atomic increment semantics for traffic totals

### Verification Expectations

Phase-one acceptance:

1. `master` writes are visible to workers within one poll cycle
2. worker-side shared-account writes are rejected
3. concurrent node traffic flushes do not lose aggregate totals
4. workers continue operating from the last valid cache during temporary MariaDB outages

## Operational Visibility

Expose minimal operator-visible node information:

- `nodeRole`
- `nodeId`
- `syncInterval`
- `trafficFlushInterval`

Record minimal per-node status in `node_state`:

- last sync time
- last heartbeat time
- last seen version
- last error

This is sufficient for the first phase to answer:

- whether a worker is lagging behind the current shared version
- whether a node is alive but failing to synchronize
- whether traffic flush retries are accumulating due to database problems

## Implementation Boundary

Expected change areas:

- config loading and validation
- startup validation and CLI setters
- database model registration and metadata helpers
- service-layer master-only guards
- worker sync and cache services
- traffic pending and flush services
- startup wiring in the web server
- operator-facing shell scripts and docs

Areas intentionally unchanged in the first phase:

- Xray protocol implementation
- local Xray process ownership model
- panel-global settings model
- distributed leadership or push-based synchronization

## Summary

This design adds a minimal shared control plane to `3x-ui` without turning the system into a distributed cluster. `master` owns shared account writes, all nodes keep local Xray ownership, workers rebuild from synchronized snapshots, and traffic returns to MariaDB only as atomic deltas. The result is a bounded first phase that matches the existing architecture and keeps future extension paths open.
