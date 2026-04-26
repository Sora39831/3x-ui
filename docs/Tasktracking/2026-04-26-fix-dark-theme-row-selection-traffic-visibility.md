# Task Record

Date: 2026-04-26
Related Module: web/frontend (CSS, HTML templates)
Change Type: Fix

## Background

In dark theme, when multi-selecting rows in the inbound client table, the traffic and expiry columns became invisible. The text appeared white/blank because the nested `<td>` elements inside the traffic/expiry cells inherited a near-white background (`#fafafa`) from Ant Design's row-selection CSS while the dark theme text color (`rgba(255,255,255,0.75)`) was inherited from the table, resulting in white text on a white background with no contrast.

The root cause was that the 3X-UI dark theme override rule `.dark .ant-table-row-selected>td` used the child combinator `>` which only targets direct `<td>` children of the selected row. However, the traffic and expiry cells contain a nested `<table>` with their own `<td>` elements (`.tr-table-rt`, `.tr-table-bar`, `.tr-table-lt`), which were matched by Ant Design's descendant selector `.ant-table-tbody>tr.ant-table-row-selected td` (setting `background:#fafafa; color:inherit`) but NOT by the 3X-UI override.

Additionally, the batch edit bar's `margin-bottom` was only 8px, creating insufficient visual separation from the inner client table below it.

## Changes

1. `web/assets/css/custom.min.css`: Changed `.dark .ant-table-row-selected>td{...}` to `.dark .ant-table-row-selected td{background-color:rgba(0,135,113,.15)!important;color:var(--dark-color-text-primary)}`
   - Removed `>` child combinator → descendant selector to cover nested `<td>` in traffic/expiry cells
   - Added explicit `color:var(--dark-color-text-primary)` to override Ant Design's `color:inherit`

2. `web/html/inbounds.html`: Changed batch edit bar `marginBottom` from `8px` to `15px`

3. `web/public/assets/css/custom.min.*.css`: Regenerated via `go run ./cmd/genassets` with new content hash `0b627998`

4. `web/public/assets-manifest.json`: Updated entry for `css/custom.min.css` to new hash

## Impact

- **Affected columns in inner client table**: `traffic` and `expiryTime` (both contain nested `<td>` in popover trigger slot)
- **Not affected**: `info` column (popover content slot renders in portal, not in table cell DOM)
- **Themes**: dark mode (`[data-theme]` classes `.dark`, `.ultra-dark`)
- **Modes**: both debug and production builds, since both source and fingerprinted output were updated
- No database, API, or configuration changes

## Verification

- CSS fix confirmed present in both source (`web/assets/css/custom.min.css`) and fingerprinted output (`web/public/assets/css/custom.min.0b627998.css`)
- `marginBottom: '15px'` confirmed in `web/html/inbounds.html`
- `genassets` ran successfully, manifest updated correctly, old fingerprinted file removed
- No Go code changes → `go vet ./...` and `gofmt -d .` are not expected to produce diff
- **Not verified in browser** (requires running server with dark theme + multi-select to visually confirm)

## Risks And Follow-Up

- Low risk: descendant selector `.dark .ant-table-row-selected td` applies to all `<td>` descendants, which is intentional but broader than before. If any future nested tables within selected rows are expected to have different backgrounds, they would need explicit override rules.
- The `!important` flag on background-color may conflict if other CSS frameworks are introduced.
