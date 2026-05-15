package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/notification"
)

type Service struct {
	db            *gorm.DB
	notifications *notification.Service
}

type RuleRequest struct {
	Name                string `json:"name" binding:"required"`
	EventType           string `json:"eventType" binding:"required"`
	ResourceType        string `json:"resourceType"`
	ResourceScope       string `json:"resourceScope"`
	ChannelIDs          string `json:"channelIds"`
	Language            string `json:"language"`
	Enabled             *bool  `json:"enabled"`
	DedupeWindowSeconds int    `json:"dedupeWindowSeconds"`
	RequireAck          bool   `json:"requireAck"`
	OperatorID          string `json:"-"`
}

type EventRequest struct {
	EventType    string                   `json:"eventType" binding:"required"`
	ResourceType string                   `json:"resourceType"`
	ResourceID   string                   `json:"resourceId"`
	TaskID       string                   `json:"taskId"`
	ReleaseID    string                   `json:"releaseId"`
	Severity     model.AlertEventSeverity `json:"severity"`
	Summary      string                   `json:"summary"`
	Detail       string                   `json:"detail"`
	DedupeKey    string                   `json:"dedupeKey"`
	Suggestion   string                   `json:"suggestion"`
}

func NewService(db *gorm.DB, notifications *notification.Service) *Service {
	return &Service{db: db, notifications: notifications}
}

