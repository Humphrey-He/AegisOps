package host

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	alertsvc "github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	healthsvc "github.com/Humphrey-He/AegisOps/internal/healthcheck"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/notification"
	"github.com/Humphrey-He/AegisOps/internal/secret"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
)

func TestCreateDefaultsSSHPortTo22(t *testing.T) {
	database, secrets := openHostTestServices(t)
	service := NewService(database, secrets)

	secretItem, err := secrets.Create(context.Background(), secret.CreateRequest{
		Name:       "SSH Password",
		Type:       model.SecretTypeSSHPassword,
		Value:      "password123",
		OperatorID: "user-1",
	})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	hostItem, err := service.Create(context.Background(), CreateRequest{
		Name:        "prod-host",
		Address:     "127.0.0.1",
		SSHUser:     "root",
		SSHSecretID: secretItem.ID,
		OperatorID:  "user-1",
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}

	if hostItem.SSHPort != 22 {
		t.Fatalf("ssh port = %d, want 22", hostItem.SSHPort)
	}
	if hostItem.Status != model.HostStatusUnknown {
		t.Fatalf("host status = %s, want %s", hostItem.Status, model.HostStatusUnknown)
	}
}

func TestTestSSHTaskFailureRecordsTaskAvailabilityAndOfflineAlert(t *testing.T) {
	database, secrets := openHostTestServices(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, secrets)
	service.SetTaskService(tasks)

	notifications := notification.NewService(database)
	alerts := alertsvc.NewService(database, notifications)
	health := healthsvc.NewService(database, alerts)
	service.SetHealthCheckService(health)

	secretItem, err := secrets.Create(context.Background(), secret.CreateRequest{
		Name:       "Docker Token",
		Type:       model.SecretTypeDockerToken,
		Value:      "token-value",
		OperatorID: "user-1",
	})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	hostItem, err := service.Create(context.Background(), CreateRequest{
		Name:        "prod-host",
		Address:     "127.0.0.1",
		SSHPort:     22,
		SSHUser:     "root",
		SSHSecretID: secretItem.ID,
		OperatorID:  "user-1",
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}

	taskID, err := service.TestSSHTask(context.Background(), hostItem.ID, "user-1")
	if err == nil {
		t.Fatal("expected ssh test task to fail for unsupported secret type")
	}
	if !strings.Contains(err.Error(), "cannot be used for ssh") {
		t.Fatalf("ssh test error = %q, want unsupported secret type", err.Error())
	}
	if taskID == "" {
		t.Fatal("expected task id on failed ssh test")
	}

	taskItem, err := tasks.Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if taskItem.Status != model.TaskStatusFailed {
		t.Fatalf("task status = %s, want %s", taskItem.Status, model.TaskStatusFailed)
	}
	if taskItem.Error == "" {
		t.Fatal("expected failed task error message")
	}
	if len(taskItem.Logs) == 0 {
		t.Fatal("expected task logs for failed ssh test")
	}
	if taskItem.Logs[0].Level != model.TaskLogLevelError {
		t.Fatalf("task log level = %s, want %s", taskItem.Logs[0].Level, model.TaskLogLevelError)
	}

	var availability model.HostAvailabilityCheck
	if err := database.First(&availability, "host_id = ?", hostItem.ID).Error; err != nil {
		t.Fatalf("load host availability: %v", err)
	}
	if availability.Status != model.HostAvailabilityStatusUnreachable {
		t.Fatalf("availability status = %s, want %s", availability.Status, model.HostAvailabilityStatusUnreachable)
	}
	if !strings.Contains(availability.FailureReason, "cannot be used for ssh") {
		t.Fatalf("failure reason = %q, want ssh auth error", availability.FailureReason)
	}

	var alertEvent model.AlertEvent
	if err := database.First(&alertEvent, "event_type = ? AND resource_id = ?", "host_offline", hostItem.ID).Error; err != nil {
		t.Fatalf("load offline alert: %v", err)
	}
	if alertEvent.Status != model.AlertEventStatusOpen {
		t.Fatalf("offline alert status = %s, want %s", alertEvent.Status, model.AlertEventStatusOpen)
	}

	reloaded, err := service.Get(context.Background(), hostItem.ID)
	if err != nil {
		t.Fatalf("reload host: %v", err)
	}
	if reloaded.Status != model.HostStatusUnknown {
		t.Fatalf("host status = %s, want %s because ssh auth failed before dial", reloaded.Status, model.HostStatusUnknown)
	}
}

func openHostTestServices(t *testing.T) (*gorm.DB, *secret.Service) {
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
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	secrets, err := secret.NewService(database, "test-master-key")
	if err != nil {
		t.Fatalf("new secret service: %v", err)
	}
	return database, secrets
}
