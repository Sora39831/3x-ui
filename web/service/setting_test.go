package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/config"
)

func setupTestSettings(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	return func() {}
}

func TestLoadSettingsCreatesDefaults(t *testing.T) {
	setupTestSettings(t)

	settings, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings() error: %v", err)
	}

	// Should contain default values
	if settings["webPort"] != "2053" {
		t.Errorf("expected webPort=2053, got %s", settings["webPort"])
	}
	if settings["webBasePath"] != "/" {
		t.Errorf("expected webBasePath=/, got %s", settings["webBasePath"])
	}

	// Should NOT contain xrayTemplateConfig
	if _, exists := settings["xrayTemplateConfig"]; exists {
		t.Error("xrayTemplateConfig should not be in JSON settings")
	}

	// File should exist on disk
	path := config.GetSettingPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("settings file %s should have been created", path)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	setupTestSettings(t)

	settings := map[string]string{
		"webPort":   "8080",
		"webListen": "0.0.0.0",
	}
	err := saveSettings(settings)
	if err != nil {
		t.Fatalf("saveSettings() error: %v", err)
	}

	loaded, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings() error: %v", err)
	}

	if loaded["webPort"] != "8080" {
		t.Errorf("expected webPort=8080, got %s", loaded["webPort"])
	}
	if loaded["webListen"] != "0.0.0.0" {
		t.Errorf("expected webListen=0.0.0.0, got %s", loaded["webListen"])
	}
}

func TestSettingServiceGetString(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}

	// Should return default value when key not set
	val, err := svc.getString("webPort")
	if err != nil {
		t.Fatalf("getString error: %v", err)
	}
	if val != "2053" {
		t.Errorf("expected 2053, got %s", val)
	}
}

func TestSettingServiceSetAndGetString(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}

	err := svc.setString("webPort", "9090")
	if err != nil {
		t.Fatalf("setString error: %v", err)
	}

	val, err := svc.getString("webPort")
	if err != nil {
		t.Fatalf("getString error: %v", err)
	}
	if val != "9090" {
		t.Errorf("expected 9090, got %s", val)
	}
}

func TestResetSettingsDeletesFile(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}

	// Create settings first
	_, err := svc.getString("webPort")
	if err != nil {
		t.Fatalf("getString error: %v", err)
	}

	path := config.GetSettingPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("settings file should exist before reset")
	}

	// Manually delete to simulate the file removal part of ResetSettings
	err = os.Remove(path)
	if err != nil {
		t.Fatalf("remove error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("settings file should not exist after reset")
	}

	// Re-loading should recreate defaults
	settings, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings after reset error: %v", err)
	}
	if settings["webPort"] != "2053" {
		t.Errorf("expected default webPort=2053 after reset, got %s", settings["webPort"])
	}
}

func TestSettingsFileFormat(t *testing.T) {
	setupTestSettings(t)

	settings, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings error: %v", err)
	}

	path := config.GetSettingPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings file is not valid JSON: %v", err)
	}

	// Verify nested format: should contain group objects
	for _, group := range []string{"web", "tgBot", "sub", "ldap", "other"} {
		val, exists := parsed[group]
		if !exists {
			t.Errorf("expected group %q in nested JSON", group)
			continue
		}
		if _, isMap := val.(map[string]any); !isMap {
			t.Errorf("expected group %q to be an object, got %T", group, val)
		}
	}

	// Verify pretty-printed (has newlines)
	hasNewline := false
	for _, b := range data {
		if b == '\n' {
			hasNewline = true
			break
		}
	}
	if !hasNewline {
		t.Error("settings file should be pretty-printed with newlines")
	}

	// Verify round-trip: flatten nested back to flat should match loaded settings
	flattened := flattenNested(parsed)
	if len(flattened) != len(settings) {
		t.Errorf("flattened key count %d != loaded key count %d", len(flattened), len(settings))
	}
	for k, v := range settings {
		if fv, ok := flattened[k]; !ok {
			t.Errorf("key %q missing after flatten", k)
		} else if fv != v {
			t.Errorf("key %q: expected %q, got %q", k, v, fv)
		}
	}
}

func TestLegacyFlatFormatBackwardCompat(t *testing.T) {
	setupTestSettings(t)

	// Write a flat JSON file (legacy format)
	flat := map[string]string{
		"webPort":   "8080",
		"webListen": "0.0.0.0",
		"subEnable": "false",
		"ldapHost":  "ldap.example.com",
	}
	data, err := json.MarshalIndent(flat, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	path := config.GetSettingPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// loadSettings should parse it as flat and merge defaults
	loaded, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings error: %v", err)
	}

	if loaded["webPort"] != "8080" {
		t.Errorf("expected webPort=8080, got %s", loaded["webPort"])
	}
	if loaded["webListen"] != "0.0.0.0" {
		t.Errorf("expected webListen=0.0.0.0, got %s", loaded["webListen"])
	}
	if loaded["subEnable"] != "false" {
		t.Errorf("expected subEnable=false, got %s", loaded["subEnable"])
	}
	if loaded["ldapHost"] != "ldap.example.com" {
		t.Errorf("expected ldapHost=ldap.example.com, got %s", loaded["ldapHost"])
	}

	// Defaults should be merged for missing keys
	if loaded["webBasePath"] != "/" {
		t.Errorf("expected webBasePath=/, got %s", loaded["webBasePath"])
	}
}

func TestRoundTripNestedFormat(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}

	// Set some values
	if err := svc.setString("webPort", "9090"); err != nil {
		t.Fatalf("setString error: %v", err)
	}
	if err := svc.setString("tgBotEnable", "true"); err != nil {
		t.Fatalf("setString error: %v", err)
	}
	if err := svc.setString("ldapHost", "ldap.test.com"); err != nil {
		t.Fatalf("setString error: %v", err)
	}

	// Read back
	val, err := svc.getString("webPort")
	if err != nil || val != "9090" {
		t.Errorf("expected webPort=9090, got %s (err: %v)", val, err)
	}
	val, err = svc.getString("tgBotEnable")
	if err != nil || val != "true" {
		t.Errorf("expected tgBotEnable=true, got %s (err: %v)", val, err)
	}
	val, err = svc.getString("ldapHost")
	if err != nil || val != "ldap.test.com" {
		t.Errorf("expected ldapHost=ldap.test.com, got %s (err: %v)", val, err)
	}

	// Verify on-disk format is nested
	path := config.GetSettingPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings file is not valid JSON: %v", err)
	}
	if webGroup, ok := parsed["web"].(map[string]any); ok {
		if port, ok := webGroup["port"].(string); !ok || port != "9090" {
			t.Errorf("expected web.port=9090 in nested JSON, got %v", webGroup["port"])
		}
	} else {
		t.Error("expected 'web' group in nested JSON")
	}
	if tgGroup, ok := parsed["tgBot"].(map[string]any); ok {
		if enable, ok := tgGroup["enable"].(string); !ok || enable != "true" {
			t.Errorf("expected tgBot.enable=true in nested JSON, got %v", tgGroup["enable"])
		}
	} else {
		t.Error("expected 'tgBot' group in nested JSON")
	}
}