func (s *Service) ListRules(ctx context.Context, limit, offset int) ([]model.AlertRule, int64, error) {
	var items []model.AlertRule
	var total int64
	query := s.db.WithContext(ctx).Model(&model.AlertRule{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) CreateRule(ctx context.Context, req RuleRequest) (*model.AlertRule, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.DedupeWindowSeconds <= 0 {
		req.DedupeWindowSeconds = 300
	}
	item := &model.AlertRule{
		ID:                  uuid.NewString(),
		Name:                strings.TrimSpace(req.Name),
		EventType:           strings.TrimSpace(req.EventType),
		ResourceType:        strings.TrimSpace(req.ResourceType),
		ResourceScope:       strings.TrimSpace(req.ResourceScope),
		ChannelIDs:          strings.TrimSpace(req.ChannelIDs),
		Language:            normalizeRuleLanguage(req.Language),
		Enabled:             enabled,
		DedupeWindowSeconds: req.DedupeWindowSeconds,
		RequireAck:          req.RequireAck,
		CreatedBy:           req.OperatorID,
		UpdatedBy:           req.OperatorID,
	}
	if item.Name == "" || item.EventType == "" {
		return nil, fmt.Errorf("alert rule name and event type are required")
	}
	return item, s.db.WithContext(ctx).Create(item).Error
}

func (s *Service) UpdateRule(ctx context.Context, id string, req RuleRequest) (*model.AlertRule, error) {
	var item model.AlertRule
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	if req.EventType != "" {
		item.EventType = strings.TrimSpace(req.EventType)
	}
	item.ResourceType = strings.TrimSpace(req.ResourceType)
	item.ResourceScope = strings.TrimSpace(req.ResourceScope)
	item.ChannelIDs = strings.TrimSpace(req.ChannelIDs)
	item.Language = normalizeRuleLanguage(req.Language)
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	if req.DedupeWindowSeconds > 0 {
		item.DedupeWindowSeconds = req.DedupeWindowSeconds
	}
	item.RequireAck = req.RequireAck
	item.UpdatedBy = req.OperatorID
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DeleteRule(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&model.AlertRule{}, "id = ?", id).Error
}

func (s *Service) CreateEvent(ctx context.Context, req EventRequest) (*model.AlertEvent, error) {
	if req.Severity == "" {
		req.Severity = model.AlertEventSeverityWarning
	}
	if req.DedupeKey == "" {
		req.DedupeKey = strings.Join([]string{req.EventType, req.ResourceType, req.ResourceID}, ":")
	}
	now := time.Now().UTC()
	var existing model.AlertEvent
	err := s.db.WithContext(ctx).Where("dedupe_key = ? AND status IN ?", req.DedupeKey, []model.AlertEventStatus{model.AlertEventStatusOpen, model.AlertEventStatusAcked}).Order("created_at DESC").First(&existing).Error
	if err == nil {
		existing.LastTriggeredAt = now
		existing.Detail = req.Detail
		existing.Summary = firstNonEmpty(req.Summary, existing.Summary)
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	item := &model.AlertEvent{
		ID:               uuid.NewString(),
		EventType:        strings.TrimSpace(req.EventType),
		ResourceType:     strings.TrimSpace(req.ResourceType),
		ResourceID:       strings.TrimSpace(req.ResourceID),
		TaskID:           strings.TrimSpace(req.TaskID),
		ReleaseID:        strings.TrimSpace(req.ReleaseID),
		Severity:         req.Severity,
		Status:           model.AlertEventStatusOpen,
		Summary:          firstNonEmpty(req.Summary, req.EventType),
		Detail:           req.Detail,
		DedupeKey:        req.DedupeKey,
		Suggestion:       req.Suggestion,
		FirstTriggeredAt: now,
		LastTriggeredAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	_ = s.dispatch(ctx, *item)
	return item, nil
}

func (s *Service) ListEvents(ctx context.Context, status, eventType string, limit, offset int) ([]model.AlertEvent, int64, error) {
	var items []model.AlertEvent
	var total int64
	query := s.db.WithContext(ctx).Model(&model.AlertEvent{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("last_triggered_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) GetEvent(ctx context.Context, id string) (*model.AlertEvent, error) {
	var item model.AlertEvent
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) AckEvent(ctx context.Context, id, operatorID string) (*model.AlertEvent, error) {
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&model.AlertEvent{}).Where("id = ?", id).Updates(map[string]interface{}{"status": model.AlertEventStatusAcked, "acked_at": &now, "acked_by": operatorID}).Error; err != nil {
		return nil, err
	}
	return s.GetEvent(ctx, id)
}

func (s *Service) ResolveEvent(ctx context.Context, id, operatorID string) (*model.AlertEvent, error) {
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&model.AlertEvent{}).Where("id = ?", id).Updates(map[string]interface{}{"status": model.AlertEventStatusResolved, "resolved_at": &now, "resolved_by": operatorID}).Error; err != nil {
		return nil, err
	}
	return s.GetEvent(ctx, id)
}

func (s *Service) Records(ctx context.Context, limit, offset int) ([]model.NotificationRecord, int64, error) {
	return s.notifications.Records(ctx, limit, offset)
}

func (s *Service) dispatch(ctx context.Context, event model.AlertEvent) error {
	if s.notifications == nil {
		return nil
	}
	var rules []model.AlertRule
	if err := s.db.WithContext(ctx).Where("enabled = ? AND event_type = ?", true, event.EventType).Find(&rules).Error; err != nil {
		return err
	}
	for _, rule := range rules {
		channelIDs := parseIDs(rule.ChannelIDs)
		var channels []model.NotificationChannel
		query := s.db.WithContext(ctx).Where("enabled = ?", true)
		if len(channelIDs) > 0 {
			query = query.Where("id IN ?", channelIDs)
		}
		if err := query.Find(&channels).Error; err != nil {
			return err
		}
		for _, channel := range channels {
			language := firstNonEmpty(rule.Language, channel.Language, notification.LanguageChinese)
			_, _ = s.notifications.Send(ctx, channel, notification.SendRequest{
				EventID:      event.ID,
				EventType:    event.EventType,
				Severity:     string(event.Severity),
				Subject:      "[" + string(event.Severity) + "] " + event.Summary,
				Body:         event.Detail,
				ResourceType: event.ResourceType,
				ResourceID:   event.ResourceID,
				TaskID:       event.TaskID,
				ReleaseID:    event.ReleaseID,
				Suggestion:   event.Suggestion,
				Language:     language,
				TriggeredAt:  event.LastTriggeredAt,
			})
		}
	}
	return nil
}

func parseIDs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var items []string
	if strings.HasPrefix(value, "[") {
		_ = json.Unmarshal([]byte(value), &items)
		return items
	}
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func normalizeRuleLanguage(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return notification.NormalizeLanguage(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
