# Cloudflare CDN Frontend Asset Optimization Design

## Context

The panel currently serves frontend assets from embedded files under `web/assets` and references them directly from HTML templates. A subset of assets uses `?{{ .cur_ver }}` query strings for cache busting, while some third-party files have no version token at all. The server sets `Cache-Control: max-age=31536000` for requests under `/assets/`, and enables gzip at the Gin layer.

This works for basic browser caching, but it is not a strong fit for Cloudflare edge caching:

- Query-string cache busting is weaker than content-addressed filenames.
- Some assets are not versioned at all.
- HTML and static assets are not explicitly separated into short-cache vs long-cache behavior.
- The current embedded asset flow does not provide a manifest-based way to map logical asset names to hashed output names.

The deployment model is Go binary compilation with `go:embed`, so the design must preserve compile-time embedding and avoid runtime dependence on the local filesystem.

## Goals

- Keep all frontend assets self-hosted.
- Optimize asset delivery for Cloudflare CDN edge caching.
- Replace query-string cache busting with content-hashed filenames.
- Preserve the current HTML templates, base path support, and embedded deployment model.
- Keep API routes, session behavior, and WebSocket endpoints unchanged.

## Non-Goals

- No migration to third-party script or stylesheet CDNs.
- No change to business logic, Vue component behavior, or page structure.
- No runtime asset compilation in production.
- No broad frontend bundler migration in this change.

## Recommended Approach

Adopt a build-time asset fingerprinting pipeline that generates a new embedded asset output tree and a manifest file. Templates will resolve logical asset paths through the manifest, and the server will serve only fingerprinted asset URLs with long-lived immutable caching headers.

This is the recommended approach because it is the most compatible with Cloudflare's cache model and the current Go binary deployment flow.

## Alternatives Considered

### 1. Build-time fingerprinted assets and manifest

This is the recommended option.

Pros:

- Best Cloudflare cache efficiency and invalidation behavior.
- Safe long-lived caching with `immutable`.
- Explicit and debuggable asset mapping.
- Compatible with `go:embed`.

Cons:

- Adds a pre-build asset generation step.
- Requires template updates to use a manifest helper.

### 2. Runtime virtual hashed routes backed by embedded assets

Pros:

- No extra pre-build step.

Cons:

- Adds runtime complexity to compute or maintain mappings.
- Less transparent than generated files.
- Harder to reason about and test than build-time outputs.

### 3. Keep filenames and use per-file hash query strings

Pros:

- Smallest code change.

Cons:

- Weaker fit for Cloudflare edge caching.
- Less operationally clear than immutable fingerprinted paths.
- Leaves ambiguity around caches that normalize or vary on query strings.

## Design

### Asset Source and Output Layout

Keep `web/assets` as the source tree checked into the repository.

Add a generated output tree for embedded production assets:

- `web/public/assets/...` for fingerprinted files
- `web/public/assets-manifest.json` for logical-to-fingerprinted path mapping

`web/public` is generated content. `go:embed` in production should target the generated tree rather than the source tree.

Example mapping:

- logical: `css/custom.min.css`
- output: `css/custom.min.4f3c2a1b.css`

- logical: `js/websocket.js`
- output: `js/websocket.a9c88d71.js`

### Build Pipeline

Add a build-time generator command or script that:

1. Walks `web/assets`
2. Computes a deterministic content hash for each file
3. Writes the file into `web/public/assets` with the hash inserted before the extension
4. Emits `web/public/assets-manifest.json`

Hash requirements:

- Deterministic for identical file content
- Stable across platforms
- Short enough for readable filenames

An 8 to 12 character hex digest from SHA-256 is sufficient here.

The generator must preserve subdirectories so current logical organization remains intact.

### Manifest Format

Use a flat JSON object keyed by logical asset path relative to `web/assets`.

Example:

```json
{
  "ant-design-vue/antd.min.css": "ant-design-vue/antd.min.4f3c2a1b.css",
  "css/custom.min.css": "css/custom.min.182d7e0a.css",
  "js/axios-init.js": "js/axios-init.bf4d1d4e.js",
  "js/websocket.js": "js/websocket.a9c88d71.js",
  "Vazirmatn-UI-NL-Regular.woff2": "Vazirmatn-UI-NL-Regular.4c2a16f1.woff2"
}
```

This keeps template lookup simple and avoids path reconstruction logic.

### Embed Strategy

Replace the production asset embed source in `web/web.go` so that production serving reads from generated output, not raw source assets.

Development mode can keep serving from `web/assets` directly to avoid slowing local iteration.

Production mode behavior:

- embed `web/public/assets`
- load `web/public/assets-manifest.json`
- serve only the generated fingerprinted files

### Template Asset Resolution

Add a template function, for example `asset`, that accepts a logical asset path and returns the final URL under the current `basePath`.

