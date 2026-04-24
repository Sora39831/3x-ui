# Pre-release Install/Update Selection

## Summary

Add interactive prompts to `install.sh` and `update.sh` so users can choose between the latest **Stable** release or the latest **Pre-release** when installing or updating 3x-ui.

## Current State

- `install.sh` and `update.sh` both hardcode `GET /repos/Sora39831/3x-ui/releases/latest`, which only returns stable releases.
- No mechanism exists to install or update to a pre-release version through the automated flow.

## Design

### 1. GitHub API Fetch Helper

A shared function (duplicated in both `install.sh` and `update.sh`, matching existing script conventions) that:

- Calls `GET https://api.github.com/repos/Sora39831/3x-ui/releases` (returns all releases)
- Parses the JSON response to extract:
  - `latest_stable_tag` — first entry with `"prerelease": false`
  - `latest_prerelease_tag` — first entry with `"prerelease": true` (empty if none exists)
- Uses `grep`/`sed`/`awk` (no `jq` dependency, consistent with existing parsing patterns)
- Falls back to `curl -4` on IPv6 failure, matching existing retry pattern

### 2. Interactive Prompt

Both scripts display a menu after fetching release info:

```
Which version do you want to install/update?
  1) Latest Stable: v2.x.x
  2) Latest Pre-release: v2.x.x-beta
Please enter your choice [1-2]:
```

Behavior:
- Show actual version tags so the user knows what they're selecting
- If no pre-release exists: skip prompt, use stable automatically
- If no stable release exists (edge case): skip prompt, use pre-release automatically
- Invalid input re-prompts

### 3. install.sh Changes

In `install_x-ui()`, the no-argument path (line ~879):

**Before:** Calls `/releases/latest`, parses single tag, downloads.

**After:**
1. Call fetch helper to get both tags
2. Show interactive prompt
3. Set `tag_version` from user choice
4. Download as before (existing logic unchanged)

The specific-version path (`$1` argument) is unchanged.

### 4. update.sh Changes

In `update_x-ui()`, same pattern:

**Before:** Calls `/releases/latest`, parses single tag, downloads.

**After:**
1. Call fetch helper to get both tags
2. Show interactive prompt
3. Set `tag_version` from user choice
4. Continue existing update logic (unchanged)

`x-ui.sh` is **not modified** — it delegates to `update.sh` already.

## Files Modified

- `install.sh` — add fetch helper + prompt in `install_x-ui()`
- `update.sh` — add fetch helper + prompt in `update_x-ui()`

## Out of Scope

- Persisting user's choice across updates (always prompt each time)
- CLI flags like `--pre-release` for non-interactive use
- Changes to `x-ui.sh` (delegation is already in place)
