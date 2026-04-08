package web

import (
	"strings"
	"testing"
)

func TestLoginTemplateRebuildsTurnstileAfterTabSwitch(t *testing.T) {
	content, err := htmlFS.ReadFile("html/login.html")
	if err != nil {
		t.Fatalf("read login template: %v", err)
	}

	source := string(content)

	checks := []string{
		"turnstileContainer = null;",
		"turnstile.remove(turnstileWidgetId);",
		"turnstileContainer !== container",
		"turnstileToken = '';",
	}

	for _, check := range checks {
		if !strings.Contains(source, check) {
			t.Fatalf("expected login template to contain %q so turnstile can be rebuilt after tab switches", check)
		}
	}
}
