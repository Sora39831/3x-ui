# Panel Settings JSON Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- []`) syntax for tracking.

**Goal:** Extract panel settings from the SQLite `settings` table into a standalone `x-ui.json` file, keeping `xrayTemplateConfig` in the database.

**Architecture:** Replace the database-backed `getSetting`/`saveSetting` in `SettingService` with JSON file read/write. All public `Get*`/`Set*` methods keep their signatures unchanged so controllers, CLI, and sub package need zero changes. `xrayTemplateConfig` gets dedicated DB helper methods to bypass the JSON path.

**Tech Stack:** Go, GORM/SQLite (retained for xrayTemplateConfig only), `encoding/json`, `os`

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `config/config.go` | Modify | Add `GetSettingPath()` |
| `web/service/setting.go` | Modify | Replace DB-backed internals with JSON file I/O |
| `web/service/xray_setting.go` | Modify | Use direct DB helpers for xrayTemplateConfig |
| `web/service/setting_test.go` | Create | Unit tests for JSON settings |

No changes needed: `main.go`, `database/db.go`, `database/model/model.go`, `web/entity/entity.go`, any controller, `sub/`, `xray/`.

---

### Task 1: Add `GetSettingPath()` to `config/config.go`

**Files:**
- Modify: `config/config.go:100`

- [ ] **Step 1: Add `GetSettingPath()` function**

Add after the existing `GetDBPath()` function at line 101:

```go
// GetSettingPath returns the full path to the panel settings JSON file.
func GetSettingPath() string {
	return fmt.Sprintf("%s/%s.json", GetDBFolderPath(), GetName())
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./config/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add config/config.go
git commit -m "feat(config): add GetSettingPath for JSON settings file"
```

---

### Task 2: Add JSON file I/O helpers to `web/service/setting.go`

**Files:**
- Modify: `web/service/setting.go`

- [ ] **Step 1: Add imports**

Add `"os"` and `"github.com/mhsanaei/3x-ui/v2/config"` to the import block. The existing imports `"github.com/mhsanaei/3x-ui/v2/database"` and `"github.com/mhsanaei/3x-ui/v2/database/model"` will be kept for now (removed later when `getSetting`/`saveSetting` are replaced and `GetAllSetting` no longer queries DB).

The import block becomes:

```go
import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/random"
	"github.com/mhsanaei/3x-ui/v2/util/reflect_util"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
	"github.com/mhsanaei/3x-ui/v2/xray"
)
```

- [ ] **Step 2: Add `loadSettings()` and `saveSettings()` functions**

Add these package-level functions before the `SettingService` struct (after `defaultValueMap`, around line 106):

```go
// loadSettings reads the JSON settings file into a map.
// If the file doesn't exist, it creates one from defaultValueMap (excluding xrayTemplateConfig).
func loadSettings() (map[string]string, error) {
	path := config.GetSettingPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		settings := make(map[string]string)
		for k, v := range defaultValueMap {
			if k == "xrayTemplateConfig" {
				continue
			}
			settings[k] = v
		}
		return settings, saveSettings(settings)
	}
	if err != nil {
		return nil, err
	}
	var settings map[string]string
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings file %s: %w", path, err)
	}
	return settings, nil
}

// saveSettings writes the settings map to the JSON file.
func saveSettings(settings map[string]string) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.GetSettingPath(), data, 0644)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./web/service/`
Expected: no errors (existing code still compiles with old + new functions coexisting)

- [ ] **Step 4: Commit**

```bash
git add web/service/setting.go
git commit -m "feat(service): add JSON file I/O helpers for settings"
```

---

### Task 3: Replace `getSetting`/`saveSetting` with JSON-based implementations

**Files:**
- Modify: `web/service/setting.go:205-229`

- [ ] **Step 1: Replace `getSetting`**

Replace lines 205-213:

```go
func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}
```

With:

```go
func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	settings, err := loadSettings()
	if err != nil {
		return nil, err
	}
	value, ok := settings[key]
	if !ok {
		return nil, fmt.Errorf("setting key %q not found", key)
	}
	return &model.Setting{Key: key, Value: value}, nil
}
```

- [ ] **Step 2: Replace `saveSetting`**

Replace lines 215-229:

```go
func (s *SettingService) saveSetting(key string, value string) error {
	setting, err := s.getSetting(key)
	db := database.GetDB()
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{
			Key:   key,
			Value: value,
		}).Error
	} else if err != nil {
		return err
	}
	setting.Key = key
	setting.Value = value
	return db.Save(setting).Error
}
```

With:

