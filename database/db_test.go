package database

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DEBUG", "")
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		CloseDB()
	})
}

func TestIsSQLiteDB_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "valid.db")
	if err := InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	f, err := os.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	ok, err := IsSQLiteDB(f)
	if err != nil {
		t.Fatalf("IsSQLiteDB error: %v", err)
	}
	if !ok {
		t.Error("IsSQLiteDB should return true for a valid SQLite file")
	}
}

func TestIsSQLiteDB_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	notADB := filepath.Join(tmpDir, "notdb.txt")
	if err := os.WriteFile(notADB, []byte("this is not a database"), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(notADB)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ok, err := IsSQLiteDB(f)
	if err != nil {
		t.Fatalf("IsSQLiteDB error: %v", err)
	}
	if ok {
		t.Error("IsSQLiteDB should return false for a non-SQLite file")
	}
}

func TestIsSQLiteDB_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	empty := filepath.Join(tmpDir, "empty.db")
	if err := os.WriteFile(empty, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(empty)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ok, err := IsSQLiteDB(f)
	// Empty file returns EOF since there aren't enough bytes to read
	if err == nil && ok {
		t.Error("IsSQLiteDB should return false for an empty file")
	}
}

func TestIsSQLiteDB_WrongContent(t *testing.T) {
	// File with 16 bytes (matching SQLite header length) but wrong signature content
	r := bytes.NewReader([]byte("SQLite for     !!"))
	ok, err := IsSQLiteDB(r)
	if err != nil {
		t.Fatalf("IsSQLiteDB error: %v", err)
	}
	if ok {
		t.Error("IsSQLiteDB should return false for wrong signature content")
	}
}

func TestInitDB_CreatesTables(t *testing.T) {
	setupTestDB(t)

	// Verify all tables exist by querying them
	tables := []string{"users", "inbounds", "outbound_traffics", "settings", "inbound_client_ips", "client_traffics", "history_of_seeders"}
	for _, table := range tables {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Errorf("table %q should exist but got error: %v", table, err)
		}
	}
}

func TestInitDB_CreatesDefaultUser(t *testing.T) {
	setupTestDB(t)

	var user model.User
	if err := db.First(&user).Error; err != nil {
		t.Fatalf("should have a default user: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("default username should be 'admin', got %q", user.Username)
	}
	if user.Role != "admin" {
		t.Errorf("default role should be 'admin', got %q", user.Role)
	}
	// Password should be a bcrypt hash, not plaintext
	if user.Password == "admin" {
		t.Error("default password should be hashed, not plaintext")
	}
}

func TestInitDB_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DEBUG", "")
	dbPath := filepath.Join(tmpDir, "idempotent.db")

	// First init
	if err := InitDBWithPath(dbPath); err != nil {
		t.Fatalf("first InitDB failed: %v", err)
	}
	CloseDB()

	// Second init on the same file should not fail
	if err := InitDBWithPath(dbPath); err != nil {
		t.Fatalf("second InitDB failed: %v", err)
	}
	defer CloseDB()

	// Should still have exactly one default user
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 user after second init, got %d", count)
	}
}

func TestValidateSQLiteDB_ValidDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "valid.db")
	if err := InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	CloseDB()

	if err := ValidateSQLiteDB(dbPath); err != nil {
		t.Errorf("ValidateSQLiteDB should pass for valid DB: %v", err)
	}
}

func TestValidateSQLiteDB_NonExistent(t *testing.T) {
	err := ValidateSQLiteDB("/tmp/does-not-exist-12345.db")
	if err == nil {
		t.Error("ValidateSQLiteDB should fail for non-existent file")
	}
}

func TestValidateSQLiteDB_CorruptDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "corrupt.db")
	// Write garbage that looks like SQLite header but is corrupt
	garbage := make([]byte, 4096)
	copy(garbage, []byte("SQLite format 3\x00"))
	if err := os.WriteFile(dbPath, garbage, 0644); err != nil {
		t.Fatal(err)
	}

	err := ValidateSQLiteDB(dbPath)
	if err == nil {
		t.Error("ValidateSQLiteDB should fail for corrupt DB")
	}
}

func TestIsNotFound(t *testing.T) {
	if IsNotFound(nil) {
		t.Error("IsNotFound should return false for nil")
	}
}

func TestInitUser_OnlyOnce(t *testing.T) {
	setupTestDB(t)

	// initUser should not create a second user when table is not empty
	if err := initUser(); err != nil {
		t.Fatalf("initUser error: %v", err)
	}

	var count int64
	db.Model(&model.User{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
}
