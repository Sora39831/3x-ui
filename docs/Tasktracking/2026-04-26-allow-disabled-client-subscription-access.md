Task Record

Date: 2026-04-26
Related Module: sub subscription services
Change Type: Fix

Background

Current subscription generation logic filters clients by both `subId` and `enable=true`.
As a result, when a client account is disabled, subscription endpoints cannot return Subscription Link or Clash Link content.
Requirement is to allow access to subscription and Clash links even when the account is not enabled.

Changes

Updated client matching conditions in subscription generation flows to only match `subId` and not require `client.Enable`:
- `sub/subService.go` (`GetSubs`)
- `sub/subJsonService.go` (`GetJson`)
- `sub/subClashService.go` (`GetClash`)

The inbound-level enable filter remains unchanged.

Impact

Affected modules or files.
- `sub/subService.go`
- `sub/subJsonService.go`
- `sub/subClashService.go`

Whether APIs, database, config, build, or compatibility are affected.
- API routes unchanged.
- Database schema unchanged.
- Configuration schema unchanged.
- Runtime behavior changed: disabled clients with valid `subId` can now receive subscription payloads.

Whether upstream or downstream callers are affected.
- Subscription consumers (normal/JSON/Clash links) now receive content even when client enable flag is false.

Verification

List validation commands or checks performed.
- `go test ./sub/...`

State the result.
- Passed.

If not verified, explain why.
- No remote runtime deployment verification was performed in this local environment.

Risks And Follow-Up

Remaining risks.
- This change intentionally relaxes access control semantics for disabled clients at subscription layer. If disable is expected to fully revoke access, this behavior is now different by design.

Recommended follow-up work.
- Confirm product expectation on whether this policy should also apply to other export channels (if any).
