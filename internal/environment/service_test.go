package environment

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"gorm.io/gorm"
)

func TestEnvironmentCreateNormalizesCodeAndDefaultsActive(t *testing.T) {
	database := openEnvironmentTestDB(t)
	service := NewService(database)

	item, err := service.Create(context.Background(), CreateRequest{
		Name: "Production",
		Code: " PROD ",
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if item.Code != "prod" {
		t.Fatalf("code = %q, want prod", item.Code)
	}
	if item.Status != model.EnvironmentStatusActive {
		t.Fatalf("status = %s, want %s", item.Status, model.EnvironmentStatusActive)
	}
}

func TestEnsureActiveRejectsMissingOrDisabledEnvironment(t *testing.T) {
	database := openEnvironmentTestDB(t)
	service := NewService(database)

	if _, err := EnsureActive(context.Background(), database, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("EnsureActive missing error = %v, want not found", err)
	}
	disabled, err := service.Create(context.Background(), CreateRequest{
		Name:   "Disabled",
		Code:   "disabled",
		Status: model.EnvironmentStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create disabled environment: %v", err)
	}
	if _, err := EnsureActive(context.Background(), database, disabled.Code); err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("EnsureActive disabled error = %v, want not active", err)
	}
}

func TestDeleteRejectsReferencedEnvironment(t *testing.T) {
	database := openEnvironmentTestDB(t)
	service := NewService(database)

	item, err := service.Create(context.Background(), CreateRequest{Name: "Production", Code: "prod"})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if err := database.Create(&model.DockerNode{
		ID:          "docker-1",
		Name:        "prod docker",
		Endpoint:    "mock://prod",
		Environment: item.Code,
	}).Error; err != nil {
		t.Fatalf("seed docker node: %v", err)
	}
	err = service.Delete(context.Background(), item.ID)
	if err == nil || !strings.Contains(err.Error(), "referenced") {
		t.Fatalf("delete referenced environment error = %v, want referenced", err)
	}
}

func openEnvironmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "aegisops.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("open sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return database
}
