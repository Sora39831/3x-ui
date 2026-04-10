package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
)

func writeNodeGuardSettings(t *testing.T, settings map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	if err := os.WriteFile(config.GetSettingPath(), data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func TestRequireMasterRejectsWorker(t *testing.T) {
	setupTestDB(t)
	writeNodeGuardSettings(t, map[string]any{
		"dbType":   "mariadb",
		"nodeRole": "worker",
		"nodeId":   "worker-1",
	})

	if err := RequireMaster(); err == nil {
		t.Fatal("expected worker mode to be rejected")
	}
}

func TestRequireMasterAllowsMaster(t *testing.T) {
	setupTestDB(t)
	writeNodeGuardSettings(t, map[string]any{
		"dbType":   "mariadb",
		"nodeRole": "master",
	})

	if err := RequireMaster(); err != nil {
		t.Fatalf("expected master mode to pass: %v", err)
	}
}

func TestBumpSharedAccountsVersionRollsBackWithTransaction(t *testing.T) {
	setupTestDB(t)

	tx := database.GetDB().Begin()
	if err := database.BumpSharedAccountsVersion(tx); err != nil {
		t.Fatalf("BumpSharedAccountsVersion error: %v", err)
	}
	tx.Rollback()

	version, err := database.GetSharedAccountsVersion(database.GetDB())
	if err != nil {
		t.Fatalf("GetSharedAccountsVersion error: %v", err)
	}
	if version != 0 {
		t.Fatalf("expected rolled-back version to remain 0, got %d", version)
	}
}
