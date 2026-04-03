package model

import (
	"testing"
)

func TestGenXrayInboundConfig_EmptyListen(t *testing.T) {
	in := &Inbound{
		Listen:   "",
		Port:     443,
		Protocol: VLESS,
		Settings: `{"clients":[]}`,
		Tag:      "test-inbound",
		Sniffing: `{"enabled":true}`,
	}
	cfg := in.GenXrayInboundConfig()
	if cfg == nil {
		t.Fatal("GenXrayInboundConfig should not return nil")
	}
	// Empty listen should default to 0.0.0.0
	expected := `"0.0.0.0"`
	if string(cfg.Listen) != expected {
		t.Errorf("Listen should default to %s, got %s", expected, string(cfg.Listen))
	}
	if cfg.Port != 443 {
		t.Errorf("Port should be 443, got %d", cfg.Port)
	}
	if cfg.Protocol != "vless" {
		t.Errorf("Protocol should be vless, got %q", cfg.Protocol)
	}
	if cfg.Tag != "test-inbound" {
		t.Errorf("Tag should be test-inbound, got %q", cfg.Tag)
	}
}

func TestGenXrayInboundConfig_CustomListen(t *testing.T) {
	in := &Inbound{
		Listen:   "127.0.0.1",
		Port:     8080,
		Protocol: VMESS,
		Tag:      "custom",
	}
	cfg := in.GenXrayInboundConfig()
	expected := `"127.0.0.1"`
	if string(cfg.Listen) != expected {
		t.Errorf("Listen should be %s, got %s", expected, string(cfg.Listen))
	}
}

func TestGenXrayInboundConfig_EmptySettings(t *testing.T) {
	in := &Inbound{
		Port:     443,
		Protocol: Trojan,
	}
	cfg := in.GenXrayInboundConfig()
	if cfg == nil {
		t.Fatal("GenXrayInboundConfig should not return nil")
	}
	// Empty string Settings produces a nil RawMessage since json_util.RawMessage("") may be nil
	// Just verify no panic occurred
}
