package assetsgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	if opts.HashLen > sha256.Size*2 {
		opts.HashLen = sha256.Size * 2
	}

	manifest := make(Manifest)
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

	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

func fingerprint(rel, hash string) string {
	name := filepath.Base(rel)
	if strings.HasPrefix(name, ".") && strings.Count(name, ".") == 1 {
		return rel + "." + hash
	}

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
