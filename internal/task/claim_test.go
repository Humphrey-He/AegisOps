package task

import (
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

// 竞争窗口说明：SELECT 到 UPDATE 之间被其他实例抢占，在单进程内被
// writeMu 掩盖、无法确定性构造，因此用 claimDispatchTx 的两条用例
// 直接覆盖条件更新的两种结果（抢到 / 没抢到）。

func TestClaimDispatchTxClaimsPendingDispatch(t *testing.T) {
	t.Parallel()

	service, cleanup := newWorkerTestService(t)
	defer cleanup()
	_, dispatch := seedWorkerDispatch(t, service, "scheduled.noop", model.TaskDispatchStatusPending, 0, 0, 60)

	now := time.Now().UTC()
	leaseExpiresAt := now.Add(5 * time.Minute)
	claimed, err := claimDispatchTx(service.db, dispatch, "worker-a", leaseExpiresAt, now)
	if err != nil {
		t.Fatalf("claimDispatchTx: %v", err)
	}
	if !claimed {
		t.Fatal("claimed = false, want true for pending dispatch")
	}

	var updated model.TaskDispatch
	if err := service.db.First(&updated, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updated.Status != model.TaskDispatchStatusRunning || updated.LeaseOwner != "worker-a" || updated.LeaseExpiresAt == nil {
		t.Fatalf("dispatch not claimed: %+v", updated)
	}
}

func TestClaimDispatchTxReturnsFalseWhenNoLongerPending(t *testing.T) {
	t.Parallel()

	service, cleanup := newWorkerTestService(t)
	defer cleanup()
	_, dispatch := seedWorkerDispatch(t, service, "scheduled.noop", model.TaskDispatchStatusPending, 0, 0, 60)

	// 模拟另一个实例在 SELECT 与 UPDATE 之间抢先认领：把状态改成 running。
	stolenBy := "worker-b"
	now := time.Now().UTC()
	if err := service.db.Model(&model.TaskDispatch{}).
		Where("id = ?", dispatch.ID).
		Updates(map[string]any{
			"status":      model.TaskDispatchStatusRunning,
			"lease_owner": stolenBy,
		}).Error; err != nil {
		t.Fatalf("steal dispatch: %v", err)
	}

	claimed, err := claimDispatchTx(service.db, dispatch, "worker-a", now.Add(5*time.Minute), now)
	if err != nil {
		t.Fatalf("claimDispatchTx: %v", err)
	}
	if claimed {
		t.Fatal("claimed = true, want false when dispatch is no longer pending")
	}

	var updated model.TaskDispatch
	if err := service.db.First(&updated, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updated.LeaseOwner != stolenBy {
		t.Fatalf("lease owner changed to %q, want %q", updated.LeaseOwner, stolenBy)
	}
}
