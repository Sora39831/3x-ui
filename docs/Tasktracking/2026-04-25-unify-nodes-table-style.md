# 2026-04-25 Unify master/worker connected nodes table to a-table

## Problem

Worker node used a plain HTML `<table>` to display the connected master node, while master
used `<a-table>`. This caused several inconsistencies:
- Worker table had no role column, no ellipsis on error column
- Worker empty state had dead-code ternary (checking `nodeRole === 'master'` inside a `v-if="nodeRole === 'worker'"` block)
- Two separate `<a-empty>` instances with overlapping conditions
- 8 lines of CSS (`.node-info-table`, `.node-info-wrap`) only used by the worker table

## Fix

Unified both views to a single `<a-table>` using the existing `nodeColumns` definition:
- Removed `v-if="nodeRole === 'master'"` so the table renders for both roles
- Worker now shows all 7 columns (including role) with proper ellipsis on error
- Empty state handled via `<a-table :locale="{ emptyText: ... }">` with role-aware message
- Removed unused `.node-info-table` / `.node-info-wrap` CSS
- Removed the duplicate `<a-empty>` and the dead-code ternary

## Safety

The previous v1.6.6.x fixes are not regressed:
- `a-descriptions` tree-shaking issue: not applicable, `<a-table>` is already in the bundle (used by master)
- `v-else` on table: removed entirely, no conditional rendering on table element
- Self-closing `<a-empty>`: removed worker's `<a-empty>`, empty state is now via table locale prop

## Files Changed

- `web/html/nodes.html`: unified to single `<a-table>`, removed plain table + CSS + duplicate empty state