Example usage in templates:

```gotemplate
<link rel="stylesheet" href="{{ asset "ant-design-vue/antd.min.css" }}">
<script src="{{ asset "vue/vue.min.js" }}"></script>
```

This replaces direct `{{ .base_path }}assets/...` references and removes `?{{ .cur_ver }}` from static asset URLs.

The helper behavior should be:

- resolve the logical path through the manifest in production
- prefix with `{{ .base_path }}assets/`
- fail loudly during server init if a required manifest entry is missing

For debug mode, the helper can return the original non-fingerprinted path so templates work unchanged during local development.

### Cache-Control Policy

Separate HTML caching from static asset caching.

HTML responses:

- `Cache-Control: no-cache, must-revalidate`

Fingerprint asset responses:

- `Cache-Control: public, max-age=31536000, immutable`

This allows Cloudflare and browsers to retain asset files for a year while ensuring HTML revalidates and can reference new asset filenames after deployment.

### ETag and Last-Modified

This design does not require ETag for fingerprinted assets because filename changes already provide cache invalidation. ETag may still be present if provided by the underlying file serving behavior, but it is not required for correctness.

`Last-Modified` is also non-critical for fingerprinted assets. The current `ModTime` override tied to process start is not a reliable version signal, and should not be treated as part of cache invalidation. The fingerprinted filename is the source of truth.

### Cloudflare Behavior

Expected Cloudflare policy after this design:

- Cache `/assets/*` aggressively at the edge
- Do not cache HTML application pages for long durations
- Avoid purge-heavy workflows because asset invalidation is filename-based

This design keeps Cloudflare configuration simple. New deployments produce new asset URLs; old assets remain safely cacheable until naturally evicted.

### Backward Compatibility

Preserve:

- `basePath` support
- current routes outside static asset delivery
- current debug mode serving behavior

Change:

- production asset references move from raw names plus optional query strings to fingerprinted filenames
- production asset embed source moves to generated output

Existing un-fingerprinted `/assets/...` paths should not remain part of the production template output. If any route continues to expose them, that should be treated as compatibility-only behavior, not a primary path.

## Implementation Outline

1. Add an asset generation tool under the repository, preferably Go-based for portability with the existing build stack.
2. Generate `web/public/assets` and `web/public/assets-manifest.json` from `web/assets`.
3. Update `go:embed` usage in production to embed the generated asset tree and manifest.
4. Add manifest loading during server initialization.
5. Add the `asset` template helper.
6. Replace direct static asset references in HTML templates with `asset(...)`.
7. Update asset response headers to use immutable long-lived caching for fingerprinted assets.
8. Keep HTML responses on short-cache or revalidation semantics.
9. Document the new build prerequisite in developer and release documentation.

## Error Handling

Server startup should fail if:

- the manifest file is missing in production
- a manifest entry is malformed
- a template references an asset key that is absent from the manifest

Fail-fast is preferable here because silent fallback would hide release integrity problems and produce broken pages under CDN caching.

## Testing Strategy

### Automated

- Unit test the asset generator:
  - stable hash naming
  - preserved directory structure
  - correct manifest output
- Unit test manifest loading:
  - valid manifest parses
  - missing or malformed entries fail
- Unit test template helper:
  - returns base-path-prefixed fingerprinted URLs in production
  - returns raw asset URLs in debug mode
- Integration test asset responses:
  - fingerprinted asset path returns `Cache-Control: public, max-age=31536000, immutable`
  - HTML response returns `Cache-Control: no-cache, must-revalidate`

### Manual

- Build a production binary and open the panel in a browser
- Inspect HTML and verify asset URLs contain hashes in filenames
- Confirm page reload after deployment references new filenames when a source asset changes
- Confirm Cloudflare can cache asset responses without manual purge

## Operational Notes

- Release workflows must run the asset generation step before `go build`.
- Developers should have a single documented command to regenerate embedded assets.
- Generated assets should either be committed consistently or regenerated in CI/build scripts. This decision should be made once and documented to avoid drift.

## Open Decision

One repository policy still needs to be chosen during implementation:

- Commit generated `web/public` outputs to git
- Or treat them as build artifacts generated before release and excluded from source control

Recommendation:

Do not commit generated fingerprinted assets if the release pipeline reliably runs the generator before building. Committing generated outputs increases churn and review noise. If the project's release flow is manual and local builds are common, committing generated outputs may be acceptable for simplicity.

## Summary

Use a build-time fingerprinting pipeline to generate embedded static assets and a manifest. Resolve template asset URLs through the manifest, serve fingerprinted asset files with one-year immutable caching, and keep HTML on revalidation semantics. This gives Cloudflare a clean, robust cache model without changing the panel's runtime behavior or introducing third-party CDNs.
