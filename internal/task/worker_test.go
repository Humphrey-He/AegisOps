package task

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
)

func TestWorkerProcessesNoopDispatch(t *testing.T) {
	t.Parallel()

	service, cleanup := newWorkerTestService(t)
	defer cleanup()
	task, dispatch := seedWorkerDispatch(t, service, "scheduled.noop", model.TaskDispatchStatusPending, 0, 0, 60)

	worker := NewWorker(service)
	processed, err := worker.RunOnce(context.Background(), WorkerOptions{Owner: "test-worker"})
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	var updatedTask model.Task
	if err := service.db.First(&updatedTask, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != model.TaskStatusSuccess || updatedTask.Result == "" || updatedTask.FinishedAt == nil {
		t.Fatalf("task not successful: %+v", updatedTask)
	}
	var updatedDispatch model.TaskDispatch
	if err := service.db.First(&updatedDispatch, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updatedDispatch.Status != model.TaskDispatchStatusSuccess || updatedDispatch.LeaseOwner != "" || updatedDispatch.FinishedAt == nil {
		t.Fatalf("dispatch not successful: %+v", updatedDispatch)
	}
}

func TestWorkerRetriesFailedDispatch(t *testing.T) {
	t.Parallel()

	service, cleanup := newWorkerTestService(t)
	defer cleanup()
	task, dispatch := seedWorkerDispatch(t, service, "custom.fail", model.TaskDispatchStatusPending, 0, 1, 60)

	worker := NewWorker(service)
	processed, err := worker.RunOnce(context.Background(), WorkerOptions{
		Owner: "test-worker",
		Executor: func(context.Context, model.Task, model.TaskDispatch) (string, error) {
			return "", errors.New("temporary failure")
		},
	})
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	var updatedDispatch model.TaskDispatch
	if err := service.db.First(&updatedDispatch, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updatedDispatch.Status != model.TaskDispatchStatusPending || updatedDispatch.RetryCount != 1 || updatedDispatch.LeaseOwner != "" {
		t.Fatalf("dispatch not requeued: %+v", updatedDispatch)
	}
	var updatedTask model.Task
	if err := service.db.First(&updatedTask, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != model.TaskStatusPending || updatedTask.Error != "temporary failure" {
		t.Fatalf("task not reset for retry: %+v", updatedTask)
	}
}

func TestWorkerFailsUnsupportedTaskAfterRetryExhausted(t *testing.T) {
	t.Parallel()

	service, cleanup := newWorkerTestService(t)
	defer cleanup()
	task, dispatch := seedWorkerDispatch(t, service, "host.availability", model.TaskDispatchStatusPending, 1, 1, 60)

	worker := NewWorker(service)
	processed, err := worker.RunOnce(context.Background(), WorkerOptions{Owner: "test-worker"})
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	var updatedDispatch model.TaskDispatch
	if err := service.db.First(&updatedDispatch, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updatedDispatch.Status != model.TaskDispatchStatusFailed || updatedDispatch.FinishedAt == nil {
		t.Fatalf("dispatch not failed: %+v", updatedDispatch)
	}
	var updatedTask model.Task
	if err := service.db.First(&updatedTask, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != model.TaskStatusFailed || updatedTask.Error == "" {
		t.Fatalf("task not failed: %+v", updatedTask)
	}
}

func TestWorkerMarksTimeout(t *testing.T) {
	t.Parallel()

	service, cleanup := newWorkerTestService(t)
	defer cleanup()
	task, dispatch := seedWorkerDispatch(t, service, "scheduled.noop", model.TaskDispatchStatusPending, 0, 0, 1)

	worker := NewWorker(service)
	processed, err := worker.RunOnce(context.Background(), WorkerOptions{
		Owner: "test-worker",
		Executor: func(ctx context.Context, _ model.Task, _ model.TaskDispatch) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	var updatedDispatch model.TaskDispatch
	if err := service.db.First(&updatedDispatch, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updatedDispatch.Status != model.TaskDispatchStatusTimeout {
		t.Fatalf("dispatch status = %s, want TIMEOUT", updatedDispatch.Status)
	}
	var updatedTask model.Task
	if err := service.db.First(&updatedTask, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != model.TaskStatusFailed || updatedTask.Error != "dispatch timed out" {
		t.Fatalf("task timeout result: %+v", updatedTask)
	}
}

func TestWorkerRunStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	service, cleanup := newWorkerTestService(t)
	defer cleanup()
	seedWorkerDispatch(t, service, "scheduled.noop", model.TaskDispatchStatusPending, 0, 0, 60)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	complete := make(chan model.TaskDispatch, 1)
	worker := NewWorker(service)
	go func() {
		defer close(done)
		worker.Run(ctx, WorkerOptions{
			Interval: 10 * time.Millisecond,
			Owner:    "test-worker",
			OnComplete: func(dispatch model.TaskDispatch) {
				complete <- dispatch
				cancel()
			},
		})
	}()

	select {
	case dispatch := <-complete:
		if dispatch.Status != model.TaskDispatchStatusSuccess {
			t.Fatalf("dispatch status = %s, want SUCCESS", dispatch.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for worker completion")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for worker stop")
	}
}

func newWorkerTestService(t *testing.T) (*Service, func()) {
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
	return NewService(database), func() { _ = sqlDB.Close() }
}

func seedWorkerDispatch(t *testing.T, service *Service, taskType string, status model.TaskDispatchStatus, retryCount, maxRetry, timeoutSeconds int) (model.Task, model.TaskDispatch) {
	t.Helper()
	now := time.Now().UTC()
	task := model.Task{
		ID:     "task-" + taskType + "-" + string(status),
		Type:   taskType,
		Title:  taskType,
		Status: model.TaskStatusPending,
	}
	if err := service.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	dispatch := model.TaskDispatch{
		ID:             "dispatch-" + task.ID,
		TaskID:         task.ID,
		Source:         model.TaskDispatchSourceScheduled,
		JobID:          "job-1",
		Status:         status,
		RetryCount:     retryCount,
		MaxRetry:       maxRetry,
		TimeoutSeconds: timeoutSeconds,
		ConcurrencyKey: taskType,
		QueuedAt:       now,
	}
	if err := service.db.Create(&dispatch).Error; err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}
	return task, dispatch
}
