Task Record: Fix User Quick Import Dropdown

Date: 2026-04-26
Related Module: User panel
Change Type: Fix

Background

The `/panel/user` quick import dropdown had three menu entries for Android, iOS, and Desktop. The entries were clickable, but the frontend did not render them visibly in the dropdown.

Changes

Updated the quick import dropdown menu to use Ant Design Vue's menu `theme` prop instead of applying the current theme as a plain CSS class.
Added a dedicated dropdown overlay class so the detached popup layer can inherit page theme styles.
Added scoped sizing and alignment styles for the quick import menu icons and items.

Impact

Affected file: `web/html/user.html`.
No API, database, configuration, build, or compatibility changes were made.
The existing Android, iOS, and Desktop click handlers are unchanged.

Verification

Ran `go test ./web/...`; it failed because the default Go build cache path `/root/.cache/go-build` is read-only in this environment.
Ran `GOCACHE=/tmp/go-build go test ./web/...`; it passed.
No local browser runtime verification was performed, following the project constraint to avoid local panel startup and integration runs.

Risks And Follow-Up

The fix targets the dropdown rendering path only. A remote browser check on `/panel/user` is recommended after deployment to confirm the dropdown is visible in both light and dark themes.
