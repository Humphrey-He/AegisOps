package service

import (
	"context"
	"encoding/base64"
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
	secretsvc "github.com/Humphrey-He/AegisOps/internal/secret"
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
	assertQueuedRelease(t, database, tasks, result)
	runServiceWorker(t, database, tasks, service)

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
	if err != nil {
		t.Fatalf("enqueue release: %v", err)
	}
	if result.TaskID == "" || result.ReleaseID == "" {
		t.Fatalf("unexpected release result: %+v", result)
	}
	processed, err := runServiceWorkerAllowError(t, database, tasks, service)
	if err != nil {
		t.Fatalf("worker RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
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
	if err != nil {
		t.Fatalf("enqueue release: %v", err)
	}
	if result.TaskID == "" || result.ReleaseID == "" {
		t.Fatalf("unexpected release result: %+v", result)
	}
	processed, err := runServiceWorkerAllowError(t, database, tasks, service)
	if err != nil {
		t.Fatalf("worker RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
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

func TestReleaseRejectsWhenDispatchIsPending(t *testing.T) {
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
	if _, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "1.0.1",
		Version:    "2026.05.15",
		OperatorID: "user-1",
	}); err != nil {
		t.Fatalf("enqueue first release: %v", err)
	}
	result, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "1.0.2",
		Version:    "2026.05.16",
		OperatorID: "user-1",
	})
	if !errors.Is(err, ErrReleaseInProgress) {
		t.Fatalf("second release error = %v, want %v", err, ErrReleaseInProgress)
	}
	if result != nil {
		t.Fatalf("second release result = %+v, want nil", result)
	}
}

func TestReleaseRejectsDockerNodeEnvironmentMismatch(t *testing.T) {
	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, tasks, NoopReleaseExecutor{})

	seedEnvironment(t, database, "prod", model.EnvironmentStatusActive)
	seedEnvironment(t, database, "dev", model.EnvironmentStatusActive)
	if err := database.Create(&model.DockerNode{
		ID:          "docker-dev-1",
		Name:        "dev docker",
		Endpoint:    "mock://dev-docker",
		Environment: "dev",
		Status:      model.DockerNodeStatusOnline,
	}).Error; err != nil {
		t.Fatalf("seed docker node: %v", err)
	}
	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:          "svc-1",
		Name:        "Prod API",
		Code:        "prod-api",
		Environment: "prod",
		Image:       "registry.local/prod-api",
		DefaultTag:  "1.0.0",
		TargetID:    "docker-dev-1",
	})

	result, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "1.0.1",
		Version:    "2026.05.19",
		OperatorID: "user-1",
	})
	if err == nil {
		t.Fatal("expected release to fail on environment mismatch")
	}
	if !strings.Contains(err.Error(), "does not match service environment") {
		t.Fatalf("release error = %q, want environment mismatch", err.Error())
	}
	if result != nil {
		t.Fatalf("release result = %+v, want nil", result)
	}
}

func TestReleaseRejectsMissingDisabledOrBlankTargetEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		serviceEnv  string
		nodeEnv     string
		seedService bool
		seedNode    bool
		want        string
	}{
		{name: "missing service environment", serviceEnv: "prod", nodeEnv: "dev", seedService: false, seedNode: true, want: "not found"},
		{name: "disabled service environment", serviceEnv: "prod", nodeEnv: "dev", seedService: true, seedNode: true, want: "not active"},
		{name: "blank target environment", serviceEnv: "prod", nodeEnv: "", seedService: true, seedNode: false, want: "target docker node environment is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := openServiceTestDB(t)
			tasks := tasksvc.NewService(database)
			service := NewService(database, tasks, NoopReleaseExecutor{})
			if tt.seedService {
				status := model.EnvironmentStatusActive
				if tt.name == "disabled service environment" {
					status = model.EnvironmentStatusDisabled
				}
				seedEnvironment(t, database, tt.serviceEnv, status)
			}
			if tt.seedNode {
				seedEnvironment(t, database, tt.nodeEnv, model.EnvironmentStatusActive)
			}
			if err := database.Create(&model.DockerNode{
				ID:          "docker-1",
				Name:        "docker",
				Endpoint:    "mock://docker",
				Environment: tt.nodeEnv,
			}).Error; err != nil {
				t.Fatalf("seed docker node: %v", err)
			}
			definition := seedServiceDefinition(t, database, model.ServiceDefinition{
				ID:          "svc-1",
				Name:        "Prod API",
				Code:        "prod-api",
				Environment: tt.serviceEnv,
				Image:       "registry.local/prod-api",
				DefaultTag:  "1.0.0",
				TargetID:    "docker-1",
			})

			result, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
				ImageTag:   "1.0.1",
				Version:    "2026.05.19",
				OperatorID: "user-1",
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("release error = %v, want %q", err, tt.want)
			}
			if result != nil {
				t.Fatalf("release result = %+v, want nil", result)
			}
		})
	}
}

