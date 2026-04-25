# 2026-04-25 Fix ensureDefaultNodeSettings and worker node display

## Problem

### 1. Test failures in config package
Two tests were failing:
- `TestWriteSettingToJSONCreatesSettingsFileWhenMissing`
- `TestWriteSettingToJSONBackfillsDefaultNodeSettings`

Both failed with: `expected other group, got <nil>`

### 2. Worker frontend not showing connected master node
The worker's node management page rendered the card structure but didn't display the
master node information. The `a-descriptions` and `a-descriptions-item` components were
used in the template but were NOT included in the Ant Design Vue bundle (`antd.min.js`).
Vue silently skipped the unregistered components, resulting in an empty card body.

## Root Cause

### Test failures
`ensureDefaultNodeSettings()` only wrote defaults to the `"node"` group. Tests expected
the `"other"` group to also have defaults for backward compatibility.

### Worker node display
Ant Design Vue 2.x uses tree-shaking — only components actually imported during the build
are included in the bundle. `a-descriptions` and `a-descriptions-item` were not imported
in the project's Ant Design Vue build config, so they were missing from `antd.min.js`.
When Vue encounters an unregistered component tag, it silently ignores it.

## Fix

### Test failures
Changed `ensureDefaultNodeSettings()` to iterate over both `"node"` and `"other"` groups,
writing defaults to both for backward compatibility.

### Worker node display
Replaced `a-descriptions` / `a-descriptions-item` with a plain HTML `<table>` that
replicates the same visual layout (label-value pairs with borders). This doesn't depend
on any Ant Design Vue component.

## Files Changed

- `config/config.go`: Modified `ensureDefaultNodeSettings()` to write to both groups
- `web/html/nodes.html`: Replaced `a-descriptions` with HTML table

## Verification

- `go test -race -shuffle=on ./...` — all PASS
