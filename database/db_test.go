package database

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
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

	startSecond := time.Now().Unix()
	for time.Now().Unix() == startSecond {
		time.Sleep(10 * time.Millisecond)
	}

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

func TestRunSeeders_DoesNotRecordHistoryWhenPasswordUpdateFails(t *testing.T) {
	setupTestDB(t)

	if err := db.Exec("DELETE FROM history_of_seeders").Error; err != nil {
		t.Fatalf("clear seeders history failed: %v", err)
	}

	if err := db.Exec(`
		CREATE TRIGGER fail_user_password_update
		BEFORE UPDATE OF password ON users
		BEGIN
			SELECT RAISE(FAIL, 'boom');
		END;
	`).Error; err != nil {
		t.Fatalf("create trigger failed: %v", err)
	}

	err := runSeeders(false)
	if err == nil {
		t.Fatalf("expected runSeeders to fail when user password update fails")
	}

	var count int64
	if err := db.Model(&model.HistoryOfSeeders{}).
		Where("seeder_name = ?", "UserPasswordHash").
		Count(&count).Error; err != nil {
		t.Fatalf("count seeder history failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no UserPasswordHash history row after failed seeder, got %d", count)
	}
}

func TestSettingKey_IsUnique(t *testing.T) {
	setupTestDB(t)

	if err := db.Create(&model.Setting{Key: "dup", Value: "one"}).Error; err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	if err := db.Create(&model.Setting{Key: "dup", Value: "two"}).Error; err == nil {
		t.Fatal("expected duplicate setting key insert to fail")
	}
}

func TestInitDB_CreatesSharedMetadataTables(t *testing.T) {
	setupTestDB(t)

	for _, table := range []string{"shared_states", "node_states"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("table %s should exist: %v", table, err)
		}
	}
}

func TestBumpSharedAccountsVersion(t *testing.T) {
	setupTestDB(t)

	version, err := GetSharedAccountsVersion(GetDB())
	if err != nil {
		t.Fatalf("GetSharedAccountsVersion error: %v", err)
	}
	if version != 0 {
		t.Fatalf("expected seeded version 0, got %d", version)
	}

	tx := GetDB().Begin()
	if err := BumpSharedAccountsVersion(tx); err != nil {
		t.Fatalf("BumpSharedAccountsVersion error: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("Commit error: %v", err)
	}

	version, err = GetSharedAccountsVersion(GetDB())
	if err != nil {
		t.Fatalf("GetSharedAccountsVersion error: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected bumped version 1, got %d", version)
	}
}

func TestSeedSharedAccountsVersion_UsesPrimaryKeyLookup(t *testing.T) {
	var logs bytes.Buffer
	dryLogger := glogger.New(log.New(&logs, "", 0), glogger.Config{
		LogLevel: glogger.Info,
		Colorful: false,
	})

	dryDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DryRun: true,
		Logger: dryLogger,
	})
	if err != nil {
		t.Fatalf("open dry-run DB failed: %v", err)
	}

	if err := seedSharedAccountsVersion(dryDB); err != nil {
		t.Fatalf("seedSharedAccountsVersion error: %v", err)
	}

	sqlLogs := logs.String()
	if !strings.Contains(sqlLogs, "WHERE `shared_states`.`key` = \"shared_accounts_version\"") {
		t.Fatalf("expected primary-key lookup SQL, got logs:\n%s", sqlLogs)
	}
	if strings.Contains(sqlLogs, "WHERE key = \"shared_accounts_version\"") {
		t.Fatalf("expected seed query to avoid raw key lookup, got logs:\n%s", sqlLogs)
	}
}

func TestGetSharedAccountsVersion_UsesPrimaryKeyLookup(t *testing.T) {
	var logs bytes.Buffer
	dryLogger := glogger.New(log.New(&logs, "", 0), glogger.Config{
		LogLevel: glogger.Info,
		Colorful: false,
	})

	dryDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DryRun: true,
		Logger: dryLogger,
	})
	if err != nil {
		t.Fatalf("open dry-run DB failed: %v", err)
	}

	if _, err := GetSharedAccountsVersion(dryDB); err != nil {
		t.Fatalf("GetSharedAccountsVersion error: %v", err)
	}

	sqlLogs := logs.String()
	if !strings.Contains(sqlLogs, "WHERE `shared_states`.`key` = \"shared_accounts_version\"") {
		t.Fatalf("expected primary-key lookup SQL, got logs:\n%s", sqlLogs)
	}
	if strings.Contains(sqlLogs, "WHERE key = \"shared_accounts_version\"") {
		t.Fatalf("expected version lookup to avoid raw key lookup, got logs:\n%s", sqlLogs)
	}
}

func TestBumpSharedAccountsVersion_UsesQuotedKeyColumn(t *testing.T) {
	var logs bytes.Buffer
	dryLogger := glogger.New(log.New(&logs, "", 0), glogger.Config{
		LogLevel: glogger.Info,
		Colorful: false,
	})

	dryDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DryRun: true,
		Logger: dryLogger,
	})
	if err != nil {
		t.Fatalf("open dry-run DB failed: %v", err)
	}

	if err := BumpSharedAccountsVersion(dryDB); err != nil {
		t.Fatalf("BumpSharedAccountsVersion error: %v", err)
	}

	sqlLogs := logs.String()
	if !strings.Contains(sqlLogs, "WHERE `shared_states`.`key` = \"shared_accounts_version\"") {
		t.Fatalf("expected quoted key column in update SQL, got logs:\n%s", sqlLogs)
	}
	if strings.Contains(sqlLogs, "WHERE key = \"shared_accounts_version\"") {
		t.Fatalf("expected update SQL to avoid raw key lookup, got logs:\n%s", sqlLogs)
	}
}

func TestUpsertNodeState(t *testing.T) {
	setupTestDB(t)

	state := &model.NodeState{
		NodeID:          "worker-1",
		NodeRole:        "worker",
		LastSeenVersion: 7,
		LastError:       "dial tcp timeout",
	}
	if err := UpsertNodeState(GetDB(), state); err != nil {
		t.Fatalf("UpsertNodeState error: %v", err)
	}

	var stored model.NodeState
	if err := GetDB().First(&stored, "node_id = ?", "worker-1").Error; err != nil {
		t.Fatalf("lookup node state failed: %v", err)
	}
	if stored.LastSeenVersion != 7 {
		t.Fatalf("expected last seen version 7, got %d", stored.LastSeenVersion)
	}
	if stored.LastError != "dial tcp timeout" {
		t.Fatalf("expected last error to round-trip, got %q", stored.LastError)
	}
}
