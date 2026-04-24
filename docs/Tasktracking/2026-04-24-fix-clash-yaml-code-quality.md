# Fix Clash YAML Subscription Code Quality Issues

**Date:** 2026-04-24
**Type:** bugfix / code-quality
**Version:** v1.5.3-beta

## Issues Fixed

- **YAML injection (C2):** All string values in Clash YAML generation changed from `%s` to `%q` for proper quoting
- **Unused parameter (I1):** Removed unused `host` parameter from `GetClash` method
- **Path validation (I3):** Added backend normalization for `SubClashPath` (auto-add leading/trailing `/`)
- **Silent JSON error (I4):** Added warning log when `StreamSettings` JSON unmarshal fails
- **Template visibility:** Clash YAML template panel now expanded by default
- **Default template:** Added sensible default Clash YAML template in settings defaults

## Note

C1 from review (nil `inboundService` panic) was a false alarm — `InboundService` is a value type, not a pointer.
