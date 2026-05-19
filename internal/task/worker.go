package task

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type DispatchExecutor func(context.Context, model.Task, model.TaskDispatch) (string, error)

type WorkerOptions struct {
	Interval      time.Duration
	Limit         int
	LeaseDuration time.Duration
	Owner         string
	Executor      DispatchExecutor
	OnError       func(error)
	OnComplete    func(model.TaskDispatch)
}

type Worker struct {
	service *Service
	now     func() time.Time
}

func NewWorker(service *Service) *Worker {
	return &Worker{service: service, now: time.Now}
}

func (w *Worker) Run(ctx context.Context, opts WorkerOptions) {
	interval := opts.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	runOnce := func() {
		if _, err := w.RunOnce(ctx, opts); err != nil && opts.OnError != nil {
			opts.OnError(err)
		}
	}
	runOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context, opts WorkerOptions) (int, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	owner := strings.TrimSpace(opts.Owner)
	if owner == "" {
		hostname, _ := os.Hostname()
		owner = firstNonEmptyString(hostname, "local-worker")
	}
	leaseDuration := opts.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = 5 * time.Minute
	}
	executor := opts.Executor
	if executor == nil {
		executor = defaultDispatchExecutor
	}

	var errs []error
	processed := 0
	cutoff := w.now().UTC()
	for processed < limit {
		task, dispatch, err := w.claimNext(ctx, owner, leaseDuration, cutoff)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			errs = append(errs, err)
			break
		}
		processed++
		if err := w.execute(ctx, task, dispatch, executor); err != nil {
			errs = append(errs, err)
		}
		if opts.OnComplete != nil {
			latest := dispatch
			_ = w.service.db.WithContext(ctx).First(&latest, "id = ?", dispatch.ID).Error
			opts.OnComplete(latest)
		}
	}
	return processed, errors.Join(errs...)
}

func (w *Worker) claimNext(ctx context.Context, owner string, leaseDuration time.Duration, cutoff time.Time) (model.Task, model.TaskDispatch, error) {
	now := w.now().UTC()
	leaseExpiresAt := now.Add(leaseDuration)
	var task model.Task
	var dispatch model.TaskDispatch
	w.service.writeMu.Lock()
	defer w.service.writeMu.Unlock()
	err := w.service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ? AND queued_at <= ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", model.TaskDispatchStatusPending, cutoff, now).
			Order("queued_at ASC, created_at ASC").
			First(&dispatch).Error; err != nil {
			return err
		}
		if err := tx.First(&task, "id = ?", dispatch.TaskID).Error; err != nil {
			return err
		}
		if isTaskTerminal(task.Status) {
			return tx.Model(&model.TaskDispatch{}).Where("id = ?", dispatch.ID).Updates(map[string]any{
				"status":      model.TaskDispatchStatusCanceled,
				"finished_at": &now,
			}).Error
		}
		if err := tx.Model(&model.TaskDispatch{}).Where("id = ? AND status = ?", dispatch.ID, model.TaskDispatchStatusPending).Updates(map[string]any{
			"status":           model.TaskDispatchStatusRunning,
			"lease_owner":      owner,
			"lease_expires_at": &leaseExpiresAt,
			"started_at":       &now,
		}).Error; err != nil {
			return err
		}
		if task.Status == model.TaskStatusPending {
			if err := tx.Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
				"status":     model.TaskStatusRunning,
				"started_at": &now,
			}).Error; err != nil {
				return err
			}
			task.Status = model.TaskStatusRunning
			task.StartedAt = &now
		}
		dispatch.Status = model.TaskDispatchStatusRunning
		dispatch.LeaseOwner = owner
		dispatch.LeaseExpiresAt = &leaseExpiresAt
		dispatch.StartedAt = &now
		return nil
	})
	return task, dispatch, err
}

func (w *Worker) execute(ctx context.Context, task model.Task, dispatch model.TaskDispatch, executor DispatchExecutor) error {
	timeout := time.Duration(dispatch.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := executor(execCtx, task, dispatch)
	if err == nil && execCtx.Err() != nil {
		err = execCtx.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return w.finishDispatch(ctx, task.ID, dispatch, model.TaskDispatchStatusTimeout, model.TaskStatusFailed, result, "dispatch timed out")
	}
	if err != nil {
		if dispatch.RetryCount < dispatch.MaxRetry {
			return w.retryDispatch(ctx, dispatch, err.Error())
		}
		return w.finishDispatch(ctx, task.ID, dispatch, model.TaskDispatchStatusFailed, model.TaskStatusFailed, result, err.Error())
	}
	return w.finishDispatch(ctx, task.ID, dispatch, model.TaskDispatchStatusSuccess, model.TaskStatusSuccess, result, "")
}

func (w *Worker) retryDispatch(ctx context.Context, dispatch model.TaskDispatch, errMessage string) error {
	now := w.now().UTC()
	w.service.writeMu.Lock()
	defer w.service.writeMu.Unlock()
	return w.service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TaskDispatch{}).Where("id = ?", dispatch.ID).Updates(map[string]any{
			"status":           model.TaskDispatchStatusPending,
			"retry_count":      dispatch.RetryCount + 1,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"started_at":       nil,
			"finished_at":      nil,
			"queued_at":        now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Task{}).Where("id = ?", dispatch.TaskID).Updates(map[string]any{
			"status": model.TaskStatusPending,
			"error":  errMessage,
		}).Error
	})
}

func (w *Worker) finishDispatch(ctx context.Context, taskID string, dispatch model.TaskDispatch, dispatchStatus model.TaskDispatchStatus, taskStatus model.TaskStatus, result, errMessage string) error {
	now := w.now().UTC()
	w.service.writeMu.Lock()
	defer w.service.writeMu.Unlock()
	return w.service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TaskDispatch{}).Where("id = ?", dispatch.ID).Updates(map[string]any{
			"status":           dispatchStatus,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"finished_at":      &now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
			"status":      taskStatus,
			"result":      result,
			"error":       errMessage,
			"finished_at": &now,
		}).Error
	})
}

func defaultDispatchExecutor(ctx context.Context, task model.Task, dispatch model.TaskDispatch) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	switch strings.TrimSpace(strings.ToLower(task.Type)) {
	case "noop", "system.noop", "scheduled.noop":
		return fmt.Sprintf("dispatch %s completed by local worker", dispatch.ID), nil
	default:
		return "", fmt.Errorf("unsupported task type %q for local dispatch worker", task.Type)
	}
}

func isTaskTerminal(status model.TaskStatus) bool {
	return status == model.TaskStatusSuccess || status == model.TaskStatusFailed || status == model.TaskStatusCanceled
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
