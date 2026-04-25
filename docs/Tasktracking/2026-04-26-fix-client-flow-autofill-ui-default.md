Task Record

Date: 2026-04-26
Related Module: client add modals (web/html)
Change Type: Fix

Background

Although backend flow auto-fill was implemented for eligible new clients, the add-client UI still initialized `flow` as empty by default.
This made the feature appear non-functional during client creation.

Changes

Updated UI defaults for new VLESS clients in add dialogs:
- `web/html/modals/client_modal.html`
  - In single add flow, when inbound `canEnableTlsFlow()` is true and client flow is empty, default to `xtls-rprx-vision`.
- `web/html/modals/client_bulk_modal.html`
  - On modal show, default selected bulk `flow` to `xtls-rprx-vision` when `canEnableTlsFlow()` is true.
  - In `newClient(...)` for VLESS, initialize empty flow to `xtls-rprx-vision` under the same condition.

Impact

Affected modules or files.
- `web/html/modals/client_modal.html`
- `web/html/modals/client_bulk_modal.html`

Whether APIs, database, config, build, or compatibility are affected.
- No API contract changes.
- No database schema changes.
- UI behavior change only for default values in eligible flow-required scenarios.

Whether upstream or downstream callers are affected.
- Panel operators adding clients now immediately see expected default flow value.

Verification

List validation commands or checks performed.
- `go test ./web/...`

State the result.
- Passed.

If not verified, explain why.
- No remote runtime interaction was performed in this local environment.

Risks And Follow-Up

Remaining risks.
- Existing browser cache may keep older template assets until refresh/reload.

Recommended follow-up work.
- Verify add-client and bulk-add flows in panel UI for VLESS+TCP+TLS/Reality in deployed environment.
