# 2026-04-24 Fix Settings Save Button and UI Bugs

## Problem
Settings page save button would not enable when user changed settings.

## Root Cause Analysis
Systematic debugging found multiple UI bugs affecting the settings page:

1. **Duplicate `:min` attributes on `a-input-number`** (3 locations)
   - `general.html:42` — webPort had `:min="1" :min="65535"` (second should be `:max`)
   - `subscription/general.html:43` — subPort had same issue
   - `telegram.html:64` — tgCpu had `:min="0" :min="100"` (second should be `:max`)

2. **Mismatched closing tags**
   - `general.html:42` — `<a-input-number>` closed with `</a-input>`
   - `telegram.html:64` — `<a-input-number>` closed with `</a-switch>`

3. **twoFactorEnable toggle broken** (`security.html:39`)
   - Used `@click="toggleTwoFactor" :checked="..."` instead of proper event handling
   - `@click` passes MouseEvent as first arg, not the boolean toggle value
   - Method expected boolean but received Event → always truthy → always triggered enable flow

4. **Noise input handlers referenced undefined `event`** (`json.html:91,99`)
   - `(value) => updateNoisePacket(index, event.target.value)` — `event` is not defined
   - Arrow function parameter named `value` but code accessed global `event`

5. **Polling loop had no error handling** (`settings.html:653-656`)
   - Any error in the `while(true)` loop would silently stop change detection

## Changes
- Fixed `:min`/`:max` attributes on all `a-input-number` components
- Fixed closing tags to match opening tags
- Changed twoFactorEnable to use `@click.prevent="toggleTwoFactor(!allSetting.twoFactorEnable)"`
- Updated `toggleTwoFactor` method to only set `twoFactorEnable` on success
- Fixed noise input handlers to use `(e) => ... e.target.value`
- Added try/catch around polling loop comparison

## Files Modified
- `web/html/settings/panel/general.html` — webPort input fix
- `web/html/settings/panel/subscription/general.html` — subPort input fix
- `web/html/settings/panel/telegram.html` — tgCpu input fix
- `web/html/settings/panel/security.html` — twoFactorEnable toggle fix
- `web/html/settings/panel/subscription/json.html` — noise input handler fix
- `web/html/settings.html` — toggleTwoFactor method + polling error handling

## Verification
- Visual inspection of all modified templates
- Confirmed `ObjectUtil.equals()` shallow comparison works correctly with Vue 2 reactivity
- Confirmed `AllSetting` class properties match Go struct fields
