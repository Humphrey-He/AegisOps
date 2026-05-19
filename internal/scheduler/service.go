package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type Service struct {
	db  *gorm.DB
	now func() time.Time
}

type RunOptions struct {
	Interval   time.Duration
	Limit      int
	OnError    func(error)
	OnDispatch func([]model.TaskDispatch)
}

type JobRequest struct {
	Name            string `json:"name" binding:"required"`
	Type            string `json:"type" binding:"required"`
	Enabled         *bool  `json:"enabled"`
	CronExpr        string `json:"cronExpr" binding:"required"`
	TargetType      string `json:"targetType"`
	TargetID        string `json:"targetId"`
	PayloadJSON     string `json:"payloadJson"`
	RetryPolicyJSON string `json:"retryPolicyJson"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
	ConcurrencyKey  string `json:"concurrencyKey"`
	OperatorID      string `json:"-"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now}
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]model.ScheduledJob, int64, error) {
	var items []model.ScheduledJob
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ScheduledJob{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(ctx context.Context, id string) (*model.ScheduledJob, error) {
	var item model.ScheduledJob
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) ListDispatches(ctx context.Context, jobID string, limit, offset int) ([]model.TaskDispatch, int64, error) {
	var items []model.TaskDispatch
	var total int64
	query := s.db.WithContext(ctx).Model(&model.TaskDispatch{}).Where("job_id = ?", jobID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("queued_at DESC, created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Create(ctx context.Context, req JobRequest) (*model.ScheduledJob, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 300
	}
	cronExpr := strings.TrimSpace(req.CronExpr)
	next, err := s.nextRun(cronExpr, s.now().UTC())
	if err != nil {
		return nil, err
	}
	item := &model.ScheduledJob{
		ID:              uuid.NewString(),
		Name:            strings.TrimSpace(req.Name),
		Type:            strings.TrimSpace(req.Type),
		Enabled:         enabled,
		CronExpr:        cronExpr,
		TargetType:      strings.TrimSpace(req.TargetType),
		TargetID:        strings.TrimSpace(req.TargetID),
		PayloadJSON:     strings.TrimSpace(req.PayloadJSON),
		RetryPolicyJSON: strings.TrimSpace(req.RetryPolicyJSON),
		TimeoutSeconds:  timeout,
		ConcurrencyKey:  firstNonEmpty(req.ConcurrencyKey, req.TargetType+":"+req.TargetID+":"+req.Type),
		CreatedBy:       req.OperatorID,
		UpdatedBy:       req.OperatorID,
	}
	if item.Name == "" || item.Type == "" || item.CronExpr == "" {
		return nil, fmt.Errorf("scheduled job name, type and cronExpr are required")
	}
	item.NextRunAt = &next
	if err := s.createScheduledJob(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) Update(ctx context.Context, id string, req JobRequest) (*model.ScheduledJob, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	if req.Type != "" {
		item.Type = strings.TrimSpace(req.Type)
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	if req.CronExpr != "" {
		cronExpr := strings.TrimSpace(req.CronExpr)
		next, err := s.nextRun(cronExpr, s.now().UTC())
		if err != nil {
			return nil, err
		}
		item.CronExpr = cronExpr
		item.NextRunAt = &next
	}
	item.TargetType = strings.TrimSpace(req.TargetType)
	item.TargetID = strings.TrimSpace(req.TargetID)
	item.PayloadJSON = strings.TrimSpace(req.PayloadJSON)
	item.RetryPolicyJSON = strings.TrimSpace(req.RetryPolicyJSON)
	if req.TimeoutSeconds > 0 {
		item.TimeoutSeconds = req.TimeoutSeconds
	}
	if req.ConcurrencyKey != "" {
		item.ConcurrencyKey = strings.TrimSpace(req.ConcurrencyKey)
	}
	item.UpdatedBy = req.OperatorID
	if err := s.db.WithContext(ctx).Save(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DispatchDueJobs(ctx context.Context, limit int) ([]model.TaskDispatch, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	now := s.now().UTC()
	var jobs []model.ScheduledJob
	if err := s.db.WithContext(ctx).Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&jobs).Error; err != nil {
		return nil, err
	}
	dispatches := make([]model.TaskDispatch, 0, len(jobs))
	errs := make([]error, 0)
	for _, job := range jobs {
		dispatch, err := s.enqueueJobRun(ctx, job, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("dispatch scheduled job %s: %w", job.ID, err))
			continue
		}
		if dispatch != nil {
			dispatches = append(dispatches, *dispatch)
		}
	}
	return dispatches, errors.Join(errs...)
}

func (s *Service) Run(ctx context.Context, opts RunOptions) {
	interval := opts.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	runOnce := func() {
		dispatches, err := s.DispatchDueJobs(ctx, limit)
		if err != nil && opts.OnError != nil {
			opts.OnError(err)
		}
		if len(dispatches) > 0 && opts.OnDispatch != nil {
			opts.OnDispatch(dispatches)
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

func (s *Service) enqueueJobRun(ctx context.Context, job model.ScheduledJob, now time.Time) (*model.TaskDispatch, error) {
	var dispatch *model.TaskDispatch
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedJob model.ScheduledJob
		if err := tx.First(&lockedJob, "id = ?", job.ID).Error; err != nil {
			return err
		}
		if !lockedJob.Enabled || lockedJob.NextRunAt == nil || lockedJob.NextRunAt.After(now) {
			return nil
		}
		if lockedJob.ConcurrencyKey != "" {
			var count int64
			if err := tx.Model(&model.TaskDispatch{}).Where("concurrency_key = ? AND status IN ?", lockedJob.ConcurrencyKey, []model.TaskDispatchStatus{
				model.TaskDispatchStatusPending,
				model.TaskDispatchStatusDispatched,
				model.TaskDispatchStatusRunning,
			}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				next, err := s.nextRun(lockedJob.CronExpr, now)
				if err != nil {
					return err
				}
				return tx.Model(&model.ScheduledJob{}).Where("id = ?", lockedJob.ID).Updates(map[string]any{
					"last_run_at": &now,
					"next_run_at": &next,
				}).Error
			}
		}
		task := model.Task{
			ID:         uuid.NewString(),
			Type:       lockedJob.Type,
			Title:      lockedJob.Name,
			Status:     model.TaskStatusPending,
			TargetType: lockedJob.TargetType,
			TargetID:   lockedJob.TargetID,
			Payload:    lockedJob.PayloadJSON,
			CreatedBy:  lockedJob.UpdatedBy,
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		item := model.TaskDispatch{
			ID:             uuid.NewString(),
			TaskID:         task.ID,
			Source:         model.TaskDispatchSourceScheduled,
			JobID:          lockedJob.ID,
			Status:         model.TaskDispatchStatusPending,
			RetryCount:     0,
			MaxRetry:       maxRetryFromPolicy(lockedJob.RetryPolicyJSON),
			TimeoutSeconds: lockedJob.TimeoutSeconds,
			ConcurrencyKey: lockedJob.ConcurrencyKey,
			QueuedAt:       now,
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		next, err := s.nextRun(lockedJob.CronExpr, now)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.ScheduledJob{}).Where("id = ?", lockedJob.ID).Updates(map[string]any{
			"last_run_at": &now,
			"next_run_at": &next,
		}).Error; err != nil {
			return err
		}
		dispatch = &item
		return nil
	})
	return dispatch, err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&model.ScheduledJob{}, "id = ?", id).Error
}

func (s *Service) createScheduledJob(ctx context.Context, item *model.ScheduledJob) error {
	enabled := item.Enabled
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return err
	}
	item.Enabled = enabled
	return s.db.WithContext(ctx).Model(&model.ScheduledJob{}).Where("id = ?", item.ID).Update("enabled", enabled).Error
}

func (s *Service) nextRun(expr string, from time.Time) (time.Time, error) {
	interval, err := parseCronInterval(expr)
	if err != nil {
		return time.Time{}, err
	}
	next := from.Truncate(time.Minute).Add(interval)
	if !next.After(from) {
		next = next.Add(interval)
	}
	return next.UTC(), nil
}

func parseCronInterval(expr string) (time.Duration, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return 0, fmt.Errorf("cronExpr must contain 5 fields")
	}
	if fields[1] != "*" || fields[2] != "*" || fields[3] != "*" || fields[4] != "*" {
		return 0, fmt.Errorf("only minute interval cron expressions are supported")
	}
	minute := fields[0]
	if minute == "*" {
		return time.Minute, nil
	}
	if strings.HasPrefix(minute, "*/") {
		value, err := strconv.Atoi(strings.TrimPrefix(minute, "*/"))
		if err != nil || value <= 0 || value > 1440 {
			return 0, fmt.Errorf("invalid cron minute interval")
		}
		return time.Duration(value) * time.Minute, nil
	}
	value, err := strconv.Atoi(minute)
	if err != nil || value < 0 || value > 59 {
		return 0, fmt.Errorf("invalid cron minute value")
	}
	return time.Hour, nil
}

func maxRetryFromPolicy(policy string) int {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return 0
	}
	for _, part := range strings.FieldsFunc(policy, func(r rune) bool {
		return r == ',' || r == ';' || r == '{' || r == '}' || r == '"' || r == ' '
	}) {
		if strings.HasPrefix(part, "maxRetry:") {
			value, _ := strconv.Atoi(strings.TrimPrefix(part, "maxRetry:"))
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
