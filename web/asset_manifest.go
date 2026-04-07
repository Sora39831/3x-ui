package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
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

func assetRequestCacheControl(requestPath string, exists bool) string {
	if exists {
		return assetCacheControl(requestPath)
	}
	return "public, max-age=300"
}

func assetExists(assetsFS fs.FS, assetPath string) bool {
	if assetPath == "" {
		return false
	}
	if _, err := fs.Stat(assetsFS, assetPath); err != nil {
		return false
	}
	return true
}

func hasFingerprint(requestPath string) bool {
	base := path.Base(requestPath)
	parts := strings.Split(base, ".")
	if len(parts) < 2 {
		return false
	}
	if isFingerprintHash(parts[len(parts)-1]) {
		return true
	}
	if len(parts) >= 3 && isFingerprintHash(parts[len(parts)-2]) {
		return true
	}
	return false
}

func isFingerprintHash(hash string) bool {
	if len(hash) < 6 || len(hash) > 64 {
		return false
	}
	for _, ch := range hash {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return false
		}
	}
	return true
}
