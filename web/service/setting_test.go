package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
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

func TestSettingServiceSetAndGetWebDomain(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}

	if err := svc.SetWebDomain("panel.example.com"); err != nil {
		t.Fatalf("SetWebDomain error: %v", err)
	}

	val, err := svc.GetWebDomain()
	if err != nil {
		t.Fatalf("GetWebDomain error: %v", err)
	}
	if val != "panel.example.com" {
		t.Fatalf("expected panel.example.com, got %s", val)
	}
}

func TestSaveXrayTemplateConfigToDB_UpdatesSingleRow(t *testing.T) {
	setupTestSettings(t)
	setupTestDB(t)

	if err := saveXrayTemplateConfigToDB(`{"version":1}`); err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	if err := saveXrayTemplateConfigToDB(`{"version":2}`); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	var settings []model.Setting
	if err := database.GetDB().
		Where("key = ?", "xrayTemplateConfig").
		Find(&settings).Error; err != nil {
		t.Fatalf("query settings failed: %v", err)
	}
	if len(settings) != 1 {
		t.Fatalf("expected exactly one xrayTemplateConfig row, got %d", len(settings))
	}
	if settings[0].Value != `{"version":2}` {
		t.Fatalf("expected latest config value to be persisted, got %s", settings[0].Value)
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

	// Verify nested format: should contain group objects and metadata.
	for _, group := range []string{
		"_meta",
		"panelNetwork", "panelTLS", "panelSecurity", "panelUX",
		"telegramBot",
		"subscriptionNetwork", "subscriptionBranding", "subscriptionRouting",
		"ldapConnection", "ldapSync",
		"systemIntegration", "databaseConnection",
	} {
		val, exists := parsed[group]
		if !exists {
			t.Errorf("expected group %q in nested JSON", group)
			continue
		}
		if _, isMap := val.(map[string]any); !isMap {
			t.Errorf("expected group %q to be an object, got %T", group, val)
		}
	}
	if meta, ok := parsed["_meta"].(map[string]any); ok {
		if layout, ok := meta["layout"].(string); !ok || layout != "按模块-用途来归类" {
			t.Errorf("expected _meta.layout to explain grouping, got %v", meta["layout"])
		}
	} else {
		t.Error("expected _meta group in nested JSON")
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

func TestLegacyNestedFormatBackwardCompat(t *testing.T) {
	setupTestSettings(t)

	legacy := map[string]any{
		"web": map[string]any{
			"port":     "8088",
			"basePath": "/legacy/",
		},
		"tgBot": map[string]any{
			"enable": "true",
		},
		"other": map[string]any{
			"dbType": "mariadb",
			"dbHost": "192.168.1.10",
		},
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	path := config.GetSettingPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	loaded, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings error: %v", err)
	}
	if loaded["webPort"] != "8088" {
		t.Errorf("expected webPort=8088, got %s", loaded["webPort"])
	}
	if loaded["webBasePath"] != "/legacy/" {
		t.Errorf("expected webBasePath=/legacy/, got %s", loaded["webBasePath"])
	}
	if loaded["tgBotEnable"] != "true" {
		t.Errorf("expected tgBotEnable=true, got %s", loaded["tgBotEnable"])
	}
	if loaded["dbType"] != "mariadb" {
		t.Errorf("expected dbType=mariadb, got %s", loaded["dbType"])
	}
	if loaded["dbHost"] != "192.168.1.10" {
		t.Errorf("expected dbHost=192.168.1.10, got %s", loaded["dbHost"])
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
	if webGroup, ok := parsed["panelNetwork"].(map[string]any); ok {
		if port, ok := webGroup["port"].(string); !ok || port != "9090" {
			t.Errorf("expected panelNetwork.port=9090 in nested JSON, got %v", webGroup["port"])
		}
	} else {
		t.Error("expected 'panelNetwork' group in nested JSON")
	}
	if tgGroup, ok := parsed["telegramBot"].(map[string]any); ok {
		if enable, ok := tgGroup["enable"].(string); !ok || enable != "true" {
			t.Errorf("expected telegramBot.enable=true in nested JSON, got %v", tgGroup["enable"])
		}
	} else {
		t.Error("expected 'telegramBot' group in nested JSON")
	}
}

func TestUpdateAllSettingPreservesOmittedFields(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}
	if err := svc.setString("turnstileSiteKey", "site-key-123"); err != nil {
		t.Fatalf("setString turnstileSiteKey error: %v", err)
	}
	if err := svc.setString("dbType", "mariadb"); err != nil {
		t.Fatalf("setString dbType error: %v", err)
	}
	if err := svc.setString("dbHost", "10.0.0.8"); err != nil {
		t.Fatalf("setString dbHost error: %v", err)
	}

	allSetting, err := svc.GetAllSetting()
	if err != nil {
		t.Fatalf("GetAllSetting error: %v", err)
	}
	allSetting.WebPort = 9443

	presentKeys := map[string]struct{}{}
	for _, key := range []string{
		"webListen", "webDomain", "webPort", "webCertFile", "webKeyFile", "webBasePath", "sessionMaxAge",
		"pageSize", "expireDiff", "trafficDiff", "remarkModel", "datepicker",
		"tgBotEnable", "tgBotToken", "tgBotProxy", "tgBotAPIServer", "tgBotChatId", "tgRunTime", "tgBotBackup", "tgBotLoginNotify", "tgCpu", "tgLang",
		"timeLocation", "twoFactorEnable", "twoFactorToken",
		"subEnable", "subJsonEnable", "subTitle", "subSupportUrl", "subProfileUrl", "subAnnounce", "subEnableRouting", "subRoutingRules", "subListen", "subPort", "subPath", "subDomain", "subCertFile", "subKeyFile", "subUpdates", "externalTrafficInformEnable", "externalTrafficInformURI", "subEncrypt", "subShowInfo", "subURI", "subJsonPath", "subJsonURI", "subJsonFragment", "subJsonNoises", "subJsonMux", "subJsonRules",
		"ldapEnable", "ldapHost", "ldapPort", "ldapUseTLS", "ldapBindDN", "ldapPassword", "ldapBaseDN", "ldapUserFilter", "ldapUserAttr", "ldapVlessField", "ldapSyncCron", "ldapFlagField", "ldapTruthyValues", "ldapInvertFlag", "ldapInboundTags", "ldapAutoCreate", "ldapAutoDelete", "ldapDefaultTotalGB", "ldapDefaultExpiryDays", "ldapDefaultLimitIP",
	} {
		presentKeys[key] = struct{}{}
	}

	if err := svc.UpdateAllSetting(allSetting, presentKeys); err != nil {
		t.Fatalf("UpdateAllSetting error: %v", err)
	}

	settings, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings error: %v", err)
	}
	if got := settings["turnstileSiteKey"]; got != "site-key-123" {
		t.Fatalf("expected turnstileSiteKey to be preserved, got %q", got)
	}
	if got := settings["dbType"]; got != "mariadb" {
		t.Fatalf("expected dbType to be preserved, got %q", got)
	}
	if got := settings["dbHost"]; got != "10.0.0.8" {
		t.Fatalf("expected dbHost to be preserved, got %q", got)
	}
	if got := settings["webPort"]; got != "9443" {
		t.Fatalf("expected webPort to be updated, got %q", got)
	}
}
