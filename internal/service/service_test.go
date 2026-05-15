package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
)

func TestDeleteRejectsServiceWithInstances(t *testing.T) {
	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, tasks, NoopReleaseExecutor{})

	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:       "svc-1",
		Name:     "Demo API",
		Code:     "demo-api",
		Image:    "registry.local/demo-api",
		TargetID: "docker-node-1",
	})
	if err := database.Create(&model.ServiceInstance{
		ID:        "instance-1",
		ServiceID: definition.ID,
		Version:   "1.0.0",
		Image:     definition.Image,
		ImageTag:  "1.0.0",
		Name:      definition.Code,
		Status:    model.ServiceInstanceStatusRunning,
	}).Error; err != nil {
		t.Fatalf("seed instance: %v", err)
	}

	err := service.Delete(context.Background(), definition.ID)
	if err == nil {
		t.Fatal("expected delete to fail when instances exist")
	}
	if !strings.Contains(err.Error(), "service has instances") {
		t.Fatalf("delete error = %q, want instance constraint", err.Error())
	}
}

func TestReleaseSuccessCreatesVersionInstanceAndTask(t *testing.T) {
	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, tasks, NoopReleaseExecutor{})

	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:         "svc-1",
		Name:       "Demo API",
		Code:       "demo-api",
		Image:      "registry.local/demo-api",
		DefaultTag: "1.0.0",
		TargetID:   "docker-node-1",
	})

	result, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "1.0.1",
		Version:    "2026.05.15",
		OperatorID: "user-1",
	})
	if err != nil {
		t.Fatalf("release service: %v", err)
	}
	if result.TaskID == "" || result.ReleaseID == "" {
		t.Fatalf("unexpected release result: %+v", result)
	}

	var version model.ServiceVersion
	if err := database.First(&version, "service_id = ?", definition.ID).Error; err != nil {
		t.Fatalf("load version: %v", err)
	}
	if version.Version != "2026.05.15" || version.ImageTag != "1.0.1" {
		t.Fatalf("unexpected version record: %+v", version)
	}

	var instance model.ServiceInstance
	if err := database.First(&instance, "service_id = ?", definition.ID).Error; err != nil {
		t.Fatalf("load instance: %v", err)
	}
	if instance.Status != model.ServiceInstanceStatusRunning {
		t.Fatalf("instance status = %s, want %s", instance.Status, model.ServiceInstanceStatusRunning)
	}
	if !strings.HasPrefix(instance.ContainerID, "noop-") {
		t.Fatalf("container id = %q, want noop prefix", instance.ContainerID)
	}

	var release model.ServiceReleaseRecord
	if err := database.First(&release, "id = ?", result.ReleaseID).Error; err != nil {
		t.Fatalf("load release: %v", err)
	}
	if release.Status != model.TaskStatusSuccess {
		t.Fatalf("release status = %s, want %s", release.Status, model.TaskStatusSuccess)
	}
	if release.TargetVersionID != version.ID {
		t.Fatalf("target version id = %q, want %q", release.TargetVersionID, version.ID)
	}

	stored, err := service.Get(context.Background(), definition.ID)
	if err != nil {
		t.Fatalf("reload service: %v", err)
	}
	if stored.Status != model.ServiceStatusActive {
		t.Fatalf("service status = %s, want %s", stored.Status, model.ServiceStatusActive)
	}
	if stored.CurrentVersion != "2026.05.15" {
		t.Fatalf("current version = %q, want %q", stored.CurrentVersion, "2026.05.15")
	}

	task, err := tasks.Get(context.Background(), result.TaskID)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != model.TaskStatusSuccess {
		t.Fatalf("task status = %s, want %s", task.Status, model.TaskStatusSuccess)
	}
	if len(task.Steps) != 5 {
		t.Fatalf("task step count = %d, want 5", len(task.Steps))
	}
	for _, step := range task.Steps {
		if step.Status != model.TaskStatusSuccess {
			t.Fatalf("step %q status = %s, want %s", step.Name, step.Status, model.TaskStatusSuccess)
		}
	}
}

