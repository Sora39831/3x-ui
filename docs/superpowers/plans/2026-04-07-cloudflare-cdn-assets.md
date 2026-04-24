# Cloudflare CDN Assets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add build-time fingerprinted frontend assets, manifest-based template asset resolution, and Cloudflare-friendly cache headers without changing panel behavior.

**Architecture:** Introduce a small Go asset-generation package plus a CLI that transforms `web/assets` into fingerprinted files under `web/public/assets` and emits `web/public/assets-manifest.json`. Update the web server to load the manifest in production, expose an `asset` template helper, serve the generated embedded files, and separate HTML caching from immutable asset caching.

**Tech Stack:** Go 1.26, `go:embed`, Gin, standard library `crypto/sha256`, `encoding/json`, `html/template`, Go tests

---

## File Structure

- Create: `web/assetsgen/generator.go`
- Create: `web/assetsgen/generator_test.go`
- Create: `cmd/genassets/main.go`
- Create: `web/public/.gitkeep`
- Create: `web/public/README.md`
- Create: `web/asset_manifest.go`
- Create: `web/asset_manifest_test.go`
- Modify: `web/web.go`
- Modify: `web/html/common/page.html`
- Modify: `web/html/component/aPersianDatepicker.html`
- Modify: `web/html/inbounds.html`
- Modify: `web/html/settings/panel/subscription/subpage.html`
- Modify: `web/html/settings.html`
- Modify: `web/html/xray.html`
- Modify: `Dockerfile`
- Modify: `README.md`

### Task 1: Build Fingerprinted Asset Generator

**Files:**
- Create: `web/assetsgen/generator.go`
- Test: `web/assetsgen/generator_test.go`

- [ ] **Step 1: Write the failing generator tests**

```go
package assetsgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateProducesFingerprintManifestAndFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := os.MkdirAll(filepath.Join(src, "js"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "js", "app.js"), []byte("console.log('v1')\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := Generate(Options{
		SourceDir: src,
		OutputDir: filepath.Join(dst, "assets"),
		HashLen:   8,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	got, ok := manifest["js/app.js"]
	if !ok {
		t.Fatalf("manifest missing logical path: %#v", manifest)
	}
	if got == "js/app.js" {
		t.Fatalf("expected hashed filename, got %q", got)
	}

	if _, err := os.Stat(filepath.Join(dst, "assets", got)); err != nil {
		t.Fatalf("hashed output missing: %v", err)
	}
}

func TestWriteManifestSerializesStableJson(t *testing.T) {
	dst := t.TempDir()
	path := filepath.Join(dst, "assets-manifest.json")
	manifest := Manifest{
		"css/a.css": "css/a.11111111.css",
		"js/b.js":   "js/b.22222222.js",
	}

	if err := WriteManifest(path, manifest); err != nil {
		t.Fatalf("WriteManifest returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]string
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("manifest json invalid: %v", err)
	}
	if decoded["js/b.js"] != "js/b.22222222.js" {
		t.Fatalf("unexpected manifest entry: %#v", decoded)
	}
}
```

- [ ] **Step 2: Run the generator tests to verify they fail**

Run: `go test ./web/assetsgen -run 'TestGenerate|TestWriteManifest' -count=1`

Expected: FAIL with undefined `Generate`, `Options`, `Manifest`, or `WriteManifest`.

- [ ] **Step 3: Write the minimal generator implementation**

```go
package assetsgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Manifest map[string]string

type Options struct {
	SourceDir string
	OutputDir string
	HashLen   int
}

func Generate(opts Options) (Manifest, error) {
	if opts.HashLen <= 0 {
		opts.HashLen = 8
	}

	manifest := make(Manifest)
	if err := os.RemoveAll(opts.OutputDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, err
	}

	err := filepath.WalkDir(opts.SourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(opts.SourceDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(raw)
		hash := hex.EncodeToString(sum[:])[:opts.HashLen]
		target := fingerprint(rel, hash)
		targetPath := filepath.Join(opts.OutputDir, filepath.FromSlash(target))

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, raw, 0o644); err != nil {
			return err
		}

		manifest[rel] = target
		return nil
	})
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

func WriteManifest(path string, manifest Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	keys := make([]string, 0, len(manifest))
	for key := range manifest {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ordered := make(map[string]string, len(keys))
	for _, key := range keys {
		ordered[key] = manifest[key]
	}

	raw, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

func fingerprint(rel, hash string) string {
	ext := filepath.Ext(rel)
	base := strings.TrimSuffix(rel, ext)
	if ext == "" {
		return rel + "." + hash
	}
	return base + "." + hash + ext
}

func CopyFile(dst string, src io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, src)
	return err
}
```

