package task

import (
	"context"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/metrics"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// 本文件的用例断言包级计数器增量，不调 t.Parallel()：
// 需要与同包其他 worker 测试串行执行，避免并发打点干扰断言。

func TestWorkerMetricsCountClaimAndSuccessCompletion(t *testing.T) {
	service, cleanup := newWorkerTestService(t)
	t.Cleanup(cleanup)
	worker := NewWorker(service)

	seedWorkerDispatch(t, service, "noop", model.TaskDispatchStatusPending, 0, 0, 60)

	claimsWonBefore := testutil.ToFloat64(metrics.DispatchClaims.WithLabelValues("won"))
	successBefore := testutil.ToFloat64(metrics.DispatchCompletions.WithLabelValues(string(model.TaskDispatchStatusSuccess)))

	processed, err := worker.RunOnce(context.Background(), WorkerOptions{})
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	if delta := testutil.ToFloat64(metrics.DispatchClaims.WithLabelValues("won")) - claimsWonBefore; delta != 1 {
		t.Fatalf("claims won delta = %v, want 1", delta)
	}
	if delta := testutil.ToFloat64(metrics.DispatchCompletions.WithLabelValues(string(model.TaskDispatchStatusSuccess))) - successBefore; delta != 1 {
		t.Fatalf("success completions delta = %v, want 1", delta)
	}
	if count := testutil.CollectAndCount(metrics.DispatchDuration); count == 0 {
		t.Fatalf("dispatch duration histogram has no observations after execution")
	}
}

func TestWorkerMetricsCountRetryAndFailedCompletion(t *testing.T) {
	service, cleanup := newWorkerTestService(t)
	t.Cleanup(cleanup)
	worker := NewWorker(service)

	// 不支持的任务类型 + MaxRetry=1：第一轮执行失败进入重试，第二轮重试耗尽落 FAILED。
	seedWorkerDispatch(t, service, "metrics.unsupported", model.TaskDispatchStatusPending, 0, 1, 60)

	retriesBefore := testutil.ToFloat64(metrics.DispatchRetries)
	failedBefore := testutil.ToFloat64(metrics.DispatchCompletions.WithLabelValues(string(model.TaskDispatchStatusFailed)))

	if _, err := worker.RunOnce(context.Background(), WorkerOptions{}); err != nil {
		t.Fatalf("first RunOnce: %v", err)
	}
	if delta := testutil.ToFloat64(metrics.DispatchRetries) - retriesBefore; delta != 1 {
		t.Fatalf("retries delta after first run = %v, want 1", delta)
	}

	if _, err := worker.RunOnce(context.Background(), WorkerOptions{}); err != nil {
		t.Fatalf("second RunOnce: %v", err)
	}
	if delta := testutil.ToFloat64(metrics.DispatchCompletions.WithLabelValues(string(model.TaskDispatchStatusFailed))) - failedBefore; delta != 1 {
		t.Fatalf("failed completions delta after retry exhausted = %v, want 1", delta)
	}
}
