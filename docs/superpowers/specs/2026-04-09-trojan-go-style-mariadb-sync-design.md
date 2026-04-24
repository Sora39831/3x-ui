# Trojan-Go Style Multi-VPS MariaDB Sync Design

## Context

The current project already supports MariaDB as a database backend, but the runtime model is still single-node in practice: the panel reads database state, generates a local Xray configuration, and starts a local Xray process. Xray itself does not read MariaDB directly.

The target deployment is multiple VPS instances sharing the same MariaDB-backed account system with the following data:

- account names
- passwords
- traffic quotas
- traffic usage counters

The goal is to keep the change set small and to reuse the existing MariaDB backend, while moving toward a `trojan-go`-style pattern where nodes periodically pull account state and maintain local runtime caches.

This design intentionally does not turn the system into a distributed cluster. It is a shared control-data model with one writable node and multiple read-oriented nodes.

## Goals

- Keep MariaDB as the single source of truth.
- Share account, password, quota, and traffic state across multiple VPS instances.
- Minimize code changes and preserve the current Xray startup model.
- Prevent multiple nodes from concurrently mutating account definitions.
- Support local runtime caches so each VPS can continue operating if the control node is temporarily unavailable.

## Non-Goals

- No direct database access from Xray.
- No distributed consensus or leader election.
- No automatic active-active write coordination.
- No node-aware load balancing or request routing layer.
- No redesign of the existing panel UI or the MariaDB migration logic.

## Recommended Approach

Use a two-role model:

- `writer/master` node: the only node allowed to write shared account state.
- `reader/worker` nodes: read shared account state, generate local Xray configuration, and report traffic deltas.

This is the lowest-risk option because it keeps the current database-centric flow intact and only adds synchronization boundaries around it.

## Alternatives Considered

### 1. Fully shared read/write MariaDB across all VPS

Pros:

- Lowest immediate implementation effort.
- No role distinction to manage.

Cons:

- Highest risk of conflicting writes.
- Traffic counters can be corrupted by last-write-wins behavior.
- Hard to reason about ownership of account edits.

### 2. Single writer, multiple readers with local caches

Pros:

- Clear ownership model.
- Smallest safe change.
- Compatible with periodic polling and incremental stat updates.

Cons:

- Not real-time.
- Requires one node to be the administrative source of writes.

### 3. Push-based control plane

Pros:

- Lower sync latency.
- Better scalability for many nodes.

Cons:

- Larger change set.
- Needs a push channel, retry logic, and delivery tracking.

## Design

### 1. Role Model

Introduce a node role concept:

- `master`: may create, update, and delete shared account records.
- `worker`: may only read shared account data and submit traffic increments.

The role is a deployment-time setting, not an automated cluster election. This avoids new consensus logic.

### 2. Data Ownership

Split the shared data into three logical groups:

- `account state`: username, password, enabled/disabled, quota, expiry, tags.
- `usage state`: accumulated download/upload totals, quota consumption, last update time.
- `node state`: node identifier, last sync time, last heartbeat, last reported version.

The important rule is that `usage state` must be updated as an increment, not as an overwrite.

### 3. Synchronization Flow

Each worker node follows this loop:

1. Poll MariaDB for account records and version changes.
2. Refresh the local cache when a newer version is detected.
3. Regenerate the local Xray configuration from the cache.
4. Continue serving traffic using the local runtime state.
5. Periodically submit traffic deltas back to MariaDB.

The master node follows the same pull logic, but additionally accepts account edits and propagates new version numbers.

### 4. Versioning

Add a monotonically increasing version marker to the shared account records or to the account set as a whole.

The version marker is used to answer one question only:

- "Has the account state changed since the last sync?"

This avoids full-table diffs and keeps polling cheap.

### 5. Traffic Accounting

Traffic accounting must be incremental.

Required behavior:

- each node accumulates local upload/download totals for the accounts it serves
- the node submits `delta_upload` and `delta_download` on a schedule
- MariaDB applies atomic increments to the canonical counters

Forbidden behavior:

- writing a full absolute total from a worker node
- letting multiple nodes overwrite the same total counter

This rule is what prevents traffic counter loss under concurrent updates.

### 6. Local Runtime Cache

Each VPS should keep a local in-memory cache, and optionally a lightweight on-disk snapshot if that is already aligned with the existing runtime flow.

The cache is the source for:

- Xray configuration generation
- local access checks
- short-lived serving behavior if MariaDB is temporarily unreachable

The cache is not the source of truth.

### 7. Failure Handling

If MariaDB is unreachable:

- worker nodes continue using the last valid cached account state
- traffic deltas remain buffered locally until the database returns
- no attempt is made to write account edits from workers

If the master node is unreachable:

- workers continue serving
- account edits pause
- existing synced data remains usable

