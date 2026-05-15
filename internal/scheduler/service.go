package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type Service struct {
	db *gorm.DB
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
	return &Service{db: db}
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

func (s *Service) Create(ctx context.Context, req JobRequest) (*model.ScheduledJob, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 300
	}
	item := &model.ScheduledJob{
		ID:              uuid.NewString(),
		Name:            strings.TrimSpace(req.Name),
		Type:            strings.TrimSpace(req.Type),
		Enabled:         enabled,
		CronExpr:        strings.TrimSpace(req.CronExpr),
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
	next := time.Now().UTC().Add(time.Minute)
	item.NextRunAt = &next
	return item, s.db.WithContext(ctx).Create(item).Error
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
		item.CronExpr = strings.TrimSpace(req.CronExpr)
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

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&model.ScheduledJob{}, "id = ?", id).Error
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
