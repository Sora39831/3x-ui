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

func TestFingerprintCacheHeaderIncludesImmutableForDotfile(t *testing.T) {
	got := assetCacheControl(".env.12345678")
	if !strings.Contains(got, "immutable") {
		t.Fatalf("expected immutable cache-control for dotfile, got %q", got)
	}
}

func TestFingerprintCacheHeaderSupportsVariableHexLength(t *testing.T) {
	got := assetCacheControl("js/websocket.123456789abc.js")
	if !strings.Contains(got, "immutable") {
		t.Fatalf("expected immutable cache-control for variable-length hash, got %q", got)
	}
}

func TestFingerprintCacheHeaderRejectsObviousNonFingerprint(t *testing.T) {
	got := assetCacheControl("js/websocket.nothex123.js")
	if strings.Contains(got, "immutable") {
		t.Fatalf("expected short-lived cache-control for non-fingerprint, got %q", got)
	}
}

func TestAssetRequestCacheControlDoesNotMarkMissingFingerprintPathImmutable(t *testing.T) {
	got := assetRequestCacheControl("js/missing.123456789abc.js", false)
	if strings.Contains(got, "immutable") {
		t.Fatalf("expected missing asset path to avoid immutable cache-control, got %q", got)
	}
}