If a traffic delta write fails:

- keep the delta in a retry queue
- retry with idempotent semantics if possible

## Minimal Schema Changes

The design assumes the existing MariaDB schema already stores the relevant shared data. Only small additions are needed.

Recommended additions:

- `version` or `updated_at` for account state change detection
- `node_id` for identifying each VPS instance
- `last_sync_at` for sync monitoring
- `pending_upload_delta` and `pending_download_delta` if local buffering is persisted

If the current schema already has equivalent fields, reuse them instead of adding new ones.

## Implementation Boundary

The following areas should change:

- database read/write logic for account sync
- traffic update path to support incremental writes
- configuration generation path to consume local cache
- role enforcement in update operations

The following areas should stay unchanged in the first pass:

- Xray protocol handling
- local Xray process management
- existing MariaDB connection settings and backend initialization
- unrelated panel features

## Operational Model

### Deployment

1. Pick one VPS as `master`.
2. Point every VPS at the same MariaDB instance.
3. Mark all non-master VPS instances as `worker`.
4. Configure workers to poll at a fixed interval.
5. Configure workers to submit traffic deltas on a fixed interval.

### Administration

Only the master node should be used for:

- adding accounts
- changing passwords
- changing quotas
- disabling/enabling shared accounts

Workers should expose read-only behavior for shared account data.

### Node Configuration

Store node metadata in the existing JSON configuration file so the role can be switched without touching the database schema.

Recommended keys:

- `nodeRole`: `master` or `worker`
- `nodeId`: unique node identifier, such as hostname or UUID
- `syncInterval`: account sync interval in seconds
- `trafficFlushInterval`: traffic flush interval in seconds

Operational rules:

- `nodeRole` controls write permission for shared account state
- `nodeId` identifies the node when writing heartbeats or usage deltas
- `master` and `worker` both continue to use the same MariaDB backend
- switching `nodeRole` should be a config update plus restart, not a database migration

### Management Entry Points

The shell entry points should expose the same separation of responsibilities:

- `x-ui.sh`: runtime switching and node-role management
- `install.sh`: first-install selection of node role and database type

`x-ui.sh` should gain a node-management menu that can:

- show the current node role
- switch between `master` and `worker`
- edit `nodeId`
- show last sync and heartbeat information

`install.sh` should prompt for:

- node role
- database type
- MariaDB connection settings when `mariadb` is selected

This keeps post-install operation and initial deployment distinct.

### Default Database Policy

The default database for new installs should be MariaDB, while SQLite remains fully supported for compatibility and downgrade scenarios.

Recommended rules:

- fresh installs default to `mariadb`
- existing SQLite installs remain on SQLite unless the operator explicitly migrates
- SQLite remains a valid fallback for single-node or offline deployments
- MariaDB remains the preferred backend for multi-VPS shared-account deployments

This means `dbType` is still a backend choice, not a role choice. `nodeRole` and `dbType` are independent settings.

### SQLite Compatibility

Keep SQLite support intact in the first pass:

- do not remove SQLite migration paths
- do not require MariaDB for local single-node operation
- preserve existing SQLite behavior for users who never opt into multi-node syncing

The compatibility guarantee is:

- SQLite continues to work as a standalone backend
- MariaDB becomes the default for new deployments
- node-role logic applies on top of either backend, but only the multi-node sync model depends on MariaDB in practice

## Testing Strategy

### Unit Tests

- version change detection
- account cache refresh behavior
- traffic delta accumulation
- atomic counter update semantics

### Integration Tests

- master edits account data and workers observe the update
- multiple workers submit traffic deltas without losing increments
- workers continue with cached data during temporary database outage

### Failure Tests

- stale cache fallback
- retry behavior for failed usage writes
- duplicate delta submission protection if the same buffered record is retried

## Risks

- Polling introduces sync delay.
- Incremental traffic writes must be idempotent or protected against retry duplication.
- Without node isolation, it is still possible to mix usage ownership if deployment discipline is poor.
- This design improves safety, but it is not a substitute for a real distributed control plane.

## Recommended Rollout

1. Add role-based write protection for shared account edits.
2. Add versioned polling for account state.
3. Add incremental traffic writeback.
4. Add buffered retry for failed delta submissions.
5. Only after that, consider pushing updates or adding more advanced node management.

## Open Questions

- Should traffic counters be stored per node and aggregated periodically, or written directly into a shared total counter with atomic increments?
- Should workers be allowed to serve indefinitely from cache, or should a sync-age limit disable stale accounts after a timeout?

## Decisions Captured

- Use JSON configuration for `nodeRole` and `nodeId`
- Add runtime node switching in `x-ui.sh`
- Add first-install role selection in `install.sh`
- Default new installs to MariaDB
- Preserve SQLite compatibility for existing and standalone deployments
