# Task Record

Date: 2026-06-17
Related Module: web/service, web/controller, web/html, config, xray
Change Type: Add

## Background
Worker nodes sync ALL xray template configuration from master node via shared DB. There was no mechanism for a worker to maintain independent local configuration items (e.g., custom routing rules, DNS settings, log levels) that survive master syncs. Users needed per-worker configuration overrides that stay local.

## Changes
- **config/config.go**: Added `GetXrayOverridePath()` returning `<DBFolder>/xray-override.json`
- **web/service/setting.go**: Added `saveXrayTemplateOverrideToFile()` and `getXrayTemplateOverrideFromFile()` for local file I/O (file not found returns empty, no error)
- **web/service/xray_setting.go**: Added `SaveXrayOverride()` (validates JSON then writes local file) and `GetXrayOverride()` methods on `XraySettingService`
- **web/service/xray.go**: Added `mergeXrayConfig()` (top-level key replacement merge). Modified `BuildConfigFromInbounds()` to load local override and merge before building xray.Config
- **web/controller/xray_setting.go**: Added `GET /panel/xray/override` and `POST /panel/xray/override` routes with handlers
- **web/html/settings/xray/override.html**: New template with hidden textarea for CodeMirror binding (new template file)
- **web/html/xray.html**: Added "Local Override" tab (`tpl-override`) with `force-render`, `currentTab` tracking, `xrayOverride`/`oldXrayOverride` data properties, `changePage` extension, `updateXraySetting` override branch, dirty-check extension, and `initOverrideEditor()`/`getXrayOverride()`/`saveXrayOverride()` methods
- **web/translation/*.toml**: Added `pages.xray.override` and `pages.xray.overrideDesc` in English and Chinese

## Impact
- **No sync impact**: Override save writes only local file, never bumps `shared_accounts_version` — other nodes unaffected
- **All nodes**: Override tab visible on all nodes (master and worker), but merge only matters on workers where template is synced from elsewhere
- **Existing behavior preserved**: Empty/missing override file → no merge → identical to before
- **Merge strategy**: Top-level key replacement (not recursive deep merge). Example: `{"routing": {...}}` replaces entire routing, does not merge individual rules

## Verification
- `gofmt -l -w .` → no changes needed
- `go vet ./...` → clean
- `go build -ldflags "-w -s"` → succeeds
- `go test -race -run "TestMergeXrayConfig|TestSaveXrayOverride|TestGetXrayOverride|TestXrayOverride|TestSaveAndLoad|TestGetOverride|TestBuildConfigFromInbounds" ./web/service/ -v` → 17/17 PASS
- `go test -race ./...` → ALL PASS (all packages)

## Risks And Follow-Up
- **ATOMIC WRITES**: `os.WriteFile` is not atomic — theoretically a concurrent read could see partial data. In practice override writes are user-triggered (rare) and reads occur during sync cycles. Same pattern as clashTemplate/servers files.
- **Not auto-restart**: Saving override does not trigger xray restart (consistent with template save behavior). Override takes effect on next sync cycle or manual restart.
- **No UI per-node control**: All nodes see the override tab. No server-side role gating. Users should understand override only has meaning on workers.
