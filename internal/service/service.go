package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	dockersvc "github.com/Humphrey-He/AegisOps/internal/docker"
	"github.com/Humphrey-He/AegisOps/internal/model"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
)

var (
	ErrServiceCodeExists = errors.New("service code already exists")
	ErrNoCurrentVersion  = errors.New("service has no current version")
	ErrReleaseInProgress = errors.New("service release is already running")
)

type Service struct {
	db        *gorm.DB
	tasks     *tasksvc.Service
	executor  ReleaseExecutor
	releaseMu sync.Mutex
}

type ReleaseExecutor interface {
	Validate(ctx context.Context, req DeployRequest) error
	Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error)
}

type DeployRequest struct {
	NodeID        string
	ServiceID     string
	ServiceCode   string
	ContainerName string
	Image         string
	ImageTag      string
	PortsJSON     string
	EnvsJSON      string
	MountsJSON    string
	ResourcesJSON string
}

type DeployResult struct {
	ContainerID string `json:"containerId"`
	Image       string `json:"image"`
}

type DockerReleaseExecutor struct {
	docker *dockersvc.Service
}

func NewDockerReleaseExecutor(dockerService *dockersvc.Service) *DockerReleaseExecutor {
	return &DockerReleaseExecutor{docker: dockerService}
}

func (e *DockerReleaseExecutor) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	result, err := e.docker.DeployContainer(ctx, dockersvc.DeployRequest{
		NodeID:        req.NodeID,
		ServiceID:     req.ServiceID,
		ServiceCode:   req.ServiceCode,
		ContainerName: req.ContainerName,
		Image:         req.Image,
		ImageTag:      req.ImageTag,
		PortsJSON:     req.PortsJSON,
		EnvsJSON:      req.EnvsJSON,
		MountsJSON:    req.MountsJSON,
		ResourcesJSON: req.ResourcesJSON,
	})
	if err != nil {
		return nil, err
	}
	return &DeployResult{ContainerID: result.ContainerID, Image: result.Image}, nil
}

func (e *DockerReleaseExecutor) Validate(ctx context.Context, req DeployRequest) error {
	return e.docker.ValidateDeploy(ctx, dockersvc.DeployRequest{
		NodeID:        req.NodeID,
		ServiceID:     req.ServiceID,
		ServiceCode:   req.ServiceCode,
		ContainerName: req.ContainerName,
		Image:         req.Image,
		ImageTag:      req.ImageTag,
		PortsJSON:     req.PortsJSON,
		EnvsJSON:      req.EnvsJSON,
		MountsJSON:    req.MountsJSON,
		ResourcesJSON: req.ResourcesJSON,
	})
}

type NoopReleaseExecutor struct{}

func (NoopReleaseExecutor) Validate(_ context.Context, req DeployRequest) error {
	if strings.TrimSpace(req.NodeID) == "" {
		return fmt.Errorf("target docker node is required")
	}
	if strings.TrimSpace(req.Image) == "" {
		return fmt.Errorf("service image is required")
	}
	return nil
}

func (NoopReleaseExecutor) Deploy(_ context.Context, req DeployRequest) (*DeployResult, error) {
	return &DeployResult{
		ContainerID: "noop-" + uuid.NewString(),
		Image:       imageRef(req.Image, req.ImageTag),
	}, nil
}

type CreateRequest struct {
	Name           string              `json:"name" binding:"required"`
	Code           string              `json:"code" binding:"required"`
	Group          string              `json:"group"`
	Tags           string              `json:"tags"`
	Description    string              `json:"description"`
	RegistryID     string              `json:"registryId"`
	Image          string              `json:"image" binding:"required"`
	DefaultTag     string              `json:"defaultTag"`
	Ports          json.RawMessage     `json:"ports"`
	Envs           json.RawMessage     `json:"envs"`
	Mounts         json.RawMessage     `json:"mounts"`
	ResourceLimits json.RawMessage     `json:"resourceLimits"`
	TargetType     string              `json:"targetType"`
	TargetID       string              `json:"targetId"`
	Status         model.ServiceStatus `json:"status"`
	OperatorID     string              `json:"-"`
}

