package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v2/web/assetsgen"
)

func main() {
	const (
		sourceDir    = "web/assets"
		outputDir    = "web/public/assets"
		manifestPath = "web/public/assets-manifest.json"
	)

	if err := os.RemoveAll(outputDir); err != nil {
		log.Fatalf("remove stale asset output: %v", err)
	}
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("remove stale asset manifest: %v", err)
	}

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
