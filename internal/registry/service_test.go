package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	secretsvc "github.com/Humphrey-He/AegisOps/internal/secret"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
	"gorm.io/gorm"
)

func TestWorkerExecutesRegistryTestDispatch(t *testing.T) {
	t.Parallel()

	database, secretService, cleanup := newRegistryTestDB(t)
	defer cleanup()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/" {
			t.Fatalf("request path = %s, want /v2/", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	registry := model.Registry{
		ID:       "registry-success",
		Name:     "Registry Success",
		URL:      server.URL,
		AuthType: model.RegistryAuthTypeNone,
		Status:   model.RegistryStatusUnknown,
	}
	if err := database.Create(&registry).Error; err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	taskItem := model.Task{
		ID:         "task-registry-success",
		Type:       "registry.test",
		Title:      "test registry",
		Status:     model.TaskStatusPending,
		TargetType: "registry",
		TargetID:   registry.ID,
	}
	if err := database.Create(&taskItem).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	dispatch := model.TaskDispatch{
		ID:             "dispatch-registry-success",
		TaskID:         taskItem.ID,
		Source:         model.TaskDispatchSourceScheduled,
		Status:         model.TaskDispatchStatusPending,
		TimeoutSeconds: 60,
		ConcurrencyKey: "registry:" + registry.ID + ":test",
		QueuedAt:       time.Now().UTC(),
	}
	if err := database.Create(&dispatch).Error; err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}

	registryService := NewService(database, secretService)
	taskService := tasksvc.NewService(database)
	worker := tasksvc.NewWorker(taskService)
	processed, err := worker.RunOnce(context.Background(), tasksvc.WorkerOptions{
		Owner:    "registry-test-worker",
		Executor: tasksvc.NewDispatchExecutor(registryService),
	})
	if err != nil {
		t.Fatalf("worker RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	var updatedRegistry model.Registry
	if err := database.First(&updatedRegistry, "id = ?", registry.ID).Error; err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if updatedRegistry.Status != model.RegistryStatusOnline || updatedRegistry.LastTestAt == nil {
		t.Fatalf("registry status = %s lastTestAt=%v, want ONLINE with timestamp", updatedRegistry.Status, updatedRegistry.LastTestAt)
	}
	var updatedTask model.Task
	if err := database.First(&updatedTask, "id = ?", taskItem.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != model.TaskStatusSuccess || updatedTask.Result == "" {
		t.Fatalf("task after registry test = %+v", updatedTask)
	}
	var updatedDispatch model.TaskDispatch
	if err := database.First(&updatedDispatch, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updatedDispatch.Status != model.TaskDispatchStatusSuccess || updatedDispatch.FinishedAt == nil {
		t.Fatalf("dispatch after registry test = %+v", updatedDispatch)
	}
}

func TestWorkerRegistryTestDispatchFailureMarksOffline(t *testing.T) {
	t.Parallel()

	database, secretService, cleanup := newRegistryTestDB(t)
	defer cleanup()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("registry unavailable"))
	}))
	defer server.Close()
	registry := model.Registry{
		ID:       "registry-failure",
		Name:     "Registry Failure",
		URL:      server.URL,
		AuthType: model.RegistryAuthTypeNone,
		Status:   model.RegistryStatusUnknown,
	}
	if err := database.Create(&registry).Error; err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	taskItem := model.Task{
		ID:         "task-registry-failure",
		Type:       "registry.test",
		Title:      "test registry",
		Status:     model.TaskStatusPending,
		TargetType: "registry",
		TargetID:   registry.ID,
	}
	if err := database.Create(&taskItem).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	dispatch := model.TaskDispatch{
		ID:             "dispatch-registry-failure",
		TaskID:         taskItem.ID,
		Source:         model.TaskDispatchSourceScheduled,
		Status:         model.TaskDispatchStatusPending,
		TimeoutSeconds: 60,
		ConcurrencyKey: "registry:" + registry.ID + ":test",
		QueuedAt:       time.Now().UTC(),
	}
	if err := database.Create(&dispatch).Error; err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}

	registryService := NewService(database, secretService)
	taskService := tasksvc.NewService(database)
	worker := tasksvc.NewWorker(taskService)
	processed, err := worker.RunOnce(context.Background(), tasksvc.WorkerOptions{
		Owner:    "registry-test-worker",
		Executor: tasksvc.NewDispatchExecutor(registryService),
	})
	if err != nil {
		t.Fatalf("worker RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	var updatedRegistry model.Registry
	if err := database.First(&updatedRegistry, "id = ?", registry.ID).Error; err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if updatedRegistry.Status != model.RegistryStatusOffline || updatedRegistry.LastTestAt == nil {
		t.Fatalf("registry status = %s lastTestAt=%v, want OFFLINE with timestamp", updatedRegistry.Status, updatedRegistry.LastTestAt)
	}
	var updatedTask model.Task
	if err := database.First(&updatedTask, "id = ?", taskItem.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != model.TaskStatusFailed || updatedTask.Error == "" {
		t.Fatalf("task after registry failure = %+v", updatedTask)
	}
	var updatedDispatch model.TaskDispatch
	if err := database.First(&updatedDispatch, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updatedDispatch.Status != model.TaskDispatchStatusFailed || updatedDispatch.FinishedAt == nil {
		t.Fatalf("dispatch after registry failure = %+v", updatedDispatch)
	}
}

func newRegistryTestDB(t *testing.T) (*gorm.DB, *secretsvc.Service, func()) {
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
		t.Fatalf("get sql database: %v", err)
	}
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	secretService, err := secretsvc.NewService(database, "test-secret-key")
	if err != nil {
		t.Fatalf("new secret service: %v", err)
	}
	return database, secretService, func() { _ = sqlDB.Close() }
}