type UpdateRequest struct {
	Name           string              `json:"name"`
	Group          string              `json:"group"`
	Tags           string              `json:"tags"`
	Description    string              `json:"description"`
	RegistryID     string              `json:"registryId"`
	Image          string              `json:"image"`
	DefaultTag     string              `json:"defaultTag"`
	Ports          json.RawMessage     `json:"ports"`
	Envs           json.RawMessage     `json:"envs"`
	Mounts         json.RawMessage     `json:"mounts"`
	ResourceLimits json.RawMessage     `json:"resourceLimits"`
	TargetType     string              `json:"targetType"`
	TargetID       string              `json:"targetId"`
	Status         model.ServiceStatus `json:"status"`
	OperatorID     string              `json:"-"`
}

type ReleaseRequest struct {
	ImageTag    string `json:"imageTag"`
	ImageDigest string `json:"imageDigest"`
	Version     string `json:"version"`
	TargetID    string `json:"targetId"`
	OperatorID  string `json:"-"`
}

type RollbackRequest struct {
	VersionID  string `json:"versionId" binding:"required"`
	OperatorID string `json:"-"`
}

type ReleaseResult struct {
	TaskID    string `json:"taskId"`
	ReleaseID string `json:"releaseId"`
}

func NewService(db *gorm.DB, tasks *tasksvc.Service, executor ReleaseExecutor) *Service {
	if executor == nil {
		executor = NoopReleaseExecutor{}
	}
	return &Service{db: db, tasks: tasks, executor: executor}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.ServiceDefinition, error) {
	if req.Status == "" {
		req.Status = model.ServiceStatusDraft
	}
	if req.TargetType == "" {
		req.TargetType = "DOCKER_NODE"
	}
	item := &model.ServiceDefinition{
		ID:             uuid.NewString(),
		Name:           strings.TrimSpace(req.Name),
		Code:           strings.TrimSpace(req.Code),
		Group:          req.Group,
		Tags:           req.Tags,
		Description:    req.Description,
		RegistryID:     req.RegistryID,
		Image:          strings.TrimSpace(req.Image),
		DefaultTag:     firstNonEmpty(req.DefaultTag, "latest"),
		Ports:          rawJSON(req.Ports),
		Envs:           rawJSON(req.Envs),
		Mounts:         rawJSON(req.Mounts),
		ResourceLimits: rawJSON(req.ResourceLimits),
		TargetType:     req.TargetType,
		TargetID:       req.TargetID,
		Status:         req.Status,
		CreatedBy:      req.OperatorID,
		UpdatedBy:      req.OperatorID,
	}
	err := s.db.WithContext(ctx).Create(item).Error
	if isUniqueConstraint(err) {
		return nil, ErrServiceCodeExists
	}
	return item, err
}

