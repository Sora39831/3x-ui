# Fix User Panel Traffic Bar Overflow & Merge Quick Import with Subscription Link

**Date:** 2026-05-06
**Version:** 1.8.2.3

## Changes

### Fix traffic bar overflow
- Added `:show-info="false"` to `<a-progress>` in total traffic to hide built-in percentage label that caused overflow in narrow `a-descriptions-item` container
- Kept "used / total" text above the progress bar

### Merge quick import & subscription into one card
- Merged separate Clash Link Card and Quick Import Actions Card into a single combined card
- Changed 3 vertical `block` buttons (Android, iOS, Desktop) to horizontal `size="small"` buttons in one row
- Added centered section labels: "一键添加" and "订阅链接"
- Subscription URL and copy button now sit inside the same card below the quick import buttons

### i18n
- Added `subscriptionLink` key to `translate.en_US.toml` ("Subscription Link") and `translate.zh_CN.toml` ("订阅链接")

## Files modified
- `web/html/user.html` — traffic bar fix, merged card layout, updated CSS
- `web/translation/translate.en_US.toml` — new i18n key
- `web/translation/translate.zh_CN.toml` — new i18n key
- `config/version` — bump to 1.8.2.3
