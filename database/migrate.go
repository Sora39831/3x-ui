package database

import (
	"fmt"
	"log"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// allModels returns the list of all model structs for migration.
func allModels() []any {
	return []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
	}
}

// tableNames returns the GORM table names for all models, in dependency order.
func tableNames() []string {
	return []string{
		"users",
		"inbounds",
		"outbound_traffics",
		"settings",
		"inbound_client_ips",
		"client_traffics",
		"history_of_seeders",
	}
}

// openSQLite opens a read-only SQLite connection for migration.
func openSQLite(dbPath string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
}

// openMariaDB opens a MariaDB connection for migration.
func openMariaDB() (*gorm.DB, error) {
	dbConfig := config.GetDBConfigFromJSON()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
}

// migrateTable copies all rows from src table to dst table using the given model slice.
// It returns the number of rows migrated.
func migrateTable[T any](src, dst *gorm.DB, tableName string) (int64, error) {
	var rows []T
	if err := src.Table(tableName).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("reading from %s: %w", tableName, err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := dst.Table(tableName).CreateInBatches(&rows, 100).Error; err != nil {
		return 0, fmt.Errorf("writing to %s: %w", tableName, err)
	}
	return int64(len(rows)), nil
}

// MigrateSQLiteToMariaDB copies all data from the SQLite database to MariaDB.
// The SQLite file is kept as a backup. The MariaDB tables are created via AutoMigrate first.
func MigrateSQLiteToMariaDB() error {
	srcDB, err := openSQLite(config.GetDBPath())
	if err != nil {
		return fmt.Errorf("failed to open SQLite source: %w", err)
	}

	dstDB, err := openMariaDB()
	if err != nil {
		return fmt.Errorf("failed to open MariaDB destination: %w", err)
	}

	// AutoMigrate all tables on destination
	for _, m := range allModels() {
		if err := dstDB.AutoMigrate(m); err != nil {
			return fmt.Errorf("failed to migrate table on MariaDB: %w", err)
		}
	}

	// Clear the seeder history so seeders run fresh on the new DB
	dstDB.Exec("DELETE FROM history_of_seeders")

	// Migrate each table
	total := int64(0)
	for _, name := range tableNames() {
		var count int64
		switch name {
		case "users":
			count, err = migrateTable[model.User](srcDB, dstDB, name)
		case "inbounds":
			count, err = migrateTable[model.Inbound](srcDB, dstDB, name)
		case "outbound_traffics":
			count, err = migrateTable[model.OutboundTraffics](srcDB, dstDB, name)
		case "settings":
			count, err = migrateTable[model.Setting](srcDB, dstDB, name)
		case "inbound_client_ips":
			count, err = migrateTable[model.InboundClientIps](srcDB, dstDB, name)
		case "client_traffics":
			count, err = migrateTable[xray.ClientTraffic](srcDB, dstDB, name)
		case "history_of_seeders":
			count, err = migrateTable[model.HistoryOfSeeders](srcDB, dstDB, name)
		}
		if err != nil {
			return fmt.Errorf("migration failed for %s: %w", name, err)
		}
		total += count
		log.Printf("Migrated %d rows from %s", count, name)
	}

	log.Printf("SQLite to MariaDB migration complete: %d total rows", total)
	return nil
}

// MigrateMariaDBToSQLite copies all data from MariaDB to the SQLite database.
// A new SQLite file is created (or overwritten) at the configured path.
func MigrateMariaDBToSQLite() error {
	srcDB, err := openMariaDB()
	if err != nil {
		return fmt.Errorf("failed to open MariaDB source: %w", err)
	}

	dstDB, err := openSQLite(config.GetDBPath())
	if err != nil {
		return fmt.Errorf("failed to open SQLite destination: %w", err)
	}

	// AutoMigrate all tables on destination
	for _, m := range allModels() {
		if err := dstDB.AutoMigrate(m); err != nil {
			return fmt.Errorf("failed to migrate table on SQLite: %w", err)
		}
	}

	// Clear the seeder history so seeders run fresh on the new DB
	dstDB.Exec("DELETE FROM history_of_seeders")

	// Migrate each table
	total := int64(0)
	for _, name := range tableNames() {
		var count int64
		switch name {
		case "users":
			count, err = migrateTable[model.User](srcDB, dstDB, name)
		case "inbounds":
			count, err = migrateTable[model.Inbound](srcDB, dstDB, name)
		case "outbound_traffics":
			count, err = migrateTable[model.OutboundTraffics](srcDB, dstDB, name)
		case "settings":
			count, err = migrateTable[model.Setting](srcDB, dstDB, name)
		case "inbound_client_ips":
			count, err = migrateTable[model.InboundClientIps](srcDB, dstDB, name)
		case "client_traffics":
			count, err = migrateTable[xray.ClientTraffic](srcDB, dstDB, name)
		case "history_of_seeders":
			count, err = migrateTable[model.HistoryOfSeeders](srcDB, dstDB, name)
		}
		if err != nil {
			return fmt.Errorf("migration failed for %s: %w", name, err)
		}
		total += count
		log.Printf("Migrated %d rows from %s", count, name)
	}

	log.Printf("MariaDB to SQLite migration complete: %d total rows", total)
	return nil
}
