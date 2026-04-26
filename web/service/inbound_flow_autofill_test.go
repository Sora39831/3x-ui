package service

import (
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

func TestAutoFillVisionFlowInSettings_AllClients(t *testing.T) {
	settings := `{"clients":[{"id":"a","email":"a@test","flow":""},{"id":"b","email":"b@test"}]}`
	stream := `{"network":"tcp","security":"tls"}`

	updated, changed, err := autoFillVisionFlowInSettings(settings, model.VLESS, stream, nil)
	if err != nil {
		t.Fatalf("autoFillVisionFlowInSettings() error = %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if strings.Count(updated, "xtls-rprx-vision") != 2 {
		t.Fatalf("expected both clients to be auto-filled, got: %s", updated)
	}
}

func TestAutoFillVisionFlowInSettings_SelectedClientsOnly(t *testing.T) {
	settings := `{"clients":[{"id":"a","email":"a@test","flow":""},{"id":"b","email":"b@test","flow":""}]}`
	stream := `{"network":"tcp","security":"reality"}`
	targetIDs := map[string]struct{}{"b": {}}

	updated, changed, err := autoFillVisionFlowInSettings(settings, model.VLESS, stream, targetIDs)
	if err != nil {
		t.Fatalf("autoFillVisionFlowInSettings() error = %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(updated), &m); err != nil {
		t.Fatalf("unmarshal updated settings: %v", err)
	}
	clients, ok := m["clients"].([]any)
	if !ok || len(clients) != 2 {
		t.Fatalf("clients parse failed: %#v", m["clients"])
	}
	a, _ := clients[0].(map[string]any)
	b, _ := clients[1].(map[string]any)
	if flowA, _ := a["flow"].(string); flowA != "" {
		t.Fatalf("client a flow should remain empty, got %q", flowA)
	}
	if flowB, _ := b["flow"].(string); flowB != "xtls-rprx-vision" {
		t.Fatalf("client b flow should be auto-filled, got %q", flowB)
	}
}

func TestAutoFillVisionFlowInSettings_SkipWhenFlowNotRequired(t *testing.T) {
	settings := `{"clients":[{"id":"a","email":"a@test","flow":""}]}`
	stream := `{"network":"ws","security":"tls"}`

	updated, changed, err := autoFillVisionFlowInSettings(settings, model.VLESS, stream, nil)
	if err != nil {
		t.Fatalf("autoFillVisionFlowInSettings() error = %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false")
	}
	if updated != settings {
		t.Fatalf("settings should stay unchanged")
	}
}
