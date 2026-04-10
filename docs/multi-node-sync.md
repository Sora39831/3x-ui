# Multi-Node Shared Control

## Roles

- `master`: the only node allowed to change shared account definitions
- `worker`: rebuilds local Xray config from shared snapshots and flushes traffic deltas

## Requirements

- shared mode requires MariaDB
- each worker needs a unique `nodeId`
- workers keep `/etc/x-ui/shared-cache.json` for outage survival

## Runtime Loops

- workers poll `shared_accounts_version` every `syncInterval`
- all nodes flush `/etc/x-ui/traffic-pending.json` every `trafficFlushInterval`
- only `master` runs shared traffic reconciliation that can disable or renew clients

## Manual Verification

1. Start a `master` node on MariaDB.
2. Start a `worker` node on the same MariaDB with a unique `nodeId`.
3. Change an inbound or client on `master`.
4. Confirm the worker sees a newer `shared_accounts_version` and rebuilds local Xray.
5. Generate traffic on both nodes.
6. Confirm aggregated MariaDB counters increase without overwriting each other.
7. Stop MariaDB briefly and confirm the worker continues using `shared-cache.json`.
8. Restore MariaDB and confirm pending traffic deltas flush successfully.