func TestReleaseDeployFailureMarksReleaseTaskAndInstanceFailed(t *testing.T) {
	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, tasks, stubReleaseExecutor{
		deployErr: errors.New("docker daemon unavailable"),
	})

	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:         "svc-1",
		Name:       "Demo API",
		Code:       "demo-api",
		Image:      "registry.local/demo-api",
		DefaultTag: "1.0.0",
		TargetID:   "docker-node-1",
	})

	result, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "1.0.1",
		Version:    "2026.05.15",
		OperatorID: "user-1",
	})
	if err == nil {
		t.Fatal("expected deploy failure")
	}
	if !strings.Contains(err.Error(), "docker daemon unavailable") {
		t.Fatalf("release error = %q, want deploy error", err.Error())
	}
	if result != nil {
		t.Fatalf("release result = %+v, want nil on failure", result)
	}

	var release model.ServiceReleaseRecord
	if err := database.First(&release, "service_id = ?", definition.ID).Error; err != nil {
		t.Fatalf("load release: %v", err)
	}
	if release.Status != model.TaskStatusFailed {
		t.Fatalf("release status = %s, want %s", release.Status, model.TaskStatusFailed)
	}
	if !strings.Contains(release.Message, "docker daemon unavailable") {
		t.Fatalf("release message = %q, want deploy error", release.Message)
	}

	var instance model.ServiceInstance
	if err := database.First(&instance, "service_id = ?", definition.ID).Error; err != nil {
		t.Fatalf("load failed instance: %v", err)
	}
	if instance.Status != model.ServiceInstanceStatusFailed {
		t.Fatalf("instance status = %s, want %s", instance.Status, model.ServiceInstanceStatusFailed)
	}
	if instance.LastError != "docker daemon unavailable" {
		t.Fatalf("instance last error = %q, want deploy error", instance.LastError)
	}

	task, err := tasks.Get(context.Background(), release.TaskID)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != model.TaskStatusFailed {
		t.Fatalf("task status = %s, want %s", task.Status, model.TaskStatusFailed)
	}
	if task.Error != "docker daemon unavailable" {
		t.Fatalf("task error = %q, want deploy error", task.Error)
	}
}

func TestReleaseHealthCheckFailureMarksLatestInstanceFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, tasks, NoopReleaseExecutor{})

	notifications := notification.NewService(database)
	alerts := alertsvc.NewService(database, notifications)
	health := healthsvc.NewService(database, alerts)
	service.SetHealthCheckService(health)

	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:             "svc-1",
		Name:           "Demo API",
		Code:           "demo-api",
		Image:          "registry.local/demo-api",
		DefaultTag:     "2.0.0",
		TargetID:       "docker-node-1",
		CurrentVersion: "1.0.0",
		Envs:           healthPolicyEnvJSON(server.URL),
	})
	if err := database.Create(&model.ServiceVersion{
		ID:        "ver-prev",
		ServiceID: definition.ID,
		Version:   "1.0.0",
		Image:     definition.Image,
		ImageTag:  "1.0.0",
	}).Error; err != nil {
		t.Fatalf("seed previous version: %v", err)
	}

	result, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "2.0.0",
		Version:    "2.0.0",
		OperatorID: "user-1",
	})
	if err == nil {
		t.Fatal("expected release to fail on health check")
	}
	if !strings.Contains(err.Error(), "unexpected http status 503") {
		t.Fatalf("release error = %q, want health check failure", err.Error())
	}
	if result != nil {
		t.Fatalf("release result = %+v, want nil on failure", result)
	}

	var release model.ServiceReleaseRecord
	if err := database.Order("created_at DESC").First(&release, "service_id = ?", definition.ID).Error; err != nil {
		t.Fatalf("load release: %v", err)
	}
	if release.Status != model.TaskStatusFailed {
		t.Fatalf("release status = %s, want %s", release.Status, model.TaskStatusFailed)
	}

	var instance model.ServiceInstance
	if err := database.Order("created_at DESC").First(&instance, "service_id = ?", definition.ID).Error; err != nil {
		t.Fatalf("load instance: %v", err)
	}
	if instance.Status != model.ServiceInstanceStatusFailed {
		t.Fatalf("instance status = %s, want %s", instance.Status, model.ServiceInstanceStatusFailed)
	}
	if !strings.Contains(instance.LastError, "unexpected http status 503") {
		t.Fatalf("instance last error = %q, want health check failure", instance.LastError)
	}

	var check model.ServiceHealthCheck
	if err := database.First(&check, "service_id = ?", definition.ID).Error; err != nil {
		t.Fatalf("load health check: %v", err)
	}
	if check.Status != model.HealthCheckStatusFailed {
		t.Fatalf("health check status = %s, want %s", check.Status, model.HealthCheckStatusFailed)
	}

	task, err := tasks.Get(context.Background(), release.TaskID)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != model.TaskStatusFailed {
		t.Fatalf("task status = %s, want %s", task.Status, model.TaskStatusFailed)
	}
}

