package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DEBUG", "")
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := database.InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		database.CloseDB()
	})
}

func TestGetFirstUser(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	user, err := svc.GetFirstUser()
	if err != nil {
		t.Fatalf("GetFirstUser error: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", user.Username)
	}
}

func TestCheckUser_ValidCredentials(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	user, err := svc.CheckUser("admin", "admin", "")
	if err != nil {
		t.Fatalf("CheckUser error: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", user.Username)
	}
}

func TestCheckUser_WrongPassword(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	_, err := svc.CheckUser("admin", "wrongpassword", "")
	if err == nil {
		t.Error("CheckUser should fail with wrong password")
	}
}

func TestCheckUser_NonExistentUser(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	_, err := svc.CheckUser("nonexistent", "password", "")
	if err == nil {
		t.Error("CheckUser should fail for non-existent user")
	}
}

func TestUpdateFirstUser(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	err := svc.UpdateFirstUser("newadmin", "newpassword123")
	if err != nil {
		t.Fatalf("UpdateFirstUser error: %v", err)
	}

	// Verify login with new credentials
	user, err := svc.CheckUser("newadmin", "newpassword123", "")
	if err != nil {
		t.Fatalf("CheckUser with new credentials error: %v", err)
	}
	if user.Username != "newadmin" {
		t.Errorf("expected username 'newadmin', got %q", user.Username)
	}
}

func TestUpdateFirstUser_EmptyUsername(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	err := svc.UpdateFirstUser("", "password")
	if err == nil {
		t.Error("UpdateFirstUser should fail with empty username")
	}
}

func TestUpdateFirstUser_EmptyPassword(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	err := svc.UpdateFirstUser("admin", "")
	if err == nil {
		t.Error("UpdateFirstUser should fail with empty password")
	}
}

func TestUpdateFirstUser_CreateWhenNone(t *testing.T) {
	// Use a fresh temp dir so no users table data exists
	tmpDir := t.TempDir()
	os.Setenv("XUI_DEBUG", "")
	os.Setenv("XUI_DB_FOLDER", tmpDir)
	defer func() {
		os.Unsetenv("XUI_DEBUG")
		os.Unsetenv("XUI_DB_FOLDER")
	}()

	dbPath := filepath.Join(tmpDir, "empty.db")
	if err := database.InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.CloseDB()

	// Delete all users to simulate empty table
	database.GetDB().Where("1 = 1").Delete(&model.User{})

	svc := &UserService{}
	err := svc.UpdateFirstUser("firstadmin", "firstpass")
	if err != nil {
		t.Fatalf("UpdateFirstUser should create user when table is empty: %v", err)
	}

	user, err := svc.GetFirstUser()
	if err != nil {
		t.Fatalf("GetFirstUser error: %v", err)
	}
	if user.Username != "firstadmin" {
		t.Errorf("expected username 'firstadmin', got %q", user.Username)
	}
	if !crypto.CheckPasswordHash(user.Password, "firstpass") {
		t.Error("password hash should match 'firstpass'")
	}
}
