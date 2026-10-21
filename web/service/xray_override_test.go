package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

func setupOverrideTest(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DEBUG", "")
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	t.Cleanup(func() {
		overridePath := config.GetXrayOverridePath()
		os.Remove(overridePath)
	})
}

func TestMergeXrayConfig_EmptyOverride(t *testing.T) {
	_, err := mergeXrayConfig(`{"log":{"loglevel":"info"}}`, "")
	if err == nil {
		t.Error("expected error for empty override string")
	}
}

func TestMergeXrayConfig_SingleKeyOverride(t *testing.T) {
	base := `{"log":{"loglevel":"info"},"routing":{}}`
	override := `{"log":{"loglevel":"debug"}}`
	result, err := mergeXrayConfig(base, override)
	if err != nil {
		t.Fatalf("mergeXrayConfig error: %v", err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal result error: %v", err)
	}
	// log should be fully replaced
	if string(parsed["log"]) != `{"loglevel":"debug"}` {
		t.Errorf("expected log to be replaced, got %s", string(parsed["log"]))
	}
	// routing should be kept
	if string(parsed["routing"]) != `{}` {
		t.Errorf("expected routing to remain, got %s", string(parsed["routing"]))
	}
}

func TestMergeXrayConfig_MultipleKeysOverride(t *testing.T) {
	base := `{"log":{},"api":{},"routing":{}}`
	override := `{"log":{"loglevel":"debug"},"routing":{"domainStrategy":"IPIfNonMatch"}}`
	result, err := mergeXrayConfig(base, override)
	if err != nil {
		t.Fatalf("mergeXrayConfig error: %v", err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal result error: %v", err)
	}
	if _, exists := parsed["api"]; !exists {
		t.Error("expected api key to remain")
	}
}

func TestMergeXrayConfig_FullOverride(t *testing.T) {
	base := `{"log":{},"api":{}}`
	override := `{"log":{"loglevel":"debug"},"api":{},"dns":{}}`
	result, err := mergeXrayConfig(base, override)
	if err != nil {
		t.Fatalf("mergeXrayConfig error: %v", err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal result error: %v", err)
	}
	if _, exists := parsed["dns"]; !exists {
		t.Error("expected dns key from override to be added")
	}
}

func TestMergeXrayConfig_InvalidBase(t *testing.T) {
	_, err := mergeXrayConfig("not json", `{}`)
	if err == nil {
		t.Error("expected error for invalid base JSON")
	}
}

func TestMergeXrayConfig_InvalidOverride(t *testing.T) {
	_, err := mergeXrayConfig(`{}`, "not json")
	if err == nil {
		t.Error("expected error for invalid override JSON")
	}
}

func TestMergeXrayConfig_BothEmpty(t *testing.T) {
	_, err := mergeXrayConfig("", "")
	if err == nil {
		t.Error("expected error for empty base and override")
	}
}

func TestSaveAndLoadOverride(t *testing.T) {
	setupOverrideTest(t)
	content := `{"log":{"loglevel":"debug"}}`
	if err := saveXrayTemplateOverrideToFile(content); err != nil {
		t.Fatalf("save error: %v", err)
	}
	loaded, err := getXrayTemplateOverrideFromFile()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded != content {
		t.Errorf("expected %s, got %s", content, loaded)
	}
}

func TestGetOverrideNonExistent(t *testing.T) {
	setupOverrideTest(t)
	result, err := getXrayTemplateOverrideFromFile()
	if err != nil {
		t.Fatalf("getXrayTemplateOverrideFromFile error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for non-existent file, got %s", result)
	}
}

func TestGetOverrideEmptyFile(t *testing.T) {
	setupOverrideTest(t)
	if err := os.WriteFile(config.GetXrayOverridePath(), []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	result, err := getXrayTemplateOverrideFromFile()
	if err != nil {
		t.Fatalf("getXrayTemplateOverrideFromFile error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for empty file, got %s", result)
	}
}

func TestSaveOverrideOverwrites(t *testing.T) {
	setupOverrideTest(t)
	if err := saveXrayTemplateOverrideToFile(`{"first":true}`); err != nil {
		t.Fatalf("first save error: %v", err)
	}
	if err := saveXrayTemplateOverrideToFile(`{"second":true}`); err != nil {
		t.Fatalf("second save error: %v", err)
	}
	loaded, err := getXrayTemplateOverrideFromFile()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded != `{"second":true}` {
		t.Errorf("expected override to be overwritten, got %s", loaded)
	}
}

func TestSaveXrayOverride_ValidJSON(t *testing.T) {
	setupOverrideTest(t)
	svc := &XraySettingService{}
	if err := svc.SaveXrayOverride(`{"log":{}}`); err != nil {
		t.Fatalf("SaveXrayOverride error: %v", err)
	}
	loaded, err := getXrayTemplateOverrideFromFile()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded != `{"log":{}}` {
		t.Errorf("expected {\"log\":{}}, got %s", loaded)
	}
}

func TestSaveXrayOverride_InvalidJSON(t *testing.T) {
	setupOverrideTest(t)
	svc := &XraySettingService{}
	if err := svc.SaveXrayOverride("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSaveXrayOverride_Empty(t *testing.T) {
	setupOverrideTest(t)
	svc := &XraySettingService{}
	if err := svc.SaveXrayOverride(""); err != nil {
		t.Fatalf("SaveXrayOverride with empty string should succeed: %v", err)
	}
	loaded, err := getXrayTemplateOverrideFromFile()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded != "" {
		t.Errorf("expected empty override, got %s", loaded)
	}
}

func TestGetXrayOverride_Empty(t *testing.T) {
	setupOverrideTest(t)
	svc := &XraySettingService{}
	result, err := svc.GetXrayOverride()
	if err != nil {
		t.Fatalf("GetXrayOverride error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty override, got %s", result)
	}
}

func TestXrayOverrideRoundTrip(t *testing.T) {
	setupOverrideTest(t)
	svc := &XraySettingService{}
	content := `{"log":{"loglevel":"info"},"routing":{"domainStrategy":"AsIs"}}`
	if err := svc.SaveXrayOverride(content); err != nil {
		t.Fatalf("SaveXrayOverride error: %v", err)
	}
	loaded, err := svc.GetXrayOverride()
	if err != nil {
		t.Fatalf("GetXrayOverride error: %v", err)
	}
	if loaded != content {
		t.Errorf("round-trip failed: expected %s, got %s", content, loaded)
	}
}

func TestBuildConfigFromInboundsWithOverride(t *testing.T) {
	setupOverrideTest(t)
	setupTestDB(t)

	// Write template config to DB
	if err := database.GetDB().Create(&model.Setting{
		Key:   "xrayTemplateConfig",
		Value: `{"log":{"loglevel":"info"},"inbounds":[{"tag":"api","listen":"127.0.0.1","port":62789,"protocol":"tunnel","settings":{"address":"127.0.0.1"}}],"outbounds":[{"tag":"direct","protocol":"freedom"}],"routing":{"domainStrategy":"AsIs"}}`,
	}).Error; err != nil {
		t.Fatalf("create template config error: %v", err)
	}

	// Write override that changes log level
	if err := saveXrayTemplateOverrideToFile(`{"log":{"loglevel":"debug"}}`); err != nil {
		t.Fatalf("save override error: %v", err)
	}

	xrayService := &XrayService{}

	config, err := xrayService.BuildConfigFromInbounds(nil)
	if err != nil {
		t.Fatalf("BuildConfigFromInbounds error: %v", err)
	}
	if string(config.LogConfig) != `{"loglevel":"debug"}` {
		t.Errorf("expected log config to be overridden to debug, got %s", string(config.LogConfig))
	}
	if string(config.RouterConfig) != `{"domainStrategy":"AsIs"}` {
		t.Errorf("expected routing config to be base value, got %s", string(config.RouterConfig))
	}
}

func TestBuildConfigFromInboundsWithoutOverride(t *testing.T) {
	setupOverrideTest(t)
	setupTestDB(t)

	if err := database.GetDB().Create(&model.Setting{
		Key:   "xrayTemplateConfig",
		Value: `{"log":{"loglevel":"info"},"inbounds":[{"tag":"api","listen":"127.0.0.1","port":62789,"protocol":"tunnel","settings":{"address":"127.0.0.1"}}],"outbounds":[]}`,
	}).Error; err != nil {
		t.Fatalf("create template config error: %v", err)
	}

	xrayService := &XrayService{}

	config, err := xrayService.BuildConfigFromInbounds(nil)
	if err != nil {
		t.Fatalf("BuildConfigFromInbounds error: %v", err)
	}
	if string(config.LogConfig) != `{"loglevel":"info"}` {
		t.Errorf("expected log config to remain info, got %s", string(config.LogConfig))
	}
}