func TestReleasePassesPrivateRegistryAuthToDockerExecutor(t *testing.T) {
	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	secretService, err := secretsvc.NewService(database, "test-master-key")
	if err != nil {
		t.Fatalf("new secret service: %v", err)
	}
	secret, err := secretService.Create(context.Background(), secretsvc.CreateRequest{
		Name:  "registry basic",
		Type:  model.SecretTypeDockerToken,
		Value: "robot:super-secret",
	})
	if err != nil {
		t.Fatalf("create registry secret: %v", err)
	}
	registry := model.Registry{
		ID:       "registry-1",
		Name:     "Private Registry",
		URL:      "registry.local",
		AuthType: model.RegistryAuthTypeBasic,
		SecretID: secret.ID,
	}
	if err := database.Create(&registry).Error; err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	executor := &capturingReleaseExecutor{}
	service := NewService(database, tasks, executor)
	service.SetSecretService(secretService)
	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:         "svc-1",
		Name:       "Demo API",
		Code:       "demo-api",
		RegistryID: registry.ID,
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
		t.Fatalf("enqueue release: %v", err)
	}
	if result.TaskID == "" {
		t.Fatal("expected task id")
	}
	runServiceWorker(t, database, tasks, service)
	if executor.deployReq.RegistryAuth == "" {
		t.Fatal("registry auth was not passed to deploy request")
	}
	authBytes, err := base64.URLEncoding.DecodeString(executor.deployReq.RegistryAuth)
	if err != nil {
		t.Fatalf("decode registry auth: %v", err)
	}
	var auth map[string]string
	if err := json.Unmarshal(authBytes, &auth); err != nil {
		t.Fatalf("unmarshal registry auth: %v", err)
	}
	if auth["username"] != "robot" || auth["password"] != "super-secret" || auth["serveraddress"] != "registry.local" {
		t.Fatalf("registry auth = %+v", auth)
	}
}

func TestReleaseThenRollbackThroughWorker(t *testing.T) {
	database := openServiceTestDB(t)
	tasks := tasksvc.NewService(database)
	service := NewService(database, tasks, NoopReleaseExecutor{})
	definition := seedServiceDefinition(t, database, model.ServiceDefinition{
		ID:             "svc-rollback",
		Name:           "Rollback API",
		Code:           "rollback-api",
		Image:          "registry.local/rollback-api",
		DefaultTag:     "1.0.0",
		TargetID:       "docker-node-1",
		Status:         model.ServiceStatusActive,
		CurrentVersion: "1.0.0",
	})
	if err := database.Create(&model.ServiceVersion{
		ID:        "ver-1",
		ServiceID: definition.ID,
		Version:   "1.0.0",
		Image:     definition.Image,
		ImageTag:  "1.0.0",
	}).Error; err != nil {
		t.Fatalf("seed previous version: %v", err)
	}
	if err := database.Create(&model.ServiceInstance{
		ID:           "inst-1",
		ServiceID:    definition.ID,
		VersionID:    "ver-1",
		Version:      "1.0.0",
		Image:        definition.Image,
		ImageTag:     "1.0.0",
		DockerNodeID: "docker-node-1",
		Name:         definition.Code,
		Status:       model.ServiceInstanceStatusRunning,
	}).Error; err != nil {
		t.Fatalf("seed previous instance: %v", err)
	}
	releaseResult, err := service.Release(context.Background(), definition.ID, ReleaseRequest{
		ImageTag:   "2.0.0",
		Version:    "2.0.0",
		OperatorID: "user-1",
	})
	if err != nil {
		t.Fatalf("enqueue release: %v", err)
	}
	runServiceWorker(t, database, tasks, service)
	task, err := tasks.Get(context.Background(), releaseResult.TaskID)
	if err != nil {
		t.Fatalf("load release task: %v", err)
	}
	if task.Status != model.TaskStatusSuccess || len(task.Steps) != 5 {
		t.Fatalf("release task = %+v", task)
	}
	var running model.ServiceInstance
	if err := database.First(&running, "service_id = ? AND status = ?", definition.ID, model.ServiceInstanceStatusRunning).Error; err != nil {
		t.Fatalf("load running instance: %v", err)
	}
	if running.Version != "2.0.0" || !strings.HasPrefix(running.ContainerID, "noop-") {
		t.Fatalf("running instance after release = %+v", running)
	}
	rollbackResult, err := service.Rollback(context.Background(), definition.ID, RollbackRequest{
		VersionID:   "ver-1",
		OperatorID: "user-1",
	})
	if err != nil {
		t.Fatalf("enqueue rollback: %v", err)
	}
	runServiceWorker(t, database, tasks, service)
	rollbackTask, err := tasks.Get(context.Background(), rollbackResult.TaskID)
	if err != nil {
		t.Fatalf("load rollback task: %v", err)
	}
	if rollbackTask.Status != model.TaskStatusSuccess {
		t.Fatalf("rollback task status = %s, want %s", rollbackTask.Status, model.TaskStatusSuccess)
	}
	stored, err := service.Get(context.Background(), definition.ID)
	if err != nil {
		t.Fatalf("reload service: %v", err)
	}
	if stored.CurrentVersion != "1.0.0" {
		t.Fatalf("current version after rollback = %q, want 1.0.0", stored.CurrentVersion)
	}
}

