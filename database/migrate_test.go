package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTestSQLiteDB opens an in-memory or file-based SQLite database for testing.
func openTestSQLiteDB(t *testing.T, dbPath string) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("failed to open test SQLite DB: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := gdb.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return gdb
}

// createTestTables runs AutoMigrate on the given DB for all models.
func createTestTables(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
	}
	for _, m := range models {
		if err := gdb.AutoMigrate(m); err != nil {
			t.Fatalf("AutoMigrate failed: %v", err)
		}
	}
}

func TestMigrateAllTables_EmoprySource(t *testing.T) {
	srcDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "src.db"))
	dstDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "dst.db"))

	createTestTables(t, srcDB)
	createTestTables(t, dstDB)

	err := migrateAllTables(srcDB, dstDB)
	if err != nil {
		t.Fatalf("migrateAllTables on empty source should succeed: %v", err)
	}

	// Verify destination is still empty
	for _, name := range tableNames() {
		var count int64
		dstDB.Table(name).Count(&count)
		if count != 0 {
			t.Errorf("table %s should be empty, got %d rows", name, count)
		}
	}
}

func TestMigrateAllTables_WithData(t *testing.T) {
	srcDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "src.db"))
	dstDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "dst.db"))

	createTestTables(t, srcDB)
	createTestTables(t, dstDB)

	// Insert test data into source
	srcDB.Create(&model.User{Username: "testuser", Password: "testpass", Role: "admin"})
	srcDB.Create(&model.Setting{Key: "testkey", Value: "testvalue"})

	err := migrateAllTables(srcDB, dstDB)
	if err != nil {
		t.Fatalf("migrateAllTables failed: %v", err)
	}

	// Verify data was copied
	var userCount int64
	dstDB.Table("users").Count(&userCount)
	if userCount != 1 {
		t.Errorf("expected 1 user in dst, got %d", userCount)
	}

	var settingCount int64
	dstDB.Table("settings").Count(&settingCount)
	if settingCount != 1 {
		t.Errorf("expected 1 setting in dst, got %d", settingCount)
	}
}

func TestMigrateAllTables_OverwritesExisting(t *testing.T) {
	srcDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "src.db"))
	dstDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "dst.db"))

	createTestTables(t, srcDB)
	createTestTables(t, dstDB)

	// Insert existing data in destination that should be cleared
	dstDB.Create(&model.User{Username: "olduser", Password: "oldpass", Role: "admin"})
	dstDB.Create(&model.Setting{Key: "oldkey", Value: "oldvalue"})

	// Insert new data in source
	srcDB.Create(&model.User{Username: "newuser", Password: "newpass", Role: "admin"})

	err := migrateAllTables(srcDB, dstDB)
	if err != nil {
		t.Fatalf("migrateAllTables failed: %v", err)
	}

	// Verify old data was replaced
	var userCount int64
	dstDB.Table("users").Count(&userCount)
	if userCount != 1 {
		t.Errorf("expected 1 user in dst after overwrite, got %d", userCount)
	}

	var user model.User
	dstDB.Table("users").First(&user)
	if user.Username != "newuser" {
		t.Errorf("expected username 'newuser', got '%s'", user.Username)
	}

	// Settings should be empty since source has no settings
	var settingCount int64
	dstDB.Table("settings").Count(&settingCount)
	if settingCount != 0 {
		t.Errorf("expected 0 settings in dst after overwrite, got %d", settingCount)
	}
}

func TestMigrateTable_Generic(t *testing.T) {
	srcDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "src.db"))
	dstDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "dst.db"))

	createTestTables(t, srcDB)
	createTestTables(t, dstDB)

	// Insert test users
	srcDB.Create(&model.User{Username: "user1", Password: "pass1", Role: "admin"})
	srcDB.Create(&model.User{Username: "user2", Password: "pass2", Role: "admin"})

	count, err := migrateTable[model.User](srcDB, dstDB, "users")
	if err != nil {
		t.Fatalf("migrateTable failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows migrated, got %d", count)
	}

	var dstCount int64
	dstDB.Table("users").Count(&dstCount)
	if dstCount != 2 {
		t.Errorf("expected 2 users in dst, got %d", dstCount)
	}
}

func TestMigrateTable_EmptyTable(t *testing.T) {
	srcDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "src.db"))
	dstDB := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "dst.db"))

	createTestTables(t, srcDB)
	createTestTables(t, dstDB)

	count, err := migrateTable[model.User](srcDB, dstDB, "users")
	if err != nil {
		t.Fatalf("migrateTable on empty table should succeed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows migrated, got %d", count)
	}
}
