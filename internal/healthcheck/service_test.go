package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	alertsvc "github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/notification"
)

func TestRunServiceCheckSuccessPersistsResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	database := openHealthTestDB(t)
	service := newHealthService(database)

	definition := model.ServiceDefinition{
		ID:   "svc-1",
		Code: "demo-api",
		Envs: healthPolicyEnvJSON(server.URL, http.StatusOK),
	}
	release := model.ServiceReleaseRecord{ID: "release-1"}

	check, err := service.RunServiceCheck(context.Background(), definition, release, "task-1")
	if err != nil {
		t.Fatalf("run service check: %v", err)
	}

	if check.Status != model.HealthCheckStatusSuccess {
		t.Fatalf("status = %s, want %s", check.Status, model.HealthCheckStatusSuccess)
	}
	if check.HTTPStatus != http.StatusOK {
		t.Fatalf("http status = %d, want %d", check.HTTPStatus, http.StatusOK)
	}
	if !strings.Contains(check.Output, "http status 200") {
		t.Fatalf("output = %q, want success message", check.Output)
	}

	var count int64
	if err := database.Model(&model.AlertEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count alert events: %v", err)
	}
	if count != 0 {
		t.Fatalf("alert event count = %d, want 0", count)
	}
}

func TestRunServiceCheckFailureCreatesAlertWithRollbackSuggestion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	database := openHealthTestDB(t)
	service := newHealthService(database)

	definition := model.ServiceDefinition{
		ID:             "svc-1",
		Code:           "demo-api",
		CurrentVersion: "2.0.0",
		Envs:           healthPolicyEnvJSON(server.URL, http.StatusOK),
	}
	if err := database.Create(&definition).Error; err != nil {
		t.Fatalf("seed service: %v", err)
	}
	previous := model.ServiceVersion{
		ID:        "ver-prev",
		ServiceID: definition.ID,
		Version:   "1.0.0",
		Image:     "registry.local/demo-api",
		ImageTag:  "1.0.0",
	}
	current := model.ServiceVersion{
		ID:        "ver-current",
		ServiceID: definition.ID,
		Version:   "2.0.0",
		Image:     "registry.local/demo-api",
		ImageTag:  "2.0.0",
	}
	if err := database.Create(&previous).Error; err != nil {
		t.Fatalf("seed previous version: %v", err)
	}
	if err := database.Create(&current).Error; err != nil {
		t.Fatalf("seed current version: %v", err)
	}

	release := model.ServiceReleaseRecord{
		ID:              "release-1",
		ServiceID:       definition.ID,
		TargetVersionID: current.ID,
	}
	check, err := service.RunServiceCheck(context.Background(), definition, release, "task-1")
	if err == nil {
		t.Fatal("expected health check failure")
	}
	if !strings.Contains(err.Error(), "unexpected http status 503") {
		t.Fatalf("error = %q, want http status mismatch", err.Error())
	}
	if check.Status != model.HealthCheckStatusFailed {
		t.Fatalf("status = %s, want %s", check.Status, model.HealthCheckStatusFailed)
	}
	if check.ErrorMessage == "" {
		t.Fatal("expected stored error message")
	}

	var event model.AlertEvent
	if err := database.First(&event, "event_type = ?", "service_health_check_failed").Error; err != nil {
		t.Fatalf("load alert event: %v", err)
	}
	if event.Status != model.AlertEventStatusOpen {
		t.Fatalf("event status = %s, want %s", event.Status, model.AlertEventStatusOpen)
	}
	if !strings.Contains(event.Suggestion, "\"version\":\"1.0.0\"") {
		t.Fatalf("suggestion = %q, want previous version payload", event.Suggestion)
	}
}

