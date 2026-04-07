package assetsgen

import (
	"crypto/sha256"
	"encoding/hex"
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

	sum := sha256.Sum256([]byte("console.log('v1')\n"))
	wantHash := hex.EncodeToString(sum[:])[:8]
	want := "js/app." + wantHash + ".js"
	if got != want {
		t.Fatalf("unexpected hashed filename: got %q want %q", got, want)
	}

	if _, err := os.Stat(filepath.Join(dst, "assets", got)); err != nil {
		t.Fatalf("hashed output missing: %v", err)
	}

	defaultManifest, err := Generate(Options{
		SourceDir: src,
		OutputDir: filepath.Join(dst, "default-assets"),
	})
	if err != nil {
		t.Fatalf("Generate with default hash length returned error: %v", err)
	}

	if gotDefault := defaultManifest["js/app.js"]; gotDefault != want {
		t.Fatalf("default HashLen mismatch: got %q want %q", gotDefault, want)
	}

	if _, err := os.Stat(filepath.Join(dst, "default-assets", want)); err != nil {
		t.Fatalf("default hashed output missing: %v", err)
	}
}

func TestGenerateClampsHashLenToSha256HexLength(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "main.css"), []byte("body{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := Generate(Options{
		SourceDir: src,
		OutputDir: filepath.Join(dst, "assets"),
		HashLen:   65,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	sum := sha256.Sum256([]byte("body{}\n"))
	wantHash := hex.EncodeToString(sum[:])
	want := "main." + wantHash + ".css"
	if got := manifest["main.css"]; got != want {
		t.Fatalf("unexpected hashed filename: got %q want %q", got, want)
	}

	if _, err := os.Stat(filepath.Join(dst, "assets", want)); err != nil {
		t.Fatalf("clamped hashed output missing: %v", err)
	}
}

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

func TestGenerateFingerprintsDotfilesWithoutLeadingExtension(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := os.MkdirAll(filepath.Join(src, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".env"), []byte("ROOT=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "dir", ".env"), []byte("NESTED=1\n"), 0o644); err != nil {
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

	rootSum := sha256.Sum256([]byte("ROOT=1\n"))
	rootWant := ".env." + hex.EncodeToString(rootSum[:])[:8]
	if got := manifest[".env"]; got != rootWant {
		t.Fatalf("unexpected root dotfile fingerprint: got %q want %q", got, rootWant)
	}
	if _, err := os.Stat(filepath.Join(dst, "assets", rootWant)); err != nil {
		t.Fatalf("root dotfile output missing: %v", err)
	}

	nestedSum := sha256.Sum256([]byte("NESTED=1\n"))
	nestedWant := filepath.ToSlash(filepath.Join("dir", ".env.")) + hex.EncodeToString(nestedSum[:])[:8]
	if got := manifest["dir/.env"]; got != nestedWant {
		t.Fatalf("unexpected nested dotfile fingerprint: got %q want %q", got, nestedWant)
	}
	if _, err := os.Stat(filepath.Join(dst, "assets", filepath.FromSlash(nestedWant))); err != nil {
		t.Fatalf("nested dotfile output missing: %v", err)
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

	want := "{\n  \"css/a.css\": \"css/a.11111111.css\",\n  \"js/b.js\": \"js/b.22222222.js\"\n}\n"
	if string(raw) != want {
		t.Fatalf("unexpected manifest json:\n got: %q\nwant: %q", string(raw), want)
	}

	var decoded map[string]string
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("manifest json invalid: %v", err)
	}
	if decoded["js/b.js"] != "js/b.22222222.js" {
		t.Fatalf("unexpected manifest entry: %#v", decoded)
	}
}
