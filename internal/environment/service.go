package environment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type Service struct {
	db *gorm.DB
}

type CreateRequest struct {
	Name        string                  `json:"name" binding:"required"`
	Code        string                  `json:"code" binding:"required"`
	Description string                  `json:"description"`
	Status      model.EnvironmentStatus `json:"status"`
	SortOrder   int                     `json:"sortOrder"`
	OperatorID  string                  `json:"-"`
}

type UpdateRequest struct {
	Name        string                  `json:"name"`
	Code        string                  `json:"code"`
	Description string                  `json:"description"`
	Status      model.EnvironmentStatus `json:"status"`
	SortOrder   int                     `json:"sortOrder"`
	OperatorID  string                  `json:"-"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func EnsureActive(ctx context.Context, db *gorm.DB, code string) (string, error) {
	code = normalizeCode(code)
	if code == "" {
		return "", nil
	}
	var item model.Environment
	if err := db.WithContext(ctx).First(&item, "code = ?", code).Error; err != nil {
		return "", fmt.Errorf("environment %s not found", code)
	}
	if item.Status != model.EnvironmentStatusActive {
		return "", fmt.Errorf("environment %s is not active", code)
	}
	return item.Code, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Environment, error) {
	code := normalizeCode(req.Code)
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("environment name is required")
	}
	if code == "" {
		return nil, fmt.Errorf("environment code is required")
	}
	if req.Status == "" {
		req.Status = model.EnvironmentStatusActive
	}
	if err := validateStatus(req.Status); err != nil {
		return nil, err
	}
	item := &model.Environment{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(req.Name),
		Code:        code,
		Description: req.Description,
		Status:      req.Status,
		SortOrder:   req.SortOrder,
		CreatedBy:   req.OperatorID,
		UpdatedBy:   req.OperatorID,
	}
	return item, s.db.WithContext(ctx).Create(item).Error
}

func (s *Service) List(ctx context.Context, keyword, status string, limit, offset int) ([]model.Environment, int64, error) {
	var items []model.Environment
	var total int64
	query := s.db.WithContext(ctx).Model(&model.Environment{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", like, like, like)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("sort_order ASC, created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(ctx context.Context, id string) (*model.Environment, error) {
	var item model.Environment
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*model.Environment, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	if req.Code != "" {
		item.Code = normalizeCode(req.Code)
	}
	item.Description = req.Description
	if req.Status != "" {
		if err := validateStatus(req.Status); err != nil {
			return nil, err
		}
		item.Status = req.Status
	}
	item.SortOrder = req.SortOrder
	item.UpdatedBy = req.OperatorID
	return item, s.db.WithContext(ctx).Save(item).Error
}

func (s *Service) Delete(ctx context.Context, id string) error {
	item, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if count, err := s.referenceCount(ctx, item.Code); err != nil {
		return err
	} else if count > 0 {
		return fmt.Errorf("environment %s is referenced by %d resources", item.Code, count)
	}
	return s.db.WithContext(ctx).Delete(&model.Environment{}, "id = ?", id).Error
}

func (s *Service) referenceCount(ctx context.Context, code string) (int64, error) {
	var total int64
	checks := []any{
		&model.Host{},
		&model.DockerNode{},
		&model.Registry{},
		&model.ServiceDefinition{},
		&model.NginxNode{},
	}
	for _, target := range checks {
		var count int64
		if err := s.db.WithContext(ctx).Model(target).Where("environment = ?", code).Count(&count).Error; err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func normalizeCode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateStatus(status model.EnvironmentStatus) error {
	switch status {
	case model.EnvironmentStatusActive, model.EnvironmentStatusDisabled:
		return nil
	default:
		return errors.New("unsupported environment status")
	}
}