func TestRecordHostAvailabilityCreatesAndResolvesOfflineAlert(t *testing.T) {
	database := openHealthTestDB(t)
	service := newHealthService(database)
	ctx := context.Background()
	started := time.Now().UTC().Add(-time.Second)

	check, err := service.RecordHostAvailability(ctx, "host-1", "task-1", started, errors.New("dial tcp timeout"))
	if err != nil {
		t.Fatalf("record host availability failure: %v", err)
	}
	if check.Status != model.HostAvailabilityStatusUnreachable {
		t.Fatalf("status = %s, want %s", check.Status, model.HostAvailabilityStatusUnreachable)
	}
	if check.FailureReason != "dial tcp timeout" {
		t.Fatalf("failure reason = %q, want %q", check.FailureReason, "dial tcp timeout")
	}

	var offline model.AlertEvent
	if err := database.First(&offline, "event_type = ? AND resource_id = ?", "host_offline", "host-1").Error; err != nil {
		t.Fatalf("load offline alert: %v", err)
	}
	if offline.Status != model.AlertEventStatusOpen {
		t.Fatalf("offline alert status = %s, want %s", offline.Status, model.AlertEventStatusOpen)
	}

	check, err = service.RecordHostAvailability(ctx, "host-1", "task-2", started, nil)
	if err != nil {
		t.Fatalf("record host availability success: %v", err)
	}
	if check.Status != model.HostAvailabilityStatusOnline {
		t.Fatalf("status = %s, want %s", check.Status, model.HostAvailabilityStatusOnline)
	}

	if err := database.First(&offline, "id = ?", offline.ID).Error; err != nil {
		t.Fatalf("reload offline alert: %v", err)
	}
	if offline.Status != model.AlertEventStatusResolved {
		t.Fatalf("offline alert status after recovery = %s, want %s", offline.Status, model.AlertEventStatusResolved)
	}
	if offline.ResolvedAt == nil {
		t.Fatal("expected resolved timestamp after host recovery")
	}
}

func TestRollbackSuggestionReturnsPreviousVersion(t *testing.T) {
	database := openHealthTestDB(t)
	service := newHealthService(database)

	definition := model.ServiceDefinition{
		ID:             "svc-1",
		Code:           "demo-api",
		CurrentVersion: "2.0.0",
	}
	if err := database.Create(&definition).Error; err != nil {
		t.Fatalf("seed service: %v", err)
	}
	if err := database.Create(&model.ServiceVersion{
		ID:        "ver-1",
		ServiceID: definition.ID,
		Version:   "1.0.0",
		Image:     "registry.local/demo-api",
		ImageTag:  "1.0.0",
	}).Error; err != nil {
		t.Fatalf("seed previous version: %v", err)
	}
	if err := database.Create(&model.ServiceVersion{
		ID:        "ver-2",
		ServiceID: definition.ID,
		Version:   "2.0.0",
		Image:     "registry.local/demo-api",
		ImageTag:  "2.0.0",
	}).Error; err != nil {
		t.Fatalf("seed current version: %v", err)
	}

	suggestion, err := service.RollbackSuggestion(context.Background(), definition.ID)
	if err != nil {
		t.Fatalf("rollback suggestion: %v", err)
	}
	if available, _ := suggestion["available"].(bool); !available {
		t.Fatalf("suggestion availability = %v, want true", suggestion["available"])
	}
	if suggestion["version"] != "1.0.0" {
		t.Fatalf("suggested version = %v, want 1.0.0", suggestion["version"])
	}
}

func newHealthService(database *gorm.DB) *Service {
	notifications := notification.NewService(database)
	alerts := alertsvc.NewService(database, notifications)
	return NewService(database, alerts)
}

func healthPolicyEnvJSON(target string, expectedStatus int) string {
	return fmt.Sprintf(`[{"name":"AEGISOPS_HEALTHCHECK","value":"{\"type\":\"HTTP\",\"target\":\"%s\",\"expectedHttpStatus\":%d,\"timeoutSeconds\":2,\"retryTimes\":1}"}]`, target, expectedStatus)
}

func openHealthTestDB(t *testing.T) *gorm.DB {
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
	return database
}
