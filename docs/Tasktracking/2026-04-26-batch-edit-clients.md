Task Record: Batch Edit Clients

Date: 2026-04-26
Related Module: Client management (web/service, web/controller, web/html)
Change Type: Add

Background

Users previously had to edit each client individually to change shared settings (flow, limit IP, total GB, expiry time, enable state, Telegram ID, comment, reset period). This was time-consuming when managing many clients in an inbound.

Changes

Added batch multi-select and batch editing for clients in the inbound expanded view:

Backend:
- New `BatchUpdateInboundClients` service method in `web/service/inbound.go` - updates multiple clients' common fields in one transaction, syncs client_traffics table (total, expiry_time, enable, reset, tg_id) and Xray API (enable/disable)
- New `BatchUpdateInboundClientsForUser` authorization wrapper  
- New `POST /panel/api/inbounds/batchUpdateClients` API route in `web/controller/inbound.go`
- Added `toInt64` helper for JSON number type conversion

Frontend:
- New `client_batch_edit_modal.html` modal for batch editing common fields (enable, security, flow, limitIP, totalGB, expiryTime, delayedStart, reset, tgId, comment) with "keep unchanged" defaults
- Modified `inbounds.html` expanded row: added row-selection to client table, batch action bar (batch edit, enable/disable, reset traffic, delete, deselect)
- Added `clientSelection` reactive state and helper methods (`getClientRowKey`, `getClientRowSelection`, `getSelectedClients`, `openBatchEditClient`, `batchEnableClient`, `batchResetClientTraffic`, `batchDelClient`, `clearClientSelection`)

Translations:
- Added keys to `translate.en_US.toml` and `translate.zh_CN.toml`: batchEdit, batchEditAlert, batchKeep, batchEditNoFields, batchDeselect, batchDeleteLastClient, selected

Fields explicitly NOT batch-editable (enforced both frontend and backend):
- email (unique identifier for traffic tracking)
- id (protocol-specific client UUID)
- subId (subscription identifier)
- password (Trojan client password)

Impact

Affected files:
- `web/service/inbound.go` (+~210 lines)
- `web/controller/inbound.go` (+~30 lines)
- `web/html/inbounds.html` (~+90 lines in methods, +15 lines in template)
- `web/html/modals/client_batch_edit_modal.html` (new, 209 lines)
- `web/translation/translate.en_US.toml` (+8 keys)
- `web/translation/translate.zh_CN.toml` (+8 keys)

No database schema changes. No config changes. No breaking API changes (new route only).

Verification

- `go build ./...` - passes
- `CGO_ENABLED=1 go build` - passes
- `go vet ./...` - passes
- `gofmt -d` - no formatting issues

Not verified: runtime integration testing (requires running panel with Xray and clients).

Risks And Follow-Up

- Batch delete does not prevent deleting the LAST client (only warns when trying to delete ALL remaining). The individual delete button already shows/hides based on `isRemovable()`.
- If the Xray API call fails during batch enable/disable, the service returns `needRestart=true` which triggers Xray restart by the periodic check.
- The batch edit modal resets all field values when reopened; no persistence across invocations.