- [ ] **Step 4: Run the generator tests to verify they pass**

Run: `go test ./web/assetsgen -run 'TestGenerate|TestWriteManifest' -count=1`

Expected: PASS

- [ ] **Step 5: Commit the generator package**

```bash
git add web/assetsgen/generator.go web/assetsgen/generator_test.go
git commit -m "feat: add fingerprinted asset generator"
```

### Task 2: Add Generator CLI and Generated Output Conventions

**Files:**
- Create: `cmd/genassets/main.go`
- Create: `web/public/.gitkeep`
- Create: `web/public/README.md`
- Test: `web/assetsgen/generator_test.go`

- [ ] **Step 1: Extend tests for nested paths and hash placement**

```go
func TestGeneratePreservesNestedDirectories(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := os.MkdirAll(filepath.Join(src, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "css", "custom.min.css"), []byte("body{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := Generate(Options{
		SourceDir: src,
		OutputDir: filepath.Join(dst, "assets"),
		HashLen:   8,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	got := manifest["css/custom.min.css"]
	if got == "" {
		t.Fatalf("missing css/custom.min.css entry: %#v", manifest)
	}
	if filepath.Dir(got) != "css" {
		t.Fatalf("expected nested output directory, got %q", got)
	}
	if filepath.Ext(got) != ".css" {
		t.Fatalf("expected css extension, got %q", got)
	}
}
```

- [ ] **Step 2: Run the test to verify the generator still drives the behavior**

Run: `go test ./web/assetsgen -run TestGeneratePreservesNestedDirectories -count=1`

Expected: PASS after Task 1, confirming the generator already satisfies the nested-path rule.

- [ ] **Step 3: Add the CLI entrypoint**

```go
package main

import (
	"log"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v2/web/assetsgen"
)

func main() {
	const (
		sourceDir    = "web/assets"
		outputDir    = "web/public/assets"
		manifestPath = "web/public/assets-manifest.json"
	)

	manifest, err := assetsgen.Generate(assetsgen.Options{
		SourceDir: sourceDir,
		OutputDir: outputDir,
		HashLen:   8,
	})
	if err != nil {
		log.Fatalf("generate fingerprinted assets: %v", err)
	}

	if err := assetsgen.WriteManifest(filepath.Clean(manifestPath), manifest); err != nil {
		log.Fatalf("write asset manifest: %v", err)
	}
}
```

- [ ] **Step 4: Add generated-output conventions**

`web/public/README.md`

```md
# Generated frontend assets

This directory is generated from `web/assets` by:

- `go run ./cmd/genassets`

Contents:

- `assets/`: fingerprinted files for production embedding
- `assets-manifest.json`: logical-to-fingerprinted path mapping

Do not edit generated files by hand.
```

`web/public/.gitkeep`

```text

```

- [ ] **Step 5: Verify the CLI generates output**

Run: `go run ./cmd/genassets`

Expected: `web/public/assets-manifest.json` exists and `web/public/assets/` contains hashed files.

- [ ] **Step 6: Commit the CLI and generated-output convention**

```bash
git add cmd/genassets/main.go web/public/.gitkeep web/public/README.md
git commit -m "feat: add asset generation command"
```

### Task 3: Load Asset Manifest and Serve Fingerprinted Assets

**Files:**
- Create: `web/asset_manifest.go`
- Test: `web/asset_manifest_test.go`
- Modify: `web/web.go`

- [ ] **Step 1: Write the failing manifest and helper tests**

