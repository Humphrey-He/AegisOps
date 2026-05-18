package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
)

func TestCreateRejectsInvalidCron(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()

	_, err := service.Create(context.Background(), JobRequest{
		Name:     "Bad Job",
		Type:     "bad.job",
		CronExpr: "bad cron",
	})
	if err == nil {
		t.Fatal("Create invalid cron err = nil, want error")
	}
}

func TestDispatchDueJobsCreatesTaskDispatch(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()
	now := time.Date(2026, 5, 18, 10, 0, 30, 0, time.UTC)
	service.now = func() time.Time { return now }

	job, err := service.Create(context.Background(), JobRequest{
		Name:            "Host Sweep",
		Type:            "host.availability",
		CronExpr:        "*/5 * * * *",
		TargetType:      "host",
		TargetID:        "host-1",
		PayloadJSON:     `{"mode":"quick"}`,
		TimeoutSeconds:  60,
		ConcurrencyKey:  "host:host-1:availability",
		RetryPolicyJSON: `{"maxRetry":2}`,
		OperatorID:      "1",
	})
	if err != nil {
		t.Fatalf("Create scheduled job: %v", err)
	}
	due := now.Add(-time.Minute)
	if err := service.db.Model(&model.ScheduledJob{}).Where("id = ?", job.ID).Update("next_run_at", &due).Error; err != nil {
		t.Fatalf("mark job due: %v", err)
	}

	dispatches, err := service.DispatchDueJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDueJobs: %v", err)
	}
	if len(dispatches) != 1 {
		t.Fatalf("dispatches len = %d, want 1", len(dispatches))
	}
	dispatch := dispatches[0]
	if dispatch.Source != model.TaskDispatchSourceScheduled || dispatch.JobID != job.ID || dispatch.Status != model.TaskDispatchStatusPending {
		t.Fatalf("unexpected dispatch: %+v", dispatch)
	}
	if dispatch.TimeoutSeconds != 60 || dispatch.ConcurrencyKey != "host:host-1:availability" {
		t.Fatalf("dispatch runtime fields = %+v", dispatch)
	}
	var task model.Task
	if err := service.db.First(&task, "id = ?", dispatch.TaskID).Error; err != nil {
		t.Fatalf("load dispatched task: %v", err)
	}
	if task.Type != "host.availability" || task.TargetType != "host" || task.TargetID != "host-1" || task.Payload != `{"mode":"quick"}` {
		t.Fatalf("unexpected dispatched task: %+v", task)
	}
	updated, err := service.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("load updated job: %v", err)
	}
	if updated.LastRunAt == nil || !updated.LastRunAt.Equal(now) || updated.NextRunAt == nil || !updated.NextRunAt.After(now) {
		t.Fatalf("job run timestamps last=%v next=%v", updated.LastRunAt, updated.NextRunAt)
	}

	taskService := tasksvc.NewService(service.db)
	loadedTask, err := taskService.Get(context.Background(), dispatch.TaskID)
	if err != nil {
		t.Fatalf("task get: %v", err)
	}
	if len(loadedTask.Dispatches) != 1 || loadedTask.Dispatches[0].ID != dispatch.ID {
		t.Fatalf("task dispatch preload = %+v", loadedTask.Dispatches)
	}
}

func TestDispatchDueJobsSkipsDisabledAndDuplicateConcurrency(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	disabled := false
	for _, req := range []JobRequest{
		{Name: "Disabled", Type: "host.disabled", CronExpr: "*/5 * * * *", Enabled: &disabled, TargetType: "host", TargetID: "disabled", ConcurrencyKey: "host:disabled"},
		{Name: "Duplicate", Type: "host.dup", CronExpr: "*/5 * * * *", TargetType: "host", TargetID: "dup", ConcurrencyKey: "host:dup"},
	} {
		job, err := service.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("Create %s: %v", req.Name, err)
		}
		due := now.Add(-time.Minute)
		if err := service.db.Model(&model.ScheduledJob{}).Where("id = ?", job.ID).Update("next_run_at", &due).Error; err != nil {
			t.Fatalf("mark %s due: %v", req.Name, err)
		}
	}
	task := model.Task{ID: "existing-task", Type: "host.dup", Title: "existing", Status: model.TaskStatusPending}
	if err := service.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	existing := model.TaskDispatch{
		ID:             "existing-dispatch",
		TaskID:         task.ID,
		Source:         model.TaskDispatchSourceScheduled,
		Status:         model.TaskDispatchStatusPending,
		ConcurrencyKey: "host:dup",
		QueuedAt:       now.Add(-time.Minute),
	}
	if err := service.db.Create(&existing).Error; err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}

	dispatches, err := service.DispatchDueJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDueJobs: %v", err)
	}
	if len(dispatches) != 0 {
		t.Fatalf("dispatches len = %d, want 0; dispatches=%+v", len(dispatches), dispatches)
	}
}

func newTestService(t *testing.T) (*Service, func()) {
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
