# 2026-04-25 Fix self-closing a-empty tag swallowing sibling elements

## Problem

Worker node panel showed an empty card body instead of the master node info table.
The API returned correct data (`/panel/api/nodes/list` had the master node), but the
rendered DOM was `<div><!----></div>` — both the `<a-empty>` and the info table `<div>`
were missing.

## Root Cause

Self-closing custom elements (`<a-empty ... />`) are invalid in HTML5 in-DOM templates.
The browser's HTML parser does NOT treat `/>`  as self-closing for custom elements — it
treats `<a-empty ...>` as an **opening tag** and looks for `</a-empty>`.

This caused the next sibling `<div v-if="nodes.length > 0">` to become a **child** of
`<a-empty>` instead of a sibling. When `nodes.length > 0`, the `v-if="nodes.length === 0"`
on `<a-empty>` was false, and Vue skipped rendering its entire subtree — including the
info table that was accidentally nested inside it.

Verified with a headless browser:
- `<a-empty ... />` followed by `<div>`: div NOT visible (swallowed)
- `<a-empty ...></a-empty>` followed by `<div>`: div visible (correct)

## Fix

Changed all self-closing `<a-empty ... />` to explicit `<a-empty ...></a-empty>` in
`nodes.html`.

## Files Changed

- `web/html/nodes.html`: 2 self-closing `<a-empty>` tags → explicit closing tags