```go
package web

import (
	"strings"
	"testing"
)

func TestAssetResolverReturnsFingerprintedPathInProduction(t *testing.T) {
	resolver := newAssetResolver("/panel/", false, assetManifest{
		"js/websocket.js": "js/websocket.12345678.js",
	})

	got := resolver.URL("js/websocket.js")
	want := "/panel/assets/js/websocket.12345678.js"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestAssetResolverReturnsLogicalPathInDebug(t *testing.T) {
	resolver := newAssetResolver("/panel/", true, nil)

	got := resolver.URL("js/websocket.js")
	want := "/panel/assets/js/websocket.js"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestAssetResolverPanicsOnMissingProductionAsset(t *testing.T) {
	resolver := newAssetResolver("/", false, assetManifest{})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing manifest key")
		}
	}()

	resolver.URL("missing.js")
}

func TestFingerprintCacheHeaderIncludesImmutable(t *testing.T) {
	got := assetCacheControl("js/websocket.12345678.js")
	if !strings.Contains(got, "immutable") {
		t.Fatalf("expected immutable cache-control, got %q", got)
	}
}
```

- [ ] **Step 2: Run the web tests to verify they fail**

Run: `go test ./web -run 'TestAssetResolver|TestFingerprintCacheHeader' -count=1`

Expected: FAIL with undefined `newAssetResolver`, `assetManifest`, or `assetCacheControl`.

- [ ] **Step 3: Add the manifest loader and resolver**

`web/asset_manifest.go`

```go
package web

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

type assetManifest map[string]string

type assetResolver struct {
	basePath string
	debug    bool
	manifest assetManifest
}

func newAssetResolver(basePath string, debug bool, manifest assetManifest) assetResolver {
	return assetResolver{
		basePath: basePath,
		debug:    debug,
		manifest: manifest,
	}
}

func (r assetResolver) URL(logical string) string {
	target := logical
	if !r.debug {
		hashed, ok := r.manifest[logical]
		if !ok {
			panic(fmt.Sprintf("missing asset manifest entry for %q", logical))
		}
		target = hashed
	}
	return path.Join(r.basePath, "assets", target)
}

func loadAssetManifest(raw []byte) (assetManifest, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("asset manifest is empty")
	}
	var manifest assetManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	if len(manifest) == 0 {
		return nil, fmt.Errorf("asset manifest has no entries")
	}
	return manifest, nil
}

func assetCacheControl(requestPath string) string {
	if hasFingerprint(requestPath) {
		return "public, max-age=31536000, immutable"
	}
	return "public, max-age=300"
}

func hasFingerprint(requestPath string) bool {
	base := path.Base(requestPath)
	parts := strings.Split(base, ".")
	if len(parts) < 3 {
		return false
	}
	hash := parts[len(parts)-2]
	if len(hash) != 8 {
		return false
	}
	for _, ch := range hash {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Wire manifest loading and static serving into `web/web.go`**

Add or update the embedded assets section and router setup with code shaped like this:

```go
//go:embed public/assets
var publicAssetsFS embed.FS

//go:embed public/assets-manifest.json
var assetsManifestRaw []byte

var productionAssetManifest assetManifest

func init() {
	if config.IsDebug() {
		return
	}
	manifest, err := loadAssetManifest(assetsManifestRaw)
	if err != nil {
		panic(err)
	}
	productionAssetManifest = manifest
}
```

In `initRouter`, register the helper and cache policy:

```go
assetResolver := newAssetResolver(basePath, config.IsDebug(), productionAssetManifest)

funcMap := template.FuncMap{
	"i18n":  i18nWebFunc,
	"asset": assetResolver.URL,
}

