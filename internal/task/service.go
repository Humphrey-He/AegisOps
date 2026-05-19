package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type Service struct {
	db      *gorm.DB
	writeMu sync.Mutex
}

type CreateRequest struct {
	Type       string              `json:"type" binding:"required"`
	Title      string              `json:"title" binding:"required"`
	TargetType string              `json:"targetType"`
	TargetID   string              `json:"targetId"`
	Payload    string              `json:"payload"`
	CreatedBy  string              `json:"-"`
	Steps      []CreateStepRequest `json:"steps"`
}

type CreateStepRequest struct {
	Name      string `json:"name" binding:"required"`
	SortOrder int    `json:"sortOrder"`
}

type ListFilter struct {
	Status     string
	TargetType string
	TargetID   string
}

type DispatchFilter struct {
	Status         string
	Source         string
	JobID          string
	TaskID         string
	ConcurrencyKey string
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Task, error) {
	return s.CreateWithStatus(ctx, req, model.TaskStatusPending)
}

func (s *Service) CreateRunning(ctx context.Context, req CreateRequest) (*model.Task, error) {
	return s.CreateWithStatus(ctx, req, model.TaskStatusRunning)
}

func (s *Service) CreateWithStatus(ctx context.Context, req CreateRequest, status model.TaskStatus) (*model.Task, error) {
	if status == "" {
		status = model.TaskStatusPending
	}
	var startedAt *time.Time
	if status == model.TaskStatusRunning {
		now := time.Now().UTC()
		startedAt = &now
	}
	item := &model.Task{
		ID:         uuid.NewString(),
		Type:       req.Type,
		Title:      req.Title,
		Status:     status,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Payload:    req.Payload,
		CreatedBy:  req.CreatedBy,
		StartedAt:  startedAt,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		for _, stepReq := range req.Steps {
			step := model.TaskStep{
				ID:        uuid.NewString(),
				TaskID:    item.ID,
				Name:      stepReq.Name,
				Status:    model.TaskStatusPending,
				SortOrder: stepReq.SortOrder,
			}
			if err := tx.Create(&step).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, item.ID)
}

func (s *Service) List(ctx context.Context, status string, limit, offset int) ([]model.Task, int64, error) {
	return s.ListWithFilter(ctx, ListFilter{Status: status}, limit, offset)
}

func (s *Service) ListWithFilter(ctx context.Context, filter ListFilter, limit, offset int) ([]model.Task, int64, error) {
	var items []model.Task
	var total int64
	query := s.db.WithContext(ctx).Model(&model.Task{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.TargetType != "" {
		query = query.Where("target_type = ?", filter.TargetType)
	}
	if filter.TargetID != "" {
		query = query.Where("target_id = ?", filter.TargetID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Preload("Dispatches", func(db *gorm.DB) *gorm.DB {
		return db.Order("queued_at DESC, created_at DESC")
	}).Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(ctx context.Context, id string) (*model.Task, error) {
	var item model.Task
	if err := s.db.WithContext(ctx).Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, created_at ASC")
	}).Preload("Logs", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}).Preload("Dispatches", func(db *gorm.DB) *gorm.DB {
		return db.Order("queued_at DESC, created_at DESC")
	}).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) ListDispatches(ctx context.Context, filter DispatchFilter, limit, offset int) ([]model.TaskDispatch, int64, error) {
	var items []model.TaskDispatch
	var total int64
	query := s.db.WithContext(ctx).Model(&model.TaskDispatch{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
	}
	if filter.JobID != "" {
		query = query.Where("job_id = ?", filter.JobID)
	}
	if filter.TaskID != "" {
		query = query.Where("task_id = ?", filter.TaskID)
	}
	if filter.ConcurrencyKey != "" {
		query = query.Where("concurrency_key = ?", filter.ConcurrencyKey)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("queued_at DESC, created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) GetDispatch(ctx context.Context, id string) (*model.TaskDispatch, error) {
	var item model.TaskDispatch
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Start(ctx context.Context, id string) error {
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     model.TaskStatusRunning,
		"started_at": &now,
	}).Error
}

func (s *Service) Finish(ctx context.Context, id string, status model.TaskStatus, result, errMessage string) error {
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"result":      result,
		"error":       errMessage,
		"finished_at": &now,
	}).Error
}

func (s *Service) CancelDispatch(ctx context.Context, id string) (*model.TaskDispatch, error) {
	now := time.Now().UTC()
	var dispatch model.TaskDispatch
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&dispatch, "id = ?", id).Error; err != nil {
			return err
		}
		if isDispatchTerminal(dispatch.Status) {
			return fmt.Errorf("dispatch is already finished")
		}
		if err := tx.Model(&model.TaskDispatch{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":           model.TaskDispatchStatusCanceled,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"finished_at":      &now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Task{}).Where("id = ? AND status IN ?", dispatch.TaskID, []model.TaskStatus{
			model.TaskStatusPending,
			model.TaskStatusRunning,
		}).Updates(map[string]interface{}{
			"status":      model.TaskStatusCanceled,
			"error":       "dispatch canceled by user",
			"finished_at": &now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetDispatch(ctx, id)
}

func (s *Service) RetryDispatch(ctx context.Context, id string) (*model.TaskDispatch, error) {
	now := time.Now().UTC()
	var dispatch model.TaskDispatch
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&dispatch, "id = ?", id).Error; err != nil {
			return err
		}
		if dispatch.Status != model.TaskDispatchStatusFailed &&
			dispatch.Status != model.TaskDispatchStatusTimeout &&
			dispatch.Status != model.TaskDispatchStatusCanceled {
			return fmt.Errorf("only failed, timeout or canceled dispatch can be retried")
		}
		if err := tx.Model(&model.TaskDispatch{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":           model.TaskDispatchStatusPending,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"queued_at":        now,
			"started_at":       nil,
			"finished_at":      nil,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Task{}).Where("id = ?", dispatch.TaskID).Updates(map[string]interface{}{
			"status":      model.TaskStatusPending,
			"error":       "",
			"finished_at": nil,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetDispatch(ctx, id)
}

func (s *Service) Cancel(ctx context.Context, id string) error {
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.Task
		if err := tx.First(&item, "id = ?", id).Error; err != nil {
			return err
		}
		if item.Status == model.TaskStatusSuccess || item.Status == model.TaskStatusFailed || item.Status == model.TaskStatusCanceled {
			return fmt.Errorf("task is already finished")
		}
		if err := tx.Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":      model.TaskStatusCanceled,
			"error":       "canceled by user",
			"finished_at": &now,
		}).Error; err != nil {
			return err
		}
		_ = tx.Model(&model.TaskDispatch{}).Where("task_id = ? AND status IN ?", id, []model.TaskDispatchStatus{
			model.TaskDispatchStatusPending,
			model.TaskDispatchStatusDispatched,
			model.TaskDispatchStatusRunning,
		}).Updates(map[string]interface{}{
			"status":      model.TaskDispatchStatusCanceled,
			"finished_at": &now,
		}).Error
		return nil
	})
}

func isDispatchTerminal(status model.TaskDispatchStatus) bool {
	return status == model.TaskDispatchStatusSuccess ||
		status == model.TaskDispatchStatusFailed ||
		status == model.TaskDispatchStatusCanceled ||
		status == model.TaskDispatchStatusTimeout
}

func (s *Service) Retry(ctx context.Context, id, operatorID string) (*model.Task, error) {
	var original model.Task
	if err := s.db.WithContext(ctx).Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, created_at ASC")
	}).First(&original, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if original.Status != model.TaskStatusFailed && original.Status != model.TaskStatusCanceled {
		return nil, fmt.Errorf("only failed or canceled tasks can be retried")
	}
	steps := make([]CreateStepRequest, 0, len(original.Steps))
	for _, step := range original.Steps {
		steps = append(steps, CreateStepRequest{Name: step.Name, SortOrder: step.SortOrder})
	}
	task, err := s.Create(ctx, CreateRequest{
		Type:       original.Type,
		Title:      original.Title + " retry",
		TargetType: original.TargetType,
		TargetID:   original.TargetID,
		Payload:    original.Payload,
		CreatedBy:  operatorID,
		Steps:      steps,
	})
	if err != nil {
		return nil, err
	}
	dispatch := model.TaskDispatch{
		ID:             uuid.NewString(),
		TaskID:         task.ID,
		Source:         model.TaskDispatchSourceManual,
		Status:         model.TaskDispatchStatusPending,
		RetryCount:     0,
		MaxRetry:       0,
		TimeoutSeconds: 300,
		ConcurrencyKey: original.TargetType + ":" + original.TargetID + ":" + original.Type,
		QueuedAt:       time.Now().UTC(),
	}
	_ = s.db.WithContext(ctx).Create(&dispatch).Error
	return task, nil
}

func (s *Service) AddStep(ctx context.Context, taskID, name string, sortOrder int) (*model.TaskStep, error) {
	step := &model.TaskStep{
		ID:        uuid.NewString(),
		TaskID:    taskID,
		Name:      name,
		Status:    model.TaskStatusPending,
		SortOrder: sortOrder,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return step, s.db.WithContext(ctx).Create(step).Error
}

func (s *Service) UpdateStep(ctx context.Context, stepID string, status model.TaskStatus, result, errMessage string) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status": status,
		"result": result,
		"error":  errMessage,
	}
	if status == model.TaskStatusRunning {
		updates["started_at"] = &now
	}
	if status == model.TaskStatusSuccess || status == model.TaskStatusFailed || status == model.TaskStatusCanceled {
		updates["finished_at"] = &now
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.db.WithContext(ctx).Model(&model.TaskStep{}).Where("id = ?", stepID).Updates(updates).Error
}

func (s *Service) AddLog(ctx context.Context, taskID, stepID string, level model.TaskLogLevel, message string) (*model.TaskLog, error) {
	log := &model.TaskLog{
		ID:      uuid.NewString(),
		TaskID:  taskID,
		StepID:  stepID,
		Level:   level,
		Message: message,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return log, s.db.WithContext(ctx).Create(log).Error
}
