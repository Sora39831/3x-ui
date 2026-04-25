# Fix: Clash proxy YAML indentation for Clash Verge

## Date: 2026-04-25

## Changes
- `sub/subClashService.go` — Fixed YAML indentation: proxy fields now use 4-space indent under list item, nested `*-opts` fields use 6-space indent. Previously fields were at same level as `- name:` causing "invalid yaml" in Clash Verge.

## Version
- v1.7.2.2 (same)
