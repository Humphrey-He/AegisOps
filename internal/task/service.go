package task

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type Service struct {
	db *gorm.DB
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

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Task, error) {
	item := &model.Task{
		ID:         uuid.NewString(),
		Type:       req.Type,
		Title:      req.Title,
		Status:     model.TaskStatusPending,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Payload:    req.Payload,
		CreatedBy:  req.CreatedBy,
	}
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
	var items []model.Task
	var total int64
	query := s.db.WithContext(ctx).Model(&model.Task{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(ctx context.Context, id string) (*model.Task, error) {
	var item model.Task
	if err := s.db.WithContext(ctx).Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, created_at ASC")
	}).Preload("Logs", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Start(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     model.TaskStatusRunning,
		"started_at": &now,
	}).Error
}

func (s *Service) Finish(ctx context.Context, id string, status model.TaskStatus, result, errMessage string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"result":      result,
		"error":       errMessage,
		"finished_at": &now,
	}).Error
}

func (s *Service) AddStep(ctx context.Context, taskID, name string, sortOrder int) (*model.TaskStep, error) {
	step := &model.TaskStep{
		ID:        uuid.NewString(),
		TaskID:    taskID,
		Name:      name,
		Status:    model.TaskStatusPending,
		SortOrder: sortOrder,
	}
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
	return log, s.db.WithContext(ctx).Create(log).Error
}