engine.Use(func(c *gin.Context) {
	uri := c.Request.URL.Path
	if strings.HasPrefix(uri, assetsBasePath) {
		c.Header("Cache-Control", assetCacheControl(uri))
		return
	}
	if c.Request.Method == http.MethodGet {
		c.Header("Cache-Control", "no-cache, must-revalidate")
	}
})
```

And switch the production static filesystem to:

```go
engine.StaticFS(basePath+"assets", http.FS(&wrapAssetsFS{FS: publicAssetsFS}))
```

- [ ] **Step 5: Run the web tests to verify they pass**

Run: `go test ./web -run 'TestAssetResolver|TestFingerprintCacheHeader' -count=1`

Expected: PASS

- [ ] **Step 6: Commit the server-side asset resolution changes**

```bash
git add web/asset_manifest.go web/asset_manifest_test.go web/web.go
git commit -m "feat: load fingerprinted asset manifest"
```

### Task 4: Update Templates to Use Manifest-Based Asset URLs

**Files:**
- Modify: `web/html/common/page.html`
- Modify: `web/html/component/aPersianDatepicker.html`
- Modify: `web/html/inbounds.html`
- Modify: `web/html/settings/panel/subscription/subpage.html`
- Modify: `web/html/settings.html`
- Modify: `web/html/xray.html`
- Test: `web/asset_manifest_test.go`

- [ ] **Step 1: Add a template-focused regression test**

Append this test to `web/asset_manifest_test.go`:

```go
func TestAssetResolverPreservesBasePathWithoutDoubleSlash(t *testing.T) {
	resolver := newAssetResolver("/xui/", false, assetManifest{
		"css/custom.min.css": "css/custom.min.11111111.css",
	})

	got := resolver.URL("css/custom.min.css")
	want := "/xui/assets/css/custom.min.11111111.css"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
```

- [ ] **Step 2: Run the focused test before template edits**

Run: `go test ./web -run TestAssetResolverPreservesBasePathWithoutDoubleSlash -count=1`

Expected: PASS after Task 3, confirming the helper output is safe to roll through templates.

- [ ] **Step 3: Replace direct asset URLs in shared page template**

Update `web/html/common/page.html` references like this:

```gotemplate
<link rel="stylesheet" href="{{ asset "ant-design-vue/antd.min.css" }}">
<link rel="stylesheet" href="{{ asset "css/custom.min.css" }}">
src: url('{{ asset "Vazirmatn-UI-NL-Regular.woff2" }}') format('woff2');
<script src="{{ asset "vue/vue.min.js" }}"></script>
<script src="{{ asset "moment/moment.min.js" }}"></script>
<script src="{{ asset "ant-design-vue/antd.min.js" }}"></script>
<script src="{{ asset "axios/axios.min.js" }}"></script>
<script src="{{ asset "qs/qs.min.js" }}"></script>
<script src="{{ asset "js/axios-init.js" }}"></script>
<script src="{{ asset "js/util/index.js" }}"></script>
<script src="{{ asset "js/websocket.js" }}"></script>
```

- [ ] **Step 4: Replace page-specific asset URLs**

Apply the same conversion in these files:

`web/html/component/aPersianDatepicker.html`

```gotemplate
<link rel="stylesheet" href="{{ asset "persian-datepicker/persian-datepicker.min.css" }}" />
<script src="{{ asset "moment/moment-jalali.min.js" }}"></script>
<script src="{{ asset "persian-datepicker/persian-datepicker.min.js" }}"></script>
```

`web/html/inbounds.html`

```gotemplate
<script src="{{ asset "qrcode/qrious2.min.js" }}"></script>
<script src="{{ asset "uri/URI.min.js" }}"></script>
<script src="{{ asset "js/model/reality_targets.js" }}"></script>
<script src="{{ asset "js/model/inbound.js" }}"></script>
<script src="{{ asset "js/model/dbinbound.js" }}"></script>
```

`web/html/settings/panel/subscription/subpage.html`

```gotemplate
<script src="{{ asset "moment/moment.min.js" }}"></script>
<script src="{{ asset "moment/moment-jalali.min.js" }}"></script>
<script src="{{ asset "vue/vue.min.js" }}"></script>
<script src="{{ asset "ant-design-vue/antd.min.js" }}"></script>
<script src="{{ asset "js/util/index.js" }}"></script>
<script src="{{ asset "qrcode/qrious2.min.js" }}"></script>
<script src="{{ asset "js/subscription.js" }}"></script>
```

`web/html/settings.html`

```gotemplate
<script src="{{ asset "qrcode/qrious2.min.js" }}"></script>
<script src="{{ asset "otpauth/otpauth.umd.min.js" }}"></script>
<script src="{{ asset "js/model/setting.js" }}"></script>
```

`web/html/xray.html`

```gotemplate
<link rel="stylesheet" href="{{ asset "codemirror/codemirror.min.css" }}">
<link rel="stylesheet" href="{{ asset "codemirror/fold/foldgutter.css" }}">
<link rel="stylesheet" href="{{ asset "codemirror/xq.min.css" }}">
<link rel="stylesheet" href="{{ asset "codemirror/lint/lint.css" }}">
<script src="{{ asset "js/model/outbound.js" }}"></script>
<script src="{{ asset "codemirror/codemirror.min.js" }}"></script>
<script src="{{ asset "codemirror/javascript.js" }}"></script>
<script src="{{ asset "codemirror/jshint.js" }}"></script>
<script src="{{ asset "codemirror/jsonlint.js" }}"></script>
<script src="{{ asset "codemirror/lint/lint.js" }}"></script>
<script src="{{ asset "codemirror/lint/javascript-lint.js" }}"></script>
<script src="{{ asset "codemirror/hint/javascript-hint.js" }}"></script>
<script src="{{ asset "codemirror/fold/foldcode.js" }}"></script>
<script src="{{ asset "codemirror/fold/foldgutter.js" }}"></script>
<script src="{{ asset "codemirror/fold/brace-fold.js" }}"></script>
```

- [ ] **Step 5: Verify all fingerprintable asset references use the helper**

Run: `rg -n '{{ \\.base_path }}assets|\\?{{ \\.cur_ver }}' web/html`

Expected: no remaining static asset references; route strings like `{{ .base_path }}panel/` may remain.

- [ ] **Step 6: Commit the template updates**

```bash
git add web/html/common/page.html web/html/component/aPersianDatepicker.html web/html/inbounds.html web/html/settings/panel/subscription/subpage.html web/html/settings.html web/html/xray.html web/asset_manifest_test.go
git commit -m "refactor: resolve template assets through manifest helper"
```

### Task 5: Integrate Build Workflow and Verify End-to-End Behavior

**Files:**
- Modify: `Dockerfile`
- Modify: `README.md`
- Test: `web/web.go`

- [ ] **Step 1: Add a build-step verification test for cache policy**

Add this test to `web/asset_manifest_test.go`:

```go
func TestAssetCacheControlForLogicalPathIsShortLived(t *testing.T) {
	got := assetCacheControl("js/websocket.js")
	want := "public, max-age=300"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
```

- [ ] **Step 2: Run the cache-policy tests**

Run: `go test ./web -run 'TestFingerprintCacheHeader|TestAssetCacheControlForLogicalPathIsShortLived' -count=1`

Expected: PASS

- [ ] **Step 3: Update the builder image to generate assets before `go build`**

In `Dockerfile`, insert the generator run before the binary build:

```dockerfile
COPY . .

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN go run ./cmd/genassets
RUN go build -ldflags "-w -s" -o build/x-ui main.go
```

- [ ] **Step 4: Document the direct-build workflow**

Add a section to `README.md` like this:

```md
## Building from source

Generate fingerprinted frontend assets before compiling:

- `go run ./cmd/genassets`
- `go build -ldflags "-w -s" -o build/x-ui main.go`

Production builds embed files from `web/public/assets` and `web/public/assets-manifest.json`.
```

- [ ] **Step 5: Run end-to-end verification**

Run:

```bash
go run ./cmd/genassets
go test ./web/assetsgen -count=1
go test ./web -run 'TestAssetResolver|TestFingerprintCacheHeader|TestAssetCacheControlForLogicalPathIsShortLived' -count=1
go build ./...
```

Expected:

- generator command succeeds
- asset generation tests pass
- web asset helper tests pass
- repository builds successfully

- [ ] **Step 6: Commit the build integration**

```bash
git add Dockerfile README.md web/asset_manifest_test.go
git commit -m "build: generate fingerprinted assets before compile"
```

## Self-Review

### Spec Coverage

- Build-time generator and manifest: covered by Tasks 1 and 2.
- Production embed of generated assets: covered by Task 3.
- Manifest-based template helper: covered by Tasks 3 and 4.
- Immutable asset caching and HTML revalidation: covered by Tasks 3 and 5.
- Build and release documentation updates: covered by Task 5.

No spec gaps remain.

### Placeholder Scan

- No `TODO`, `TBD`, or deferred implementation markers remain.
- Each code-changing step includes concrete code or exact replacement snippets.
- Each verification step includes an explicit command and expected result.

### Type Consistency

- `assetManifest`, `assetResolver`, `loadAssetManifest`, and `assetCacheControl` are introduced in Task 3 and used consistently later.
- Generator APIs use `assetsgen.Options`, `assetsgen.Generate`, and `assetsgen.WriteManifest` consistently across Tasks 1, 2, and 5.
