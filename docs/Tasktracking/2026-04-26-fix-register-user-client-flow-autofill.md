Task Record

Date: 2026-04-26
Related Module: web/service user registration client auto-provisioning
Change Type: Fix

Background

New user registration auto-creates clients across inbounds via `addUserClientsToAllInbounds`.
This path bypassed the previously added AddInboundClient flow auto-fill logic, so newly registered users could still get empty `flow` in eligible VLESS contexts.

Changes

Updated registration auto-provisioning path in `web/service/user.go`:
- When target inbound requires flow (`VLESS + TCP + TLS/Reality`), set client `Flow` to `xtls-rprx-vision`.
- Persist `flow` field in generated client entry when populated.

Added test in `web/service/user_test.go`:
- `TestRegisterUser_AutoFillFlowForEligibleVlessInbound`
- Verifies registered user gets `xtls-rprx-vision` flow in eligible VLESS inbound.
- Verifies non-VLESS inbound does not get forced flow.

Impact

Affected modules or files.
- `web/service/user.go`
- `web/service/user_test.go`

Whether APIs, database, config, build, or compatibility are affected.
- API unchanged.
- DB schema unchanged.
- Runtime behavior fixed for registration-created clients only.

Whether upstream or downstream callers are affected.
- Newly registered users now receive expected default flow in eligible VLESS inbounds.

Verification

List validation commands or checks performed.
- `go test ./web/service/...`

State the result.
- Passed.

If not verified, explain why.
- No remote runtime verification in deployed environment was performed locally.

Risks And Follow-Up

Remaining risks.
- Existing already-created clients are unaffected (no migration applied).

Recommended follow-up work.
- If needed, add a one-time migration tool to backfill empty flow for existing eligible clients.
