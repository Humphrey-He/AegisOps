package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/metrics"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// 断言包级计数器增量，不调 t.Parallel()，与同包其他测试串行执行。
func TestDispatchDueJobsIncrementsSchedulerDispatchMetric(t *testing.T) {
	service, cleanup := newTestService(t)
	t.Cleanup(cleanup)
	now := time.Date(2026, 10, 25, 10, 0, 30, 0, time.UTC)
	service.now = func() time.Time { return now }

	job, err := service.Create(context.Background(), JobRequest{
		Name:     "Metrics Sweep",
		Type:     "noop",
		CronExpr: "*/5 * * * *",
		TargetType: "host",
		TargetID: "host-metrics-1",
	})
	if err != nil {
		t.Fatalf("Create scheduled job: %v", err)
	}
	due := now.Add(-time.Minute)
	if err := service.db.Model(&model.ScheduledJob{}).Where("id = ?", job.ID).Update("next_run_at", &due).Error; err != nil {
		t.Fatalf("mark job due: %v", err)
	}

	before := testutil.ToFloat64(metrics.SchedulerDispatches)
	dispatches, err := service.DispatchDueJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDueJobs: %v", err)
	}
	if len(dispatches) != 1 {
		t.Fatalf("dispatches len = %d, want 1", len(dispatches))
	}
	if delta := testutil.ToFloat64(metrics.SchedulerDispatches) - before; delta != 1 {
		t.Fatalf("scheduler dispatches delta = %v, want 1", delta)
	}
}