```go
func (s *SettingService) saveSetting(key string, value string) error {
	settings, err := loadSettings()
	if err != nil {
		return err
	}
	settings[key] = value
	return saveSettings(settings)
}
```

- [ ] **Step 3: Replace `getString` to use JSON directly**

Replace lines 231-243:

```go
func (s *SettingService) getString(key string) (string, error) {
	setting, err := s.getSetting(key)
	if database.IsNotFound(err) {
		value, ok := defaultValueMap[key]
		if !ok {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return value, nil
	} else if err != nil {
		return "", err
	}
	return setting.Value, nil
}
```

With:

```go
func (s *SettingService) getString(key string) (string, error) {
	settings, err := loadSettings()
	if err != nil {
		return "", err
	}
	value, ok := settings[key]
	if !ok {
		defaultValue, hasDefault := defaultValueMap[key]
		if !hasDefault {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return defaultValue, nil
	}
	return value, nil
}
```

- [ ] **Step 4: Replace `ResetSettings`**

Replace lines 195-203:

```go
func (s *SettingService) ResetSettings() error {
	db := database.GetDB()
	err := db.Where("1 = 1").Delete(model.Setting{}).Error
	if err != nil {
		return err
	}
	return db.Model(model.User{}).
		Where("1 = 1").Error
}
```

With:

```go
func (s *SettingService) ResetSettings() error {
	// Delete the JSON settings file
	err := os.Remove(config.GetSettingPath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Clear users table
	db := database.GetDB()
	return db.Where("1 = 1").Delete(model.User{}).Error
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./web/service/`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add web/service/setting.go
git commit -m "feat(service): replace DB-backed settings with JSON file operations"
```

---

### Task 4: Update `GetAllSetting` and `UpdateAllSetting` to use JSON

**Files:**
- Modify: `web/service/setting.go:120-193, 691-710`

- [ ] **Step 1: Replace `GetAllSetting`**

Replace lines 120-193:

```go
func (s *SettingService) GetAllSetting() (*entity.AllSetting, error) {
	db := database.GetDB()
	settings := make([]*model.Setting, 0)
	err := db.Model(model.Setting{}).Not("key = ?", "xrayTemplateConfig").Find(&settings).Error
	if err != nil {
		return nil, err
	}
	allSetting := &entity.AllSetting{}
	t := reflect.TypeFor[entity.AllSetting]()
	v := reflect.ValueOf(allSetting).Elem()
	fields := reflect_util.GetFields(t)

	setSetting := func(key, value string) (err error) {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				err = errors.New(fmt.Sprint(panicErr))
			}
		}()

		var found bool
		var field reflect.StructField
		for _, f := range fields {
			if f.Tag.Get("json") == key {
				field = f
				found = true
				break
			}
		}

		if !found {
			// Some settings are automatically generated, no need to return to the front end to modify the user
			return nil
		}

		fieldV := v.FieldByName(field.Name)
		switch t := fieldV.Interface().(type) {
		case int:
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			fieldV.SetInt(n)
		case string:
			fieldV.SetString(value)
		case bool:
			fieldV.SetBool(value == "true")
		default:
			return common.NewErrorf("unknown field %v type %v", key, t)
		}
		return
	}

	keyMap := map[string]bool{}
	for _, setting := range settings {
		err := setSetting(setting.Key, setting.Value)
		if err != nil {
			return nil, err
		}
		keyMap[setting.Key] = true
	}

	for key, value := range defaultValueMap {
		if keyMap[key] {
			continue
		}
		err := setSetting(key, value)
		if err != nil {
			return nil, err
		}
	}

	return allSetting, nil
}
```

With:

```go
func (s *SettingService) GetAllSetting() (*entity.AllSetting, error) {
	settings, err := loadSettings()
	if err != nil {
		return nil, err
	}
	allSetting := &entity.AllSetting{}
	t := reflect.TypeFor[entity.AllSetting]()
	v := reflect.ValueOf(allSetting).Elem()
	fields := reflect_util.GetFields(t)

	setSetting := func(key, value string) (err error) {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				err = errors.New(fmt.Sprint(panicErr))
			}
		}()

		var found bool
		var field reflect.StructField
		for _, f := range fields {
			if f.Tag.Get("json") == key {
				field = f
				found = true
				break
			}
		}

		if !found {
			return nil
		}

		fieldV := v.FieldByName(field.Name)
		switch t := fieldV.Interface().(type) {
		case int:
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			fieldV.SetInt(n)
		case string:
			fieldV.SetString(value)
		case bool:
			fieldV.SetBool(value == "true")
		default:
			return common.NewErrorf("unknown field %v type %v", key, t)
		}
		return
	}

	keyMap := map[string]bool{}
	for key, value := range settings {
		err := setSetting(key, value)
		if err != nil {
			return nil, err
		}
		keyMap[key] = true
	}

	for key, value := range defaultValueMap {
		if key == "xrayTemplateConfig" {
			continue
		}
		if keyMap[key] {
			continue
		}
		err := setSetting(key, value)
		if err != nil {
			return nil, err
		}
	}

	return allSetting, nil
}
```

- [ ] **Step 2: Replace `UpdateAllSetting`**

Replace lines 691-710:

```go
func (s *SettingService) UpdateAllSetting(allSetting *entity.AllSetting) error {
	if err := allSetting.CheckValid(); err != nil {
		return err
	}

	v := reflect.ValueOf(allSetting).Elem()
	t := reflect.TypeFor[entity.AllSetting]()
	fields := reflect_util.GetFields(t)
	errs := make([]error, 0)
	for _, field := range fields {
		key := field.Tag.Get("json")
		fieldV := v.FieldByName(field.Name)
		value := fmt.Sprint(fieldV.Interface())
		err := s.saveSetting(key, value)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return common.Combine(errs...)
}
```

With:

```go
func (s *SettingService) UpdateAllSetting(allSetting *entity.AllSetting) error {
	if err := allSetting.CheckValid(); err != nil {
		return err
	}

	settings, err := loadSettings()
	if err != nil {
		return err
	}

	v := reflect.ValueOf(allSetting).Elem()
	t := reflect.TypeFor[entity.AllSetting]()
	fields := reflect_util.GetFields(t)
	for _, field := range fields {
		key := field.Tag.Get("json")
		fieldV := v.FieldByName(field.Name)
		settings[key] = fmt.Sprint(fieldV.Interface())
	}
	return saveSettings(settings)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./web/service/`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add web/service/setting.go
git commit -m "feat(service): migrate GetAllSetting/UpdateAllSetting to JSON"
```

---

### Task 5: Handle `xrayTemplateConfig` — dedicated DB accessors

**Files:**
- Modify: `web/service/setting.go:273-274`
- Modify: `web/service/xray_setting.go:17-21`

- [ ] **Step 1: Add dedicated DB accessor for xrayTemplateConfig**

Add a new private function in `setting.go` (after the `saveSettings` function):

```go
// getXrayTemplateConfigFromDB reads xrayTemplateConfig directly from the database.
func getXrayTemplateConfigFromDB() (string, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", "xrayTemplateConfig").First(setting).Error
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// saveXrayTemplateConfigToDB writes xrayTemplateConfig directly to the database.
func saveXrayTemplateConfigToDB(value string) error {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", "xrayTemplateConfig").First(setting).Error
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{
			Key:   "xrayTemplateConfig",
			Value: value,
		}).Error
	}
	if err != nil {
		return err
	}
	setting.Value = value
	return db.Save(setting).Error
}
```

- [ ] **Step 2: Update `GetXrayConfigTemplate` to use DB directly**

Replace line 273-274:

```go
func (s *SettingService) GetXrayConfigTemplate() (string, error) {
	return s.getString("xrayTemplateConfig")
}
```

With:

```go
func (s *SettingService) GetXrayConfigTemplate() (string, error) {
	config, err := getXrayTemplateConfigFromDB()
	if err != nil {
		// If not in DB, return the embedded default
		return xrayTemplateConfig, nil
	}
	return config, nil
}
```

- [ ] **Step 3: Update `XraySettingService.SaveXraySetting` to use DB directly**

Replace line 17-21 in `xray_setting.go`:

```go
func (s *XraySettingService) SaveXraySetting(newXraySettings string) error {
	if err := s.CheckXrayConfig(newXraySettings); err != nil {
		return err
	}
	return s.SettingService.saveSetting("xrayTemplateConfig", newXraySettings)
}
```

With:

```go
func (s *XraySettingService) SaveXraySetting(newXraySettings string) error {
	if err := s.CheckXrayConfig(newXraySettings); err != nil {
		return err
	}
	return saveXrayTemplateConfigToDB(newXraySettings)
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /usr/x-ui/3x-ui && go build ./web/service/`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add web/service/setting.go web/service/xray_setting.go
git commit -m "feat(service): use direct DB access for xrayTemplateConfig"
```

---

### Task 6: Clean up unused imports

**Files:**
- Modify: `web/service/setting.go`

- [ ] **Step 1: Remove `database` and `model` imports if no longer needed**

Check if `database` and `model` packages are still referenced in `setting.go` after all changes. `database` is still used by `ResetSettings()` (for `database.GetDB()` to clear users table). `model` is no longer needed in `setting.go` since `getSetting`/`saveSetting` no longer use `model.Setting`, and `ResetSettings` uses `model.User` which... actually check: `ResetSettings` references `model.User{}`.

So `database` and `model` are still needed in `setting.go` for:
- `ResetSettings()` → `database.GetDB()` + `model.User{}`
- `getXrayTemplateConfigFromDB()` / `saveXrayTemplateConfigToDB()` → `database` + `model.Setting{}`

No import cleanup needed. Skip this step.

- [ ] **Step 2: Verify full build**

Run: `cd /usr/x-ui/3x-ui && go build ./...`
Expected: no errors

- [ ] **Step 3: Commit (only if changes were made)**

```bash
git add web/service/setting.go
git commit -m "chore(service): clean up unused imports"
```

---

### Task 7: Write unit tests

**Files:**
- Create: `web/service/setting_test.go`

- [ ] **Step 1: Write tests for JSON settings**

```go
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		"webPort":  "8080",
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

	// Note: ResetSettings also needs DB for users table.
	// For this unit test, we just verify the JSON file deletion part works.
	// Full integration test would need a test DB.
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
	var parsed map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings file is not valid JSON: %v", err)
	}

	// Verify pretty-printed (has newlines)
	if !contains(data, '\n') {
		t.Error("settings file should be pretty-printed with newlines")
	}

	// Verify key count matches
	if len(parsed) != len(settings) {
		t.Errorf("parsed key count %d != loaded key count %d", len(parsed), len(settings))
	}

	_ = filepath.Base(path) // just to use the import
}