func TestReleaseRejectsWhenAnotherReleaseIsRunning(t *testing.T) {
	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, tasks, NoopReleaseExecutor{})

	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:         "svc-1",
		Name:       "Demo API",
		Code:       "demo-api",
		Image:      "registry.local/demo-api",
		DefaultTag: "1.0.0",
		TargetID:   "docker-node-1",
	})
	if err := database.Create(&model.ServiceReleaseRecord{
		ID:        "release-running",
		ServiceID: definition.ID,
		TaskID:    "task-running",
		Action:    model.ServiceReleaseActionRelease,
		Status:    model.TaskStatusRunning,
		Message:   "running",
	}).Error; err != nil {
		t.Fatalf("seed running release: %v", err)
	}

	result, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "1.0.1",
		Version:    "2026.05.15",
		OperatorID: "user-1",
	})
	if !errors.Is(err, ErrReleaseInProgress) {
		t.Fatalf("release error = %v, want %v", err, ErrReleaseInProgress)
	}
	if result != nil {
		t.Fatalf("release result = %+v, want nil", result)
	}
}

type stubReleaseExecutor struct {
	validateErr error
	deployErr   error
	result      *DeployResult
}

func (s stubReleaseExecutor) Validate(_ context.Context, _ DeployRequest) error {
	return s.validateErr
}

func (s stubReleaseExecutor) Deploy(_ context.Context, req DeployRequest) (*DeployResult, error) {
	if s.deployErr != nil {
		return nil, s.deployErr
	}
	if s.result != nil {
		return s.result, nil
	}
	return &DeployResult{
		ContainerID: "stub-container",
		Image:       imageRef(req.Image, req.ImageTag),
	}, nil
}

func seedServiceDefinition(t *testing.T, database *gorm.DB, definition model.ServiceDefinition) model.ServiceDefinition {
	t.Helper()
	if definition.Status == "" {
		definition.Status = model.ServiceStatusDraft
	}
	if definition.TargetType == "" {
		definition.TargetType = "DOCKER_NODE"
	}
	if err := database.Create(&definition).Error; err != nil {
		t.Fatalf("seed service definition: %v", err)
	}
	return definition
}

func healthPolicyEnvJSON(target string) string {
	policy, err := json.Marshal(map[string]any{
		"type":               "HTTP",
		"target":             target,
		"expectedHttpStatus": http.StatusOK,
		"timeoutSeconds":     2,
		"retryTimes":         1,
	})
	if err != nil {
		panic(err)
	}
	envs, err := json.Marshal([]map[string]string{
		{
			"name":  "AEGISOPS_HEALTHCHECK",
			"value": string(policy),
		},
	})
	if err != nil {
		panic(err)
	}
	return string(envs)
}

func openServiceTestDB(t *testing.T) *gorm.DB {
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