func (s *Service) List(ctx context.Context, keyword, status string, limit, offset int) ([]model.ServiceDefinition, int64, error) {
	var items []model.ServiceDefinition
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ServiceDefinition{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR image LIKE ?", like, like, like)
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
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(ctx context.Context, id string) (*model.ServiceDefinition, error) {
	var item model.ServiceDefinition
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*model.ServiceDefinition, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	if req.Group != "" {
		item.Group = req.Group
	}
	if req.Tags != "" {
		item.Tags = req.Tags
	}
	if req.Description != "" {
		item.Description = req.Description
	}
	if req.RegistryID != "" {
		item.RegistryID = req.RegistryID
	}
	if req.Image != "" {
		item.Image = strings.TrimSpace(req.Image)
	}
	if req.DefaultTag != "" {
		item.DefaultTag = req.DefaultTag
	}
	if len(req.Ports) > 0 {
		item.Ports = rawJSON(req.Ports)
	}
	if len(req.Envs) > 0 {
		item.Envs = rawJSON(req.Envs)
	}
	if len(req.Mounts) > 0 {
		item.Mounts = rawJSON(req.Mounts)
	}
	if len(req.ResourceLimits) > 0 {
		item.ResourceLimits = rawJSON(req.ResourceLimits)
	}
	if req.TargetType != "" {
		item.TargetType = req.TargetType
	}
	if req.TargetID != "" {
		item.TargetID = req.TargetID
	}
	if req.Status != "" {
		item.Status = req.Status
	}
	item.UpdatedBy = req.OperatorID
	return item, s.db.WithContext(ctx).Save(item).Error
}

func (s *Service) Delete(ctx context.Context, id string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.ServiceInstance{}).Where("service_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("service has instances and cannot be deleted")
	}
	return s.db.WithContext(ctx).Delete(&model.ServiceDefinition{}, "id = ?", id).Error
}

func (s *Service) Instances(ctx context.Context, serviceID string, limit, offset int) ([]model.ServiceInstance, int64, error) {
	var items []model.ServiceInstance
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ServiceInstance{}).Where("service_id = ?", serviceID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Releases(ctx context.Context, serviceID string, limit, offset int) ([]model.ServiceReleaseRecord, int64, error) {
	var items []model.ServiceReleaseRecord
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ServiceReleaseRecord{}).Where("service_id = ?", serviceID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Versions(ctx context.Context, serviceID string, limit, offset int) ([]model.ServiceVersion, int64, error) {
	var items []model.ServiceVersion
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ServiceVersion{}).Where("service_id = ?", serviceID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Release(ctx context.Context, serviceID string, req ReleaseRequest) (*ReleaseResult, error) {
	return s.startChange(ctx, serviceID, model.ServiceReleaseActionRelease, "", req.Version, req.ImageTag, req.ImageDigest, req.TargetID, req.OperatorID)
}

func (s *Service) Upgrade(ctx context.Context, serviceID string, req ReleaseRequest) (*ReleaseResult, error) {
	return s.startChange(ctx, serviceID, model.ServiceReleaseActionUpgrade, "", req.Version, req.ImageTag, req.ImageDigest, req.TargetID, req.OperatorID)
}

func (s *Service) Rollback(ctx context.Context, serviceID string, req RollbackRequest) (*ReleaseResult, error) {
	target, err := s.getVersion(ctx, serviceID, req.VersionID)
	if err != nil {
		return nil, err
	}
	return s.startChange(ctx, serviceID, model.ServiceReleaseActionRollback, target.ID, target.Version, target.ImageTag, target.ImageDigest, "", req.OperatorID)
}

func (s *Service) startChange(ctx context.Context, serviceID string, action model.ServiceReleaseAction, targetVersionID, version, imageTag, imageDigest, targetID, operatorID string) (*ReleaseResult, error) {
	s.releaseMu.Lock()
	defer s.releaseMu.Unlock()

	service, err := s.Get(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	if imageTag == "" {
		imageTag = service.DefaultTag
	}
	if version == "" {
		version = imageTag
	}
	if targetID == "" {
		targetID = service.TargetID
	}
	if targetID == "" {
		return nil, fmt.Errorf("target docker node is required")
	}
	if err := s.ensureNoRunningRelease(ctx, service.ID); err != nil {
		return nil, err
	}
	payloadBytes, _ := json.Marshal(map[string]string{
		"serviceId":   service.ID,
		"action":      string(action),
		"image":       service.Image,
		"imageTag":    imageTag,
		"version":     version,
		"targetId":    targetID,
		"imageDigest": imageDigest,
	})
	task, err := s.tasks.CreateRunning(ctx, tasksvc.CreateRequest{
		Type:       "service." + strings.ToLower(string(action)),
		Title:      fmt.Sprintf("%s service %s:%s", strings.ToLower(string(action)), service.Code, version),
		TargetType: "service",
		TargetID:   service.ID,
		Payload:    string(payloadBytes),
		CreatedBy:  operatorID,
		Steps: []tasksvc.CreateStepRequest{
			{Name: "validate release request", SortOrder: 1},
			{Name: "resolve image version", SortOrder: 2},
			{Name: "prepare docker execution", SortOrder: 3},
			{Name: "record service state", SortOrder: 4},
		},
	})
	if err != nil {
		return nil, err
	}
	stepsByName := map[string]string{}
	for _, step := range task.Steps {
		stepsByName[step.Name] = step.ID
	}
	runStep := func(name string, fn func() error) error {
		stepID := stepsByName[name]
		if stepID != "" {
			_ = s.tasks.UpdateStep(ctx, stepID, model.TaskStatusRunning, "", "")
		}
		if err := fn(); err != nil {
			if stepID != "" {
				_ = s.tasks.UpdateStep(ctx, stepID, model.TaskStatusFailed, "", err.Error())
				_, _ = s.tasks.AddLog(ctx, task.ID, stepID, model.TaskLogLevelError, err.Error())
			}
			return err
		}
		if stepID != "" {
			_ = s.tasks.UpdateStep(ctx, stepID, model.TaskStatusSuccess, "ok", "")
		}
		return nil
	}

	release := model.ServiceReleaseRecord{
		ID:            uuid.NewString(),
		ServiceID:     service.ID,
		TaskID:        task.ID,
		Action:        action,
		FromVersion:   service.CurrentVersion,
		TargetVersion: version,
		Status:        model.TaskStatusRunning,
		Message:       "release running",
		CreatedBy:     operatorID,
	}
	if err := s.db.WithContext(ctx).Create(&release).Error; err != nil {
		return nil, err
	}
	var versionRecord model.ServiceVersion
	if err := runStep("validate release request", func() error {
		return s.executor.Validate(ctx, DeployRequest{
			NodeID:        targetID,
			ServiceID:     service.ID,
			ServiceCode:   service.Code,
			ContainerName: service.Code + "-" + strings.ToLower(version),
			Image:         service.Image,
			ImageTag:      imageTag,
			PortsJSON:     service.Ports,
			EnvsJSON:      service.Envs,
			MountsJSON:    service.Mounts,
			ResourcesJSON: service.ResourceLimits,
		})
	}); err != nil {
		_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusFailed, "", err.Error())
		_ = s.updateReleaseFailure(ctx, release.ID, err.Error())
		return nil, err
	}
	if err := runStep("resolve image version", func() error {
		versionRecord = model.ServiceVersion{
			ID:          firstNonEmpty(targetVersionID, uuid.NewString()),
			ServiceID:   service.ID,
			Version:     version,
			Image:       service.Image,
			ImageTag:    imageTag,
			ImageDigest: imageDigest,
			Config:      service.Ports,
			CreatedBy:   operatorID,
		}
		return nil
	}); err != nil {
		_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusFailed, "", err.Error())
		_ = s.updateReleaseFailure(ctx, release.ID, err.Error())
		return nil, err
	}
	var deployResult *DeployResult
	if err := runStep("prepare docker execution", func() error {
		var err error
		deployResult, err = s.executor.Deploy(ctx, DeployRequest{
			NodeID:        targetID,
			ServiceID:     service.ID,
			ServiceCode:   service.Code,
			ContainerName: service.Code + "-" + strings.ToLower(version),
			Image:         service.Image,
			ImageTag:      imageTag,
			PortsJSON:     service.Ports,
			EnvsJSON:      service.Envs,
			MountsJSON:    service.Mounts,
			ResourcesJSON: service.ResourceLimits,
		})
		return err
	}); err != nil {
		_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusFailed, "", err.Error())
		_ = s.updateReleaseFailure(ctx, release.ID, err.Error())
		_ = s.recordFailedInstance(ctx, service.ID, targetVersionID, version, service.Image, imageTag, targetID, service.Code, err.Error())
		return nil, err
	}
	err = runStep("record service state", func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if targetVersionID == "" {
				if err := tx.Create(&versionRecord).Error; err != nil {
					return err
				}
			}
			now := time.Now().UTC()
			if err := tx.Model(&model.ServiceInstance{}).
				Where("service_id = ? AND status = ?", service.ID, model.ServiceInstanceStatusRunning).
				Updates(map[string]interface{}{
					"status":     model.ServiceInstanceStatusStopped,
					"stopped_at": &now,
				}).Error; err != nil {
				return err
			}
			instance := model.ServiceInstance{
				ID:           uuid.NewString(),
				ServiceID:    service.ID,
				VersionID:    versionRecord.ID,
				Version:      version,
				Image:        firstNonEmpty(deployResult.Image, service.Image),
				ImageTag:     imageTag,
				DockerNodeID: targetID,
				ContainerID:  deployResult.ContainerID,
				Name:         service.Code,
				Status:       model.ServiceInstanceStatusRunning,
				StartedAt:    &now,
			}
			if err := tx.Create(&instance).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ServiceReleaseRecord{}).Where("id = ?", release.ID).Updates(map[string]interface{}{
				"target_version_id": versionRecord.ID,
				"status":            model.TaskStatusSuccess,
				"message":           "service container deployed and state recorded",
			}).Error; err != nil {
				return err
			}
			release.TargetVersionID = versionRecord.ID
			release.Status = model.TaskStatusSuccess
			release.Message = "service container deployed and state recorded"
			return tx.Model(&model.ServiceDefinition{}).Where("id = ?", service.ID).Updates(map[string]interface{}{
				"status":          model.ServiceStatusActive,
				"current_version": version,
				"updated_by":      operatorID,
			}).Error
		})
	})
	if err != nil {
		_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusFailed, "", err.Error())
		_ = s.updateReleaseFailure(ctx, release.ID, err.Error())
		return nil, err
	}
	_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusSuccess, release.Message, "")
	return &ReleaseResult{TaskID: task.ID, ReleaseID: release.ID}, nil
}