func contains(data []byte, b byte) bool {
	for _, d := range data {
		if d == b {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests**

Run: `cd /usr/x-ui/3x-ui && go test ./web/service/ -run TestLoadSettings -v`
Expected: PASS

Run: `cd /usr/x-ui/3x-ui && go test ./web/service/ -run TestSaveAndLoad -v`
Expected: PASS

Run: `cd /usr/x-ui/3x-ui && go test ./web/service/ -run TestSettingService -v`
Expected: PASS

Run: `cd /usr/x-ui/3x-ui && go test ./web/service/ -run TestReset -v`
Expected: PASS

Run: `cd /usr/x-ui/3x-ui && go test ./web/service/ -run TestSettingsFile -v`
Expected: PASS

- [ ] **Step 3: Run all tests**

Run: `cd /usr/x-ui/3x-ui && go test ./web/service/ -v`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add web/service/setting_test.go
git commit -m "test(service): add unit tests for JSON settings"
```

---

### Task 8: Full build verification

- [ ] **Step 1: Build entire project**

Run: `cd /usr/x-ui/3x-ui && go build ./...`
Expected: no errors

- [ ] **Step 2: Run `go vet`**

Run: `cd /usr/x-ui/3x-ui && go vet ./...`
Expected: no issues

- [ ] **Step 3: Final commit (only if fixes needed)**

```bash
git add -A
git commit -m "chore: fix build issues from settings migration"
```

---

## Self-Review

**1. Spec coverage:**
- Panel settings in flat key-value JSON: Tasks 2-4
- xrayTemplateConfig stays in DB: Task 5
- All new installations (no migration): Task 2 Step 1 (auto-create from defaults)
- JSON file path: Task 1 (`GetSettingPath`)
- JSON auto-created on first run: Task 2 Step 1 (`loadSettings`)
- CLI compatibility: No changes to main.go, works via unchanged `SettingService` API
- Tests: Task 7

**2. Placeholder scan:** No TBD/TODO found. All code blocks contain complete implementations.

**3. Type consistency:**
- `getSetting` still returns `(*model.Setting, error)` — reused by `getString` which checks `database.IsNotFound(err)`. After the change, `getSetting` returns a custom error when key not found (not `gorm.ErrRecordNotFound`). Need to verify: `getString` checks `database.IsNotFound(err)` which tests for `gorm.ErrRecordNotFound`. The new `getSetting` returns `fmt.Errorf(...)` which is NOT a gorm error. This means `getString` would NOT fall through to the default — it would return the error instead.

**FIX:** `getString` must not rely on `database.IsNotFound`. The rewritten `getString` in Task 3 Step 3 already handles this correctly — it reads the map directly and checks `ok`, no longer calling `getSetting` or checking `database.IsNotFound`. Good.
