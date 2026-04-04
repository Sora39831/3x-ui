// Package config provides configuration management utilities for the 3x-ui panel,
// including version information, logging levels, database paths, and environment variable handling.
package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed version
var version string

//go:embed name
var name string

// LogLevel represents the logging level for the application.
type LogLevel string

// Logging level constants
const (
	Debug   LogLevel = "debug"
	Info    LogLevel = "info"
	Notice  LogLevel = "notice"
	Warning LogLevel = "warning"
	Error   LogLevel = "error"
)

// GetVersion returns the version string of the 3x-ui application.
func GetVersion() string {
	return strings.TrimSpace(version)
}

// GetName returns the name of the 3x-ui application.
func GetName() string {
	return strings.TrimSpace(name)
}

// GetLogLevel returns the current logging level based on environment variables or defaults to Info.
func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("XUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

// IsDebug returns true if debug mode is enabled via the XUI_DEBUG environment variable.
func IsDebug() bool {
	return os.Getenv("XUI_DEBUG") == "true"
}

// GetBinFolderPath returns the path to the binary folder, defaulting to "bin" if not set via XUI_BIN_FOLDER.
func GetBinFolderPath() string {
	binFolderPath := os.Getenv("XUI_BIN_FOLDER")
	if binFolderPath == "" {
		binFolderPath = "bin"
	}
	return binFolderPath
}

func getBaseDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	exeDir := filepath.Dir(exePath)
	exeDirLower := strings.ToLower(filepath.ToSlash(exeDir))
	if strings.Contains(exeDirLower, "/appdata/local/temp/") || strings.Contains(exeDirLower, "/go-build") {
		wd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return wd
	}
	return exeDir
}

// GetDBFolderPath returns the path to the database folder based on environment variables or platform defaults.
func GetDBFolderPath() string {
	dbFolderPath := os.Getenv("XUI_DB_FOLDER")
	if dbFolderPath != "" {
		return dbFolderPath
	}
	if runtime.GOOS == "windows" {
		return getBaseDir()
	}
	return "/etc/x-ui"
}

// GetDBPath returns the full path to the database file.
func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

// GetSettingPath returns the full path to the panel settings JSON file.
func GetSettingPath() string {
	return fmt.Sprintf("%s/%s.json", GetDBFolderPath(), GetName())
}

// GetLogFolder returns the path to the log folder based on environment variables or platform defaults.
func GetLogFolder() string {
	logFolderPath := os.Getenv("XUI_LOG_FOLDER")
	if logFolderPath != "" {
		return logFolderPath
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(".", "log")
	}
	return "/var/log/x-ui"
}

var settingGroupAliases = map[string][]string{
	"dbType":     {"databaseConnection", "other"},
	"dbHost":     {"databaseConnection", "other"},
	"dbPort":     {"databaseConnection", "other"},
	"dbUser":     {"databaseConnection", "other"},
	"dbPassword": {"databaseConnection", "other"},
	"dbName":     {"databaseConnection", "other"},
}

func readGroupedString(settings map[string]any, key string) string {
	if groups, ok := settingGroupAliases[key]; ok {
		for _, groupName := range groups {
			if group, ok := settings[groupName].(map[string]any); ok {
				if value, ok := group[key].(string); ok && value != "" {
					return value
				}
			}
		}
	}
	if value, ok := settings[key].(string); ok && value != "" {
		return value
	}
	return ""
}

func settingsLayoutMeta() map[string]any {
	return map[string]any{
		"layout":      "按模块-用途来归类",
		"schema":      "module-purpose-v1",
		"description": "Top-level groups are organized by module and purpose for easier maintenance and development.",
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync()
}

func init() {
	if runtime.GOOS != "windows" {
		return
	}
	if os.Getenv("XUI_DB_FOLDER") != "" {
		return
	}
	oldDBFolder := "/etc/x-ui"
	oldDBPath := fmt.Sprintf("%s/%s.db", oldDBFolder, GetName())
	newDBFolder := GetDBFolderPath()
	newDBPath := fmt.Sprintf("%s/%s.db", newDBFolder, GetName())
	_, err := os.Stat(newDBPath)
	if err == nil {
		return // new exists
	}
	_, err = os.Stat(oldDBPath)
	if os.IsNotExist(err) {
		return // old does not exist
	}
	_ = copyFile(oldDBPath, newDBPath) // ignore error
}

// GetDBTypeFromJSON reads the dbType setting directly from the JSON config file.
// This is needed before the database is initialized. Falls back to "sqlite".
func GetDBTypeFromJSON() string {
	data, err := os.ReadFile(GetSettingPath())
	if err != nil {
		return "sqlite"
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return "sqlite"
	}

	if dbType := readGroupedString(settings, "dbType"); dbType != "" {
		return dbType
	}

	return "sqlite"
}

// DBConfig holds MariaDB connection settings read from the JSON config file.
type DBConfig struct {
	Type     string
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// GetDBConfigFromJSON reads all MariaDB connection settings from the JSON config file.
func GetDBConfigFromJSON() DBConfig {
	data, err := os.ReadFile(GetSettingPath())
	if err != nil {
		return DBConfig{Type: "sqlite", Host: "127.0.0.1", Port: "3306", Name: "3xui"}
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return DBConfig{Type: "sqlite", Host: "127.0.0.1", Port: "3306", Name: "3xui"}
	}

	dbType := "sqlite"
	if t := readGroupedString(settings, "dbType"); t != "" {
		dbType = t
	}

	return DBConfig{
		Type:     dbType,
		Host:     readGroupedString(settings, "dbHost"),
		Port:     readGroupedString(settings, "dbPort"),
		User:     readGroupedString(settings, "dbUser"),
		Password: readGroupedString(settings, "dbPassword"),
		Name:     readGroupedString(settings, "dbName"),
	}
}

// WriteSettingToJSON writes a single setting key to the JSON config file.
// It reads the existing file, updates the value, and writes back.
func WriteSettingToJSON(key, value string) error {
	path := GetSettingPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}
	if _, exists := settings["_meta"]; !exists {
		settings["_meta"] = settingsLayoutMeta()
	}

	// Check if the key lives in a nested group
	if groups, ok := settingGroupAliases[key]; ok && len(groups) > 0 {
		group := groups[0]
		if _, exists := settings[group]; !exists {
			settings[group] = make(map[string]any)
		}
		settings[group].(map[string]any)[key] = value
	} else {
		settings[key] = value
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