func (s *Service) ensureNoRunningRelease(ctx context.Context, serviceID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.ServiceReleaseRecord{}).
		Where("service_id = ? AND status IN ?", serviceID, []model.TaskStatus{model.TaskStatusPending, model.TaskStatusRunning}).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrReleaseInProgress
	}
	return nil
}

func (s *Service) updateReleaseFailure(ctx context.Context, releaseID, message string) error {
	return s.db.WithContext(ctx).Model(&model.ServiceReleaseRecord{}).Where("id = ?", releaseID).Updates(map[string]interface{}{
		"status":  model.TaskStatusFailed,
		"message": message,
	}).Error
}

func (s *Service) recordFailedInstance(ctx context.Context, serviceID, versionID, version, image, imageTag, targetID, name, message string) error {
	instance := model.ServiceInstance{
		ID:           uuid.NewString(),
		ServiceID:    serviceID,
		VersionID:    versionID,
		Version:      version,
		Image:        image,
		ImageTag:     imageTag,
		DockerNodeID: targetID,
		Name:         name,
		Status:       model.ServiceInstanceStatusFailed,
		LastError:    message,
	}
	return s.db.WithContext(ctx).Create(&instance).Error
}

func (s *Service) getVersion(ctx context.Context, serviceID, versionID string) (*model.ServiceVersion, error) {
	var item model.ServiceVersion
	if err := s.db.WithContext(ctx).First(&item, "id = ? AND service_id = ?", versionID, serviceID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func rawJSON(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}
	return string(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func imageRef(image, tag string) string {
	if tag == "" || strings.Contains(lastSegment(image), ":") {
		return image
	}
	return image + ":" + tag
}

func lastSegment(value string) string {
	parts := strings.Split(value, "/")
	return parts[len(parts)-1]
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
