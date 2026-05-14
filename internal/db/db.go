package db

import (
	"os"
	"path/filepath"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Driver == "" || cfg.Driver == "sqlite" {
		if err := os.MkdirAll(filepath.Dir(cfg.DSN), 0o755); err != nil {
			return nil, err
		}
		return gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{})
	}
	return nil, ErrUnsupportedDriver
}

func AutoMigrate(database *gorm.DB) error {
	return database.AutoMigrate(model.AllModels()...)
}
