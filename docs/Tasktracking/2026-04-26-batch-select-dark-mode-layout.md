# Task Record

Date: 2026-04-26
Related Module: web (CSS + HTML template)
Change Type: Fix

## Background
The multi-select feature (added in `2026-04-26-batch-edit-clients.md`) had two visual issues:
1. In dark mode, selected table rows showed the Ant Design default light background (`#e6f7ff`), making text unreadable against the dark theme.
2. The batch operation toolbar was too crowded with all 7 elements (selected count + 6 buttons) packed in one row without wrapping.

## Changes
1. **CSS** (`web/assets/css/custom.min.css`): Added dark mode override `.dark .ant-table-row-selected>td{background-color:rgba(0,135,113,.15)!important}` using the primary theme color at low opacity for selected rows in dark mode. Also applied to the public fingerprinted copy.
2. **HTML** (`web/html/inbounds.html`:587-600): Restructured the batch toolbar layout:
   - Changed to a two-group flexbox with `justify-content: space-between`
   - Left group: selected count (`<strong>`) + action buttons (Batch Edit, Enable, Disabled, Reset Traffic, Delete) with `flexWrap: wrap`
   - Right group: Deselect All button
   - Added `flexWrap: wrap` on both containers for responsive wrapping
   - Reduced gap from 8px to 6px

## Impact
- CSS: Only affects dark mode table row selection styling; no impact on light mode
- HTML: Batch toolbar DIV now uses nested flex containers; functionality unchanged
- No API/DB/config changes
- Public assets regenerated via `go run ./cmd/genassets`

## Verification
- `go run ./cmd/genassets` — regenerated public assets successfully
- `CGO_ENABLED=1 go build ./...` — build succeeded
- `gofmt -l -w . && go vet ./...` — no issues

## Risks And Follow-Up
- Dark mode selected row background may need further tuning based on user feedback
- On very narrow screens the batch toolbar may still wrap to multiple lines, which is expected and handled by flex-wrap