func seedEnvironment(t *testing.T, database *gorm.DB, code string, status model.EnvironmentStatus) {
	t.Helper()
	if err := database.Create(&model.Environment{
		ID:     "env-" + code,
		Name:   code,
		Code:   code,
		Status: status,
	}).Error; err != nil {
		t.Fatalf("seed environment %s: %v", code, err)
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

type capturingReleaseExecutor struct {
	validateReq DeployRequest
	deployReq   DeployRequest
}

func (c *capturingReleaseExecutor) Validate(_ context.Context, req DeployRequest) error {
	c.validateReq = req
	return nil
}

func (c *capturingReleaseExecutor) Deploy(_ context.Context, req DeployRequest) (*DeployResult, error) {
	c.deployReq = req
	return &DeployResult{
		ContainerID: "captured-container",
		Image:       imageRef(req.Image, req.ImageTag),
	}, nil
}

func assertQueuedRelease(t *testing.T, database *gorm.DB, tasks *tasksvc.Service, result *ReleaseResult) {
	t.Helper()
	var release model.ServiceReleaseRecord
	if err := database.First(&release, "id = ?", result.ReleaseID).Error; err != nil {
		t.Fatalf("load queued release: %v", err)
	}
	if release.Status != model.TaskStatusPending {
		t.Fatalf("queued release status = %s, want %s", release.Status, model.TaskStatusPending)
	}
	task, err := tasks.Get(context.Background(), result.TaskID)
	if err != nil {
		t.Fatalf("load queued task: %v", err)
	}
	if task.Status != model.TaskStatusPending {
		t.Fatalf("queued task status = %s, want %s", task.Status, model.TaskStatusPending)
	}
	if len(task.Dispatches) != 1 || task.Dispatches[0].Status != model.TaskDispatchStatusPending {
		t.Fatalf("queued dispatches = %+v, want one pending dispatch", task.Dispatches)
	}
}

func runServiceWorker(t *testing.T, database *gorm.DB, tasks *tasksvc.Service, service *Service) {
	t.Helper()
	processed, err := runServiceWorkerAllowError(t, database, tasks, service)
	if err != nil {
		t.Fatalf("worker RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
}

func runServiceWorkerAllowError(t *testing.T, _ *gorm.DB, tasks *tasksvc.Service, service *Service) (int, error) {
	t.Helper()
	worker := tasksvc.NewWorker(tasks)
	return worker.RunOnce(context.Background(), tasksvc.WorkerOptions{
		Owner: "service-test-worker",
		Executor: tasksvc.NewDispatchExecutor(tasksvc.DispatchExecutorOptions{
			Service: service,
		}),
	})
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
