package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	if v == "" {
		// version file might be empty in test, that's ok
		t.Log("version is empty (expected in test environment)")
	}
}

func TestGetName(t *testing.T) {
	n := GetName()
	if n == "" {
		t.Fatal("name should not be empty")
	}
	if strings.TrimSpace(n) != n {
		t.Error("name should be trimmed")
	}
}

func TestIsDebugDefault(t *testing.T) {
	t.Setenv("XUI_DEBUG", "")
	if IsDebug() {
		t.Error("IsDebug should return false by default")
	}
}

func TestIsDebugTrue(t *testing.T) {
	t.Setenv("XUI_DEBUG", "true")
	if !IsDebug() {
		t.Error("IsDebug should return true when XUI_DEBUG=true")
	}
}

func TestIsDebugFalse(t *testing.T) {
	t.Setenv("XUI_DEBUG", "false")
	if IsDebug() {
		t.Error("IsDebug should return false when XUI_DEBUG=false")
	}
}

func TestGetLogLevelDefault(t *testing.T) {
	t.Setenv("XUI_DEBUG", "")
	t.Setenv("XUI_LOG_LEVEL", "")
	if GetLogLevel() != Info {
		t.Errorf("default log level should be Info, got %s", GetLogLevel())
	}
}

func TestGetLogLevelDebug(t *testing.T) {
	t.Setenv("XUI_DEBUG", "true")
	if GetLogLevel() != Debug {
		t.Errorf("log level should be Debug when XUI_DEBUG=true, got %s", GetLogLevel())
	}
}

func TestGetLogLevelCustom(t *testing.T) {
	t.Setenv("XUI_DEBUG", "")
	t.Setenv("XUI_LOG_LEVEL", "warning")
	if GetLogLevel() != Warning {
		t.Errorf("log level should be Warning, got %s", GetLogLevel())
	}
}

func TestGetBinFolderPathDefault(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", "")
	if GetBinFolderPath() != "bin" {
		t.Errorf("default bin folder should be 'bin', got %s", GetBinFolderPath())
	}
}

func TestGetBinFolderPathCustom(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", "/custom/bin")
	if GetBinFolderPath() != "/custom/bin" {
		t.Errorf("bin folder should be '/custom/bin', got %s", GetBinFolderPath())
	}
}

func TestGetDBFolderPathDefault(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", "")
	folder := GetDBFolderPath()
	// On Linux without env var, should be "/etc/x-ui"
	if folder != "/etc/x-ui" {
		t.Errorf("default DB folder should be '/etc/x-ui', got %s", folder)
	}
}

func TestGetDBFolderPathCustom(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", "/tmp/test-db")
	if GetDBFolderPath() != "/tmp/test-db" {
		t.Errorf("DB folder should be '/tmp/test-db', got %s", GetDBFolderPath())
	}
}

func TestGetDBPath(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", "/tmp/test")
	dbPath := GetDBPath()
	expected := "/tmp/test/" + GetName() + ".db"
	if dbPath != expected {
		t.Errorf("GetDBPath() = %q, want %q", dbPath, expected)
	}
}

func TestGetSettingPath(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", "/tmp/test")
	settingPath := GetSettingPath()
	expected := "/tmp/test/" + GetName() + ".json"
	if settingPath != expected {
		t.Errorf("GetSettingPath() = %q, want %q", settingPath, expected)
	}
}

func TestGetLogFolderDefault(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", "")
	folder := GetLogFolder()
	if folder != "/var/log/x-ui" {
		t.Errorf("default log folder should be '/var/log/x-ui', got %s", folder)
	}
}

func TestGetLogFolderCustom(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", "/custom/logs")
	if GetLogFolder() != "/custom/logs" {
		t.Errorf("log folder should be '/custom/logs', got %s", GetLogFolder())
	}
}

func TestGetDBConfigFromJSONSupportsModulePurposeLayout(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	settings := map[string]any{
		"_meta": map[string]any{
			"layout": "按模块-用途来归类",
		},
		"databaseConnection": map[string]any{
			"dbType":     "mariadb",
			"dbHost":     "10.0.0.12",
			"dbPort":     "3307",
			"dbUser":     "panel",
			"dbPassword": "secret",
			"dbName":     "paneldb",
		},
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	if err := os.WriteFile(GetSettingPath(), data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	cfg := GetDBConfigFromJSON()
	if cfg.Type != "mariadb" || cfg.Host != "10.0.0.12" || cfg.Port != "3307" || cfg.User != "panel" || cfg.Password != "secret" || cfg.Name != "paneldb" {
		t.Fatalf("unexpected DB config: %+v", cfg)
	}
}

func TestWriteSettingToJSONUsesModulePurposeGroup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	initial := map[string]any{
		"_meta": map[string]any{
			"layout": "按模块-用途来归类",
		},
		"databaseConnection": map[string]any{},
	}
	data, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	if err := os.WriteFile(GetSettingPath(), data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := WriteSettingToJSON("dbHost", "127.0.0.2"); err != nil {
		t.Fatalf("WriteSettingToJSON error: %v", err)
	}

	updated, err := os.ReadFile(GetSettingPath())
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(updated, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	group, ok := parsed["databaseConnection"].(map[string]any)
	if !ok {
		t.Fatalf("expected databaseConnection group, got %T", parsed["databaseConnection"])
	}
	if got, ok := group["dbHost"].(string); !ok || got != "127.0.0.2" {
		t.Fatalf("expected databaseConnection.dbHost to be updated, got %v", group["dbHost"])
	}
}
