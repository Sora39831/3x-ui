// Package database provides database initialization, migration, and management utilities
// for the 3x-ui panel using GORM with SQLite or MariaDB.
package database

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/xray"

	mysql2 "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

func initModels() error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

// initUser creates a default admin user if the users table is empty.
func initUser() error {
	empty, err := isTableEmpty("users")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}
	if empty {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)

		if err != nil {
			log.Printf("Error hashing default password: %v", err)
			return err
		}

		user := &model.User{
			Username: defaultUsername,
			Password: hashedPassword,
			Role:     "admin",
		}
		if err := db.Create(user).Error; err != nil {
			return err
		}

		// Mark password hashing seeder as done since initUser already uses bcrypt
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	}
	return nil
}

// runSeeders migrates user passwords to bcrypt and records seeder execution to prevent re-running.
func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	} else {
		var seedersHistory []string
		db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory)

		if !slices.Contains(seedersHistory, "UserPasswordHash") && !isUsersEmpty {
			var users []model.User
			db.Find(&users)

			for _, user := range users {
				hashedPassword, err := crypto.HashPasswordAsBcrypt(user.Password)
				if err != nil {
					log.Printf("Error hashing password for user '%s': %v", user.Username, err)
					return err
				}
				db.Model(&user).Update("password", hashedPassword)
			}

			hashSeeder := &model.HistoryOfSeeders{
				SeederName: "UserPasswordHash",
			}
			if err := db.Create(hashSeeder).Error; err != nil {
				return err
			}
		}

		if !slices.Contains(seedersHistory, "RemoveClientTrafficEmailUnique") {
			// Drop the old unique index on client_traffics.email to allow
			// the same email across multiple inbounds
			dbType := config.GetDBTypeFromJSON()
			if dbType == "mariadb" {
				db.Exec("DROP INDEX IF EXISTS idx_client_traffics_email ON client_traffics")
			} else {
				db.Exec("DROP INDEX IF EXISTS idx_client_traffics_email")
			}
			uniqueSeeder := &model.HistoryOfSeeders{
				SeederName: "RemoveClientTrafficEmailUnique",
			}
			if err := db.Create(uniqueSeeder).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// isTableEmpty returns true if the named table contains zero rows.
func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

// InitDB sets up the database connection, migrates models, and runs seeders.
// It reads the dbType from the JSON config to determine whether to use SQLite or MariaDB.
func InitDB() error {
	CloseDB() // close any existing connection before re-initializing

	dbType := config.GetDBTypeFromJSON()

	var err error
	switch dbType {
	case "mariadb":
		err = initMariaDB()
	default:
		err = initSQLite(config.GetDBPath())
	}
	if err != nil {
		return err
	}

	if err := initModels(); err != nil {
		return err
	}

	if err := initUser(); err != nil {
		return err
	}

	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
		return err
	}
	return runSeeders(isUsersEmpty)
}

// InitDBWithPath is a convenience function for tests and migrations that need
// to open a specific SQLite file.
func InitDBWithPath(dbPath string) error {
	CloseDB() // close any existing connection before re-initializing

	if err := initSQLite(dbPath); err != nil {
		return err
	}
	if err := initModels(); err != nil {
		return err
	}
	if err := initUser(); err != nil {
		return err
	}
	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
		return err
	}
	return runSeeders(isUsersEmpty)
}

// initSQLite opens a SQLite database connection and runs model migrations.
func initSQLite(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, fs.ModePerm)
	if err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	db, err = gorm.Open(sqlite.Open(dbPath), c)
	if err != nil {
		return err
	}

	return nil
}

// buildMariaDBDSN constructs a MariaDB DSN from the given config using
// go-sql-driver/mysql's Config to properly escape special characters in credentials.
func buildMariaDBDSN(dbConfig config.DBConfig) string {
	cfg := mysql2.Config{
		User:   dbConfig.User,
		Passwd: dbConfig.Password,
		Net:    "tcp",
		Addr:   dbConfig.Host + ":" + dbConfig.Port,
		DBName: dbConfig.Name,
		Params: map[string]string{
			"charset":   "utf8mb4",
			"parseTime": "True",
			"loc":       "Local",
		},
	}
	return cfg.FormatDSN()
}

// initMariaDB opens a MariaDB connection and runs model migrations.
func initMariaDB() error {
	dbConfig := config.GetDBConfigFromJSON()
	dsn := buildMariaDBDSN(dbConfig)

	var gormLogger logger.Interface
	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	var err error
	c := &gorm.Config{
		Logger: gormLogger,
	}
	db, err = gorm.Open(mysql.Open(dsn), c)
	if err != nil {
		return err
	}

	return nil
}

// CloseDB closes the database connection if it exists.
func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// GetDB returns the global GORM database instance.
func GetDB() *gorm.DB {
	return db
}

// IsNotFound checks if the given error is a GORM record not found error.
func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

// IsSQLiteDB checks if the given file is a valid SQLite database by reading its signature.
func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

// Checkpoint performs a WAL checkpoint on the SQLite database to ensure data consistency.
// For MariaDB, this is a no-op.
func Checkpoint() error {
	if config.GetDBTypeFromJSON() != "sqlite" {
		return nil
	}
	return db.Exec("PRAGMA wal_checkpoint;").Error
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil { // file must exist
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var res string
	if err := gdb.Raw("PRAGMA integrity_check;").Scan(&res).Error; err != nil {
		return err
	}
	if res != "ok" {
		return errors.New("sqlite integrity check failed: " + res)
	}
	return nil
}
