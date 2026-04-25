Task Record:

Date: 2026-04-26
Related Module: web/html/user.html
Change Type: Fix

Background

The quick import entry on `/panel/user` was implemented as an Ant Design dropdown menu. In the current UI state, menu item text failed to display reliably unless SVG-related markup was manually removed, making the one-click import actions unusable.

Changes

Replaced the dropdown-based quick import block with an inline action section inside the existing card body.
Changed the quick import area to show a small title using `pages.user.quickImport`.
Added three dedicated buttons for Android, iOS, and Desktop that reuse the existing handlers: `quickImportAndroid`, `quickImportIOS`, and `quickImportDesktop`.
Removed obsolete dropdown-specific styles and added compact styles for the new button group layout.

Impact

Affected files: `web/html/user.html`.
No API, database, build pipeline, or backend logic changes.
User-panel behavior changed from "click dropdown then choose platform" to "direct platform buttons".
No upstream/downstream interface compatibility impact.

Verification

Command: `go test ./web/...`
Result: Passed.

Risks And Follow-Up

The change avoids dropdown/menu rendering paths and should be more robust for text visibility.
If further UX tuning is needed, spacing and button style can be adjusted without changing quick import logic.
