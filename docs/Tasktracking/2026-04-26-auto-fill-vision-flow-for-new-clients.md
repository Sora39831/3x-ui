Task Record

Date: 2026-04-26
Related Module: web/service inbound client management
Change Type: Fix

Background

When adding new clients under VLESS + TCP + (TLS/Reality), `flow` may be left empty.
This causes missing expected default flow behavior and inconsistent client configuration.
Requirement is to auto-fill `flow` with `xtls-rprx-vision` only when the client context requires flow.

Changes

Added backend auto-fill logic for new clients:
- Introduced `shouldAutoFillVisionFlow(...)` to detect flow-required context:
  - protocol is `vless`
  - stream `network` is `tcp`
  - stream `security` is `tls` or `reality`
- Introduced `autoFillVisionFlowInSettings(...)` to fill empty/missing client `flow` as `xtls-rprx-vision`.

Integrated into add/update flows:
- `AddInbound`: auto-fill for initial clients of a newly created inbound.
- `AddInboundClient`: auto-fill for clients added via add-client endpoint (includes bulk add and TG bot path).
- `UpdateInbound`: auto-fill only for newly added VLESS clients (does not override existing clients).

Added tests:
- `web/service/inbound_flow_autofill_test.go`
  - all eligible clients are auto-filled
  - selected new clients only can be targeted
  - no change when flow is not required

Impact

Affected modules or files.
- `web/service/inbound.go`
- `web/service/inbound_flow_autofill_test.go`

Whether APIs, database, config, build, or compatibility are affected.
- API endpoints unchanged.
- Database schema unchanged.
- Runtime behavior change: new clients in eligible VLESS context auto-receive `xtls-rprx-vision` flow.

Whether upstream or downstream callers are affected.
- UI add-client, bulk add-client, and TG bot add-client now share consistent default flow behavior via backend logic.

Verification

List validation commands or checks performed.
- `go test ./web/service/...`

State the result.
- Passed.

If not verified, explain why.
- No remote runtime deployment test was performed in local environment.

Risks And Follow-Up

Remaining risks.
- If operators expect empty flow for newly added VLESS+TCP+TLS/Reality clients, behavior is now intentionally changed.

Recommended follow-up work.
- If needed, expose a setting switch to opt out of default flow auto-fill.
