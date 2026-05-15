package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/model"
)

type Service struct {
	db     *gorm.DB
	alerts *alert.Service
	client *http.Client
}

type Policy struct {
	Type               model.HealthCheckStrategyType `json:"type"`
	Target             string                        `json:"target"`
	ExpectedHTTPStatus int                           `json:"expectedHttpStatus"`
	TimeoutSeconds     int                           `json:"timeoutSeconds"`
	RetryTimes         int                           `json:"retryTimes"`
}

func NewService(db *gorm.DB, alerts *alert.Service) *Service {
	return &Service{
		db:     db,
		alerts: alerts,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) RunServiceCheck(ctx context.Context, service model.ServiceDefinition, release model.ServiceReleaseRecord, taskID string) (*model.ServiceHealthCheck, error) {
	policy := parsePolicy(service)
	started := time.Now().UTC()
	status := model.HealthCheckStatusSuccess
	output := "health check passed"
	var httpStatus int
	var errMessage string
	var err error
	if policy.Target == "" {
		output = "no health check target configured; skipped"
	} else {
		httpStatus, output, err = s.execute(ctx, policy)
		if err != nil {
			status = model.HealthCheckStatusFailed
			errMessage = err.Error()
		}
	}
	finished := time.Now().UTC()
	check := &model.ServiceHealthCheck{
		ID:           uuid.NewString(),
		ServiceID:    service.ID,
		ReleaseID:    release.ID,
		TaskID:       taskID,
		StrategyType: policy.Type,
		Target:       policy.Target,
		Status:       status,
		HTTPStatus:   httpStatus,
		LatencyMs:    finished.Sub(started).Milliseconds(),
		Output:       output,
		ErrorMessage: errMessage,
		StartedAt:    started,
		FinishedAt:   finished,
	}
	if err := s.db.WithContext(ctx).Create(check).Error; err != nil {
		return nil, err
	}
	if status == model.HealthCheckStatusFailed {
		suggestion := s.rollbackSuggestion(ctx, service.ID, release.TargetVersionID)
		_, _ = s.alerts.CreateEvent(ctx, alert.EventRequest{
			EventType:    "service_health_check_failed",
			ResourceType: "service",
			ResourceID:   service.ID,
			TaskID:       taskID,
			ReleaseID:    release.ID,
			Severity:     model.AlertEventSeverityCritical,
			Summary:      "service health check failed: " + service.Code,
			Detail:       errMessage,
			DedupeKey:    "service_health_check_failed:" + service.ID,
			Suggestion:   suggestion,
		})
		return check, errors.New(errMessage)
	}
	return check, nil
}

func (s *Service) ListServiceChecks(ctx context.Context, serviceID string, limit, offset int) ([]model.ServiceHealthCheck, int64, error) {
	var items []model.ServiceHealthCheck
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ServiceHealthCheck{}).Where("service_id = ?", serviceID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) ListReleaseChecks(ctx context.Context, releaseID string, limit, offset int) ([]model.ServiceHealthCheck, int64, error) {
	var items []model.ServiceHealthCheck
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ServiceHealthCheck{}).Where("release_id = ?", releaseID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) RecordHostAvailability(ctx context.Context, hostID, taskID string, started time.Time, err error) (*model.HostAvailabilityCheck, error) {
	status := model.HostAvailabilityStatusOnline
	reason := ""
	if err != nil {
		status = model.HostAvailabilityStatusUnreachable
		reason = err.Error()
	}
	finished := time.Now().UTC()
	check := &model.HostAvailabilityCheck{
		ID:            uuid.NewString(),
		HostID:        hostID,
		TaskID:        taskID,
		Status:        status,
		FailureReason: reason,
		StartedAt:     started,
		FinishedAt:    finished,
	}
	if dbErr := s.db.WithContext(ctx).Create(check).Error; dbErr != nil {
		return nil, dbErr
	}
	if err != nil {
		_, _ = s.alerts.CreateEvent(ctx, alert.EventRequest{
			EventType:    "host_offline",
			ResourceType: "host",
			ResourceID:   hostID,
			TaskID:       taskID,
			Severity:     model.AlertEventSeverityCritical,
			Summary:      "host unreachable",
			Detail:       reason,
			DedupeKey:    "host_offline:" + hostID,
		})
	} else {
		_ = s.resolveHostOffline(ctx, hostID)
	}
	return check, nil
}

func (s *Service) ListHostAvailability(ctx context.Context, hostID string, limit, offset int) ([]model.HostAvailabilityCheck, int64, error) {
	var items []model.HostAvailabilityCheck
	var total int64
	query := s.db.WithContext(ctx).Model(&model.HostAvailabilityCheck{}).Where("host_id = ?", hostID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) RollbackSuggestion(ctx context.Context, serviceID string) (map[string]interface{}, error) {
	var current model.ServiceDefinition
	if err := s.db.WithContext(ctx).First(&current, "id = ?", serviceID).Error; err != nil {
		return nil, err
	}
	var version model.ServiceVersion
	err := s.db.WithContext(ctx).Where("service_id = ? AND version <> ?", serviceID, current.CurrentVersion).Order("created_at DESC").First(&version).Error
	if err != nil {
		return map[string]interface{}{"available": false, "reason": "no previous version found"}, nil
	}
	return map[string]interface{}{"available": true, "versionId": version.ID, "version": version.Version, "imageTag": version.ImageTag}, nil
}

func (s *Service) execute(ctx context.Context, policy Policy) (int, string, error) {
	retries := policy.RetryTimes
	if retries <= 0 {
		retries = 1
	}
	var lastErr error
	var status int
	var output string
	for i := 0; i < retries; i++ {
		status, output, lastErr = s.executeOnce(ctx, policy)
		if lastErr == nil {
			return status, output, nil
		}
	}
	return status, output, lastErr
}

func (s *Service) executeOnce(ctx context.Context, policy Policy) (int, string, error) {
	timeout := time.Duration(policy.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	switch policy.Type {
	case model.HealthCheckStrategyTCP:
		conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", policy.Target)
		if err != nil {
			return 0, "", err
		}
		_ = conn.Close()
		return 0, "tcp connected", nil
	case model.HealthCheckStrategyHTTP, "":
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, policy.Target, nil)
		if err != nil {
			return 0, "", err
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return 0, "", err
		}
		defer resp.Body.Close()
		expected := policy.ExpectedHTTPStatus
		if expected == 0 {
			expected = http.StatusOK
		}
		if resp.StatusCode != expected {
			return resp.StatusCode, "", fmt.Errorf("unexpected http status %d, want %d", resp.StatusCode, expected)
		}
		return resp.StatusCode, "http status " + strconv.Itoa(resp.StatusCode), nil
	default:
		return 0, "", fmt.Errorf("unsupported health check strategy: %s", policy.Type)
	}
}

func parsePolicy(service model.ServiceDefinition) Policy {
	policy := Policy{Type: model.HealthCheckStrategyHTTP, RetryTimes: 1, TimeoutSeconds: 10}
	if strings.TrimSpace(service.Envs) == "" {
		return policy
	}
	var envs []map[string]interface{}
	if err := json.Unmarshal([]byte(service.Envs), &envs); err != nil {
		return policy
	}
	for _, env := range envs {
		name, _ := env["name"].(string)
		if name != "AEGISOPS_HEALTHCHECK" {
			continue
		}
		value, _ := env["value"].(string)
		_ = json.Unmarshal([]byte(value), &policy)
	}
	if policy.Type == "" {
		policy.Type = model.HealthCheckStrategyHTTP
	}
	return policy
}

func (s *Service) rollbackSuggestion(ctx context.Context, serviceID, excludeVersionID string) string {
	var version model.ServiceVersion
	query := s.db.WithContext(ctx).Where("service_id = ?", serviceID)
	if excludeVersionID != "" {
		query = query.Where("id <> ?", excludeVersionID)
	}
	if err := query.Order("created_at DESC").First(&version).Error; err != nil {
		return ""
	}
	payload, _ := json.Marshal(map[string]string{"versionId": version.ID, "version": version.Version, "imageTag": version.ImageTag})
	return string(payload)
}

func (s *Service) resolveHostOffline(ctx context.Context, hostID string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&model.AlertEvent{}).
		Where("event_type = ? AND resource_type = ? AND resource_id = ? AND status IN ?", "host_offline", "host", hostID, []model.AlertEventStatus{model.AlertEventStatusOpen, model.AlertEventStatusAcked}).
		Updates(map[string]interface{}{"status": model.AlertEventStatusResolved, "resolved_at": &now}).Error
}
