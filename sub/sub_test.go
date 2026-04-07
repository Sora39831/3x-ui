package sub

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSubscriptionTemplatesUseAssetHelper(t *testing.T) {
	engine := gin.New()
	engine.SetFuncMap(subscriptionTemplateFuncMap("/sub/", subscriptionAssetManifest{
		"moment/moment.min.js": "moment/moment.min.12345678.js",
	}))

	if err := setEmbeddedTemplates(engine); err != nil {
		t.Fatalf("setEmbeddedTemplates() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	rendered := engine.HTMLRender.Instance("subpage.html", gin.H{
		"title":     "subscription.title",
		"host":      "example.com",
		"base_path": "/sub/test-subid/",
	})

	if err := rendered.Render(recorder); err != nil {
		t.Fatalf("rendered.Render() error = %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `/sub/assets/moment/moment.min.12345678.js`) {
		t.Fatalf("rendered body missing subscription asset path: %s", body)
	}
}

func TestGetHtmlFilesReturnsCurrentSubscriptionTemplatePath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	writeTestFile(t, filepath.Join(tempDir, "web", "html", "common", "page.html"))
	writeTestFile(t, filepath.Join(tempDir, "web", "html", "component", "aThemeSwitch.html"))
	currentTemplatePath := filepath.Join(tempDir, "web", "html", "settings", "panel", "subscription", "subpage.html")
	writeTestFile(t, currentTemplatePath)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	server := NewServer()
	files, err := server.getHtmlFiles()
	if err != nil {
		t.Fatalf("getHtmlFiles() error = %v", err)
	}

	if !containsPath(files, currentTemplatePath) {
		t.Fatalf("getHtmlFiles() missing current subscription template path %q in %v", currentTemplatePath, files)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
