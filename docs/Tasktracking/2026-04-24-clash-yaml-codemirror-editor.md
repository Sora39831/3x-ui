# 2026-04-24: Clash YAML CodeMirror Editor + Settings Save Button Fix

## Changes
1. **Fix: settings save button not enabling when toggling Clash Subscription**
   - `confAlerts` computed property crashed when `subClashURI`/`subURI`/`subJsonURI` was null/undefined
   - Added `|| ''` fallback before `.length` access for all three URI fields

2. **Feat: CodeMirror YAML editor for Clash template**
   - Replaced plain `<a-textarea>` with CodeMirror editor (YAML syntax highlighting, line numbers, auto-indent)
   - Added `web/assets/codemirror/yaml.js` (CodeMirror 5.65.1 YAML mode)
   - Updated `settings.html` with CodeMirror CSS/JS includes, tab change handler, and init method
   - Updated `clash.html` to use hidden textarea for CodeMirror attachment

3. **Chore: version bump to v1.5.4.1-beta**

## Tag
- `v1.5.4.1-beta`
