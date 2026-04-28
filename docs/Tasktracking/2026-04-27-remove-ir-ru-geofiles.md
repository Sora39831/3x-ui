# Task Record

Date: 2026-04-27
Related Module: web/service, web/html, x-ui.sh, .github, DockerInit, README
Change Type: Refactor

## Background
IR and RU regional geofile datasets (geoip_IR.dat, geosite_IR.dat, geoip_RU.dat, geosite_RU.dat from chocolate4u/Iran-v2ray-rules and runetfreedom/russia-v2ray-rules-dat) were removed from the project. Only the main Loyalsoldier/v2ray-rules-dat dataset (geoip.dat, geosite.dat) remains.

## Changes
- `web/service/server.go`: Removed 4 IR/RU entries from `geofileAllowlist`, keeping only `geoip.dat` and `geosite.dat`
- `web/html/index.html`: Removed IR/RU file names from the Geofiles UI file list
- `web/service/server_test.go`: Removed IR/RU file names from `TestIsValidGeofileName_Valid`
- `web/html/xray.html`: Removed all `ext:geoip_IR.dat`, `ext:geosite_IR.dat`, `ext:geosite_RU.dat` routing rule presets from IPsOptions, DomainsOptions, and BlockDomainsOptions. Kept `regexp:` entries for .ir, .ru, .su, .рф domains which do not depend on the .dat files.
- `x-ui.sh`: Removed IR and RU cases from `update_geofiles()`, `update_all_geofiles()`, and `update_geo()` menu
- `.github/workflows/release.yml`: Removed IR/RU download steps from build pipeline
- `DockerInit.sh`: Removed IR/RU download lines
- `README.md` / `README.zh_CN.md` / `README.ru_RU.md` / `README.fa_IR.md` / `README.es_ES.md` / `README.ar_EG.md`: Removed Iran/Russia v2ray rules acknowledgment lines

## Impact
- Geofile allowlist reduced from 6 to 2 files
- Geofile update/sync now only downloads geoip.dat and geosite.dat
- Routing rule UI no longer offers ext: prefixed IR/RU options
- Shell install script and CI no longer download IR/RU files
- No database schema changes, no API endpoint changes

## Verification
- `gofmt -l -w .` passed with no changes
- `go vet ./...` passed with no errors
- All IR/RU references confirmed removed from 12 files

## Risks And Follow-Up
- Users with existing Xray routing rules referencing `ext:geoip_IR.dat`, `ext:geosite_IR.dat`, or `ext:geosite_RU.dat` will need to update their configurations
- Existing .dat files on disk are not automatically deleted; users may manually remove them from the bin/ directory
