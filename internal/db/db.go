package db

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Driver == "" || cfg.Driver == "sqlite" {
		if err := ensureSQLiteDir(cfg.DSN); err != nil {
			return nil, err
		}

		dsn, err := withSQLitePragmas(cfg.DSN)
		if err != nil {
			return nil, err
		}

		database, err := gorm.Open(sqlite.Dialector{
			DriverName: "sqlite",
			DSN:        dsn,
		}, &gorm.Config{})
		if err != nil {
			return nil, err
		}

		sqlDB, err := database.DB()
		if err != nil {
			return nil, err
		}
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxIdleTime(0)
		sqlDB.SetConnMaxLifetime(0)

		return database, nil
	}

	if cfg.Driver == "postgres" || cfg.Driver == "postgresql" {
		database, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
		if err != nil {
			return nil, err
		}

		sqlDB, err := database.DB()
		if err != nil {
			return nil, err
		}
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(5)

		return database, nil
	}
	return nil, ErrUnsupportedDriver
}

func AutoMigrate(database *gorm.DB) error {
	return database.AutoMigrate(model.AllModels()...)
}

func ensureSQLiteDir(dsn string) error {
	path := sqlitePath(dsn)
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func sqlitePath(dsn string) string {
	if strings.HasPrefix(dsn, "file:") {
		return dsn
	}
	if idx := strings.IndexRune(dsn, '?'); idx >= 0 {
		return dsn[:idx]
	}
	return dsn
}

func withSQLitePragmas(dsn string) (string, error) {
	base := dsn
	rawQuery := ""
	if idx := strings.IndexRune(dsn, '?'); idx >= 0 {
		base = dsn[:idx]
		rawQuery = dsn[idx+1:]
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}

	addPragmaIfMissing(values, "busy_timeout", "busy_timeout(15000)")
	addPragmaIfMissing(values, "journal_mode", "journal_mode(WAL)")
	addPragmaIfMissing(values, "synchronous", "synchronous(NORMAL)")
	addPragmaIfMissing(values, "foreign_keys", "foreign_keys(ON)")

	encoded := values.Encode()
	if encoded == "" {
		return base, nil
	}
	return base + "?" + encoded, nil
}

func addPragmaIfMissing(values url.Values, prefix string, pragma string) {
	lowerPrefix := strings.ToLower(prefix)
	for _, value := range values["_pragma"] {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), lowerPrefix) {
			return
		}
	}
	values.Add("_pragma", pragma)
}
