package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	dockersvc "github.com/Humphrey-He/AegisOps/internal/docker"
	envsvc "github.com/Humphrey-He/AegisOps/internal/environment"
	healthsvc "github.com/Humphrey-He/AegisOps/internal/healthcheck"
	"github.com/Humphrey-He/AegisOps/internal/model"
	secretsvc "github.com/Humphrey-He/AegisOps/internal/secret"
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
	health    *healthsvc.Service
	secrets   *secretsvc.Service
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
	RegistryAuth  string
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
		RegistryAuth:  req.RegistryAuth,
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
		RegistryAuth:  req.RegistryAuth,
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
	Environment    string              `json:"environment"`
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
	Environment    string              `json:"environment"`
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

type changePayload struct {
	ServiceID       string `json:"serviceId"`
	Action          string `json:"action"`
	Image           string `json:"image"`
	ImageTag        string `json:"imageTag"`
	Version         string `json:"version"`
	TargetID        string `json:"targetId"`
	ImageDigest     string `json:"imageDigest"`
	TargetVersionID string `json:"targetVersionId,omitempty"`
	ReleaseID       string `json:"releaseId,omitempty"`
	OperatorID       string `json:"operatorId,omitempty"`
}

func NewService(db *gorm.DB, tasks *tasksvc.Service, executor ReleaseExecutor) *Service {
	if executor == nil {
		executor = NoopReleaseExecutor{}
	}
	return &Service{db: db, tasks: tasks, executor: executor}
}

func (s *Service) SetHealthCheckService(health *healthsvc.Service) {
	s.health = health
}

func (s *Service) SetSecretService(secrets *secretsvc.Service) {
	s.secrets = secrets
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.ServiceDefinition, error) {
	if req.Status == "" {
		req.Status = model.ServiceStatusDraft
	}
	if req.TargetType == "" {
		req.TargetType = "DOCKER_NODE"
	}
	environment, err := envsvc.EnsureActive(ctx, s.db, req.Environment)
	if err != nil {
		return nil, err
	}
	item := &model.ServiceDefinition{
		ID:             uuid.NewString(),
		Name:           strings.TrimSpace(req.Name),
		Code:           strings.TrimSpace(req.Code),
		Environment:    environment,
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
	err = s.db.WithContext(ctx).Create(item).Error
	if isUniqueConstraint(err) {
		return nil, ErrServiceCodeExists
	}
	return item, err
}

func (s *Service) List(ctx context.Context, keyword, status, environment string, limit, offset int) ([]model.ServiceDefinition, int64, error) {
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
	if environment != "" {
		query = query.Where("environment = ?", environment)
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
	if strings.TrimSpace(req.Environment) != "" {
		environment, err := envsvc.EnsureActive(ctx, s.db, req.Environment)
		if err != nil {
			return nil, err
		}
		item.Environment = environment
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
	return s.enqueueChange(ctx, serviceID, model.ServiceReleaseActionRelease, "", req.Version, req.ImageTag, req.ImageDigest, req.TargetID, req.OperatorID)
}

func (s *Service) Upgrade(ctx context.Context, serviceID string, req ReleaseRequest) (*ReleaseResult, error) {
	return s.enqueueChange(ctx, serviceID, model.ServiceReleaseActionUpgrade, "", req.Version, req.ImageTag, req.ImageDigest, req.TargetID, req.OperatorID)
}

func (s *Service) Rollback(ctx context.Context, serviceID string, req RollbackRequest) (*ReleaseResult, error) {
	target, err := s.getVersion(ctx, serviceID, req.VersionID)
	if err != nil {
		return nil, err
	}
	return s.enqueueChange(ctx, serviceID, model.ServiceReleaseActionRollback, target.ID, target.Version, target.ImageTag, target.ImageDigest, "", req.OperatorID)
}

func (s *Service) enqueueChange(ctx context.Context, serviceID string, action model.ServiceReleaseAction, targetVersionID, version, imageTag, imageDigest, targetID, operatorID string) (*ReleaseResult, error) {
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
	if err := s.ensureTargetEnvironment(ctx, service, targetID); err != nil {
		return nil, err
	}
	releaseID := uuid.NewString()
	payloadBytes, _ := json.Marshal(changePayload{
		ServiceID:       service.ID,
		Action:          string(action),
		Image:           service.Image,
		ImageTag:        imageTag,
		Version:         version,
		TargetID:        targetID,
		ImageDigest:     imageDigest,
		TargetVersionID: targetVersionID,
		ReleaseID:       releaseID,
		OperatorID:      operatorID,
	})
	task := model.Task{
		ID:         uuid.NewString(),
		Type:       "service." + strings.ToLower(string(action)),
		Title:      fmt.Sprintf("%s service %s:%s", strings.ToLower(string(action)), service.Code, version),
		Status:     model.TaskStatusPending,
		TargetType: "service",
		TargetID:   service.ID,
		Payload:    string(payloadBytes),
		CreatedBy:  operatorID,
	}
	stepReqs := []tasksvc.CreateStepRequest{
		{Name: "validate release request", SortOrder: 1},
		{Name: "resolve image version", SortOrder: 2},
		{Name: "prepare docker execution", SortOrder: 3},
		{Name: "record service state", SortOrder: 4},
		{Name: "run post-release health check", SortOrder: 5},
	}
	release := model.ServiceReleaseRecord{
		ID:            releaseID,
		ServiceID:     service.ID,
		TaskID:        task.ID,
		Environment:   service.Environment,
		Action:        action,
		FromVersion:   service.CurrentVersion,
		TargetVersion: version,
		Status:        model.TaskStatusPending,
		Message:       "release queued",
		CreatedBy:     operatorID,
	}
	dispatch := model.TaskDispatch{
		ID:             uuid.NewString(),
		TaskID:         task.ID,
		Source:         model.TaskDispatchSourceManual,
		Status:         model.TaskDispatchStatusPending,
		TimeoutSeconds: 1800,
		ConcurrencyKey: serviceReleaseConcurrencyKey(service.ID),
		QueuedAt:       time.Now().UTC(),
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureNoRunningReleaseTx(tx, service.ID); err != nil {
			return err
		}
		if err := ensureNoRunningDispatchTx(tx, dispatch.ConcurrencyKey); err != nil {
			return err
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		for _, stepReq := range stepReqs {
			step := model.TaskStep{
				ID:        uuid.NewString(),
				TaskID:    task.ID,
				Name:      stepReq.Name,
				Status:    model.TaskStatusPending,
				SortOrder: stepReq.SortOrder,
			}
			if err := tx.Create(&step).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&release).Error; err != nil {
			return err
		}
		return tx.Create(&dispatch).Error
	})
	if err != nil {
		return nil, err
	}
	return &ReleaseResult{TaskID: task.ID, ReleaseID: release.ID}, nil
}

func (s *Service) ExecuteServiceChange(ctx context.Context, task model.Task, _ model.TaskDispatch) (string, error) {
	var payload changePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("parse service release payload: %w", err)
	}
	if payload.ServiceID == "" {
		payload.ServiceID = task.TargetID
	}
	action := model.ServiceReleaseAction(strings.ToUpper(strings.TrimSpace(payload.Action)))
	if action == "" {
		action = model.ServiceReleaseActionRelease
	}
	releaseID := payload.ReleaseID
	if releaseID == "" {
		var release model.ServiceReleaseRecord
		if err := s.db.WithContext(ctx).First(&release, "task_id = ?", task.ID).Error; err != nil {
			return "", err
		}
		releaseID = release.ID
	}
	if err := s.executeChange(ctx, task.ID, releaseID, payload.ServiceID, action, payload.TargetVersionID, payload.Version, payload.ImageTag, payload.ImageDigest, payload.TargetID, payload.OperatorID); err != nil {
		return "", err
	}
	return "service release completed", nil
}

func (s *Service) executeChange(ctx context.Context, taskID, releaseID, serviceID string, action model.ServiceReleaseAction, targetVersionID, version, imageTag, imageDigest, targetID, operatorID string) error {
	service, err := s.Get(ctx, serviceID)
	if err != nil {
		_ = s.updateReleaseFailure(ctx, releaseID, err.Error())
		return err
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
	var task model.Task
	if err := s.db.WithContext(ctx).Preload("Steps").First(&task, "id = ?", taskID).Error; err != nil {
		_ = s.updateReleaseFailure(ctx, releaseID, err.Error())
		return err
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
	if err := s.db.WithContext(ctx).Model(&model.ServiceReleaseRecord{}).Where("id = ?", releaseID).Updates(map[string]interface{}{
		"status":  model.TaskStatusRunning,
		"message": "release running",
	}).Error; err != nil {
		return err
	}
	var versionRecord model.ServiceVersion
	if err := runStep("validate release request", func() error {
		registryAuth, err := s.registryAuthForService(ctx, service, task.ID, operatorID)
		if err != nil {
			return err
		}
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
			RegistryAuth:  registryAuth,
		})
	}); err != nil {
		_ = s.updateReleaseFailure(ctx, releaseID, err.Error())
		return err
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
		_ = s.updateReleaseFailure(ctx, releaseID, err.Error())
		return err
	}
	var deployResult *DeployResult
	if err := runStep("prepare docker execution", func() error {
		registryAuth, err := s.registryAuthForService(ctx, service, task.ID, operatorID)
		if err != nil {
			return err
		}
		result, err := s.executor.Deploy(ctx, DeployRequest{
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
			RegistryAuth:  registryAuth,
		})
		deployResult = result
		return err
	}); err != nil {
		_ = s.updateReleaseFailure(ctx, releaseID, err.Error())
		_ = s.recordFailedInstance(ctx, service, targetVersionID, version, imageTag, targetID, err.Error())
		return err
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
				Environment:  service.Environment,
				ContainerID:  deployResult.ContainerID,
				Name:         service.Code,
				Status:       model.ServiceInstanceStatusRunning,
				StartedAt:    &now,
			}
			if err := tx.Create(&instance).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ServiceReleaseRecord{}).Where("id = ?", releaseID).Updates(map[string]interface{}{
				"target_version_id": versionRecord.ID,
				"status":            model.TaskStatusSuccess,
				"message":           "service container deployed and state recorded",
			}).Error; err != nil {
				return err
			}
			return tx.Model(&model.ServiceDefinition{}).Where("id = ?", service.ID).Updates(map[string]interface{}{
				"status":          model.ServiceStatusActive,
				"current_version": version,
				"updated_by":      operatorID,
			}).Error
		})
	})
	if err != nil {
		_ = s.updateReleaseFailure(ctx, releaseID, err.Error())
		return err
	}
	if err := runStep("run post-release health check", func() error {
		if s.health == nil {
			_, _ = s.tasks.AddLog(ctx, task.ID, stepsByName["run post-release health check"], model.TaskLogLevelInfo, "health check service is not configured; skipped")
			return nil
		}
		release := model.ServiceReleaseRecord{ID: releaseID, ServiceID: service.ID, TaskID: task.ID, Environment: service.Environment, Action: action, TargetVersionID: versionRecord.ID, TargetVersion: version}
		check, err := s.health.RunServiceCheck(ctx, *service, release, task.ID)
		if check != nil {
			_, _ = s.tasks.AddLog(ctx, task.ID, stepsByName["run post-release health check"], model.TaskLogLevelInfo, check.Output)
		}
		return err
	}); err != nil {
		_ = s.updateReleaseFailure(ctx, releaseID, err.Error())
		_ = s.markLatestInstanceFailed(ctx, service.ID, err.Error())
		return err
	}
	return nil
}

func (s *Service) ensureNoRunningRelease(ctx context.Context, serviceID string) error {
	return ensureNoRunningReleaseTx(s.db.WithContext(ctx), serviceID)
}

func ensureNoRunningReleaseTx(tx *gorm.DB, serviceID string) error {
	var count int64
	if err := tx.Model(&model.ServiceReleaseRecord{}).
		Where("service_id = ? AND status IN ?", serviceID, []model.TaskStatus{model.TaskStatusPending, model.TaskStatusRunning}).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrReleaseInProgress
	}
	return nil
}

func ensureNoRunningDispatchTx(tx *gorm.DB, concurrencyKey string) error {
	concurrencyKey = strings.TrimSpace(concurrencyKey)
	if concurrencyKey == "" {
		return nil
	}
	var count int64
	if err := tx.Model(&model.TaskDispatch{}).
		Where("concurrency_key = ? AND status IN ?", concurrencyKey, []model.TaskDispatchStatus{
			model.TaskDispatchStatusPending,
			model.TaskDispatchStatusDispatched,
			model.TaskDispatchStatusRunning,
		}).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrReleaseInProgress
	}
	return nil
}

func serviceReleaseConcurrencyKey(serviceID string) string {
	return "service:" + strings.TrimSpace(serviceID) + ":release"
}

func (s *Service) registryAuthForService(ctx context.Context, service *model.ServiceDefinition, taskID, operatorID string) (string, error) {
	if service == nil || strings.TrimSpace(service.RegistryID) == "" {
		return "", nil
	}
	var registry model.Registry
	if err := s.db.WithContext(ctx).First(&registry, "id = ?", service.RegistryID).Error; err != nil {
		return "", err
	}
	if registry.AuthType == "" || registry.AuthType == model.RegistryAuthTypeNone {
		return "", nil
	}
	if s.secrets == nil {
		return "", fmt.Errorf("secret service is not configured for registry auth")
	}
	if strings.TrimSpace(registry.SecretID) == "" {
		return "", fmt.Errorf("registry auth secret is required")
	}
	value, err := s.secrets.DecryptValueForUse(ctx, registry.SecretID, secretsvc.UseContext{
		ResourceType: "service",
		ResourceID:   service.ID,
		Action:       "service.release.registry_auth",
		OperatorID:   operatorID,
		TaskID:       taskID,
	})
	if err != nil {
		return "", err
	}
	auth := map[string]string{"serveraddress": registry.URL}
	switch registry.AuthType {
	case model.RegistryAuthTypeBasic:
		username, password, ok := strings.Cut(value, ":")
		if !ok || strings.TrimSpace(username) == "" {
			return "", fmt.Errorf("registry basic secret must be username:password")
		}
		auth["username"] = username
		auth["password"] = password
	case model.RegistryAuthTypeToken:
		auth["identitytoken"] = value
	default:
		return "", fmt.Errorf("unsupported registry auth type %s", registry.AuthType)
	}
	data, err := json.Marshal(auth)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

func (s *Service) updateReleaseFailure(ctx context.Context, releaseID, message string) error {
	return s.db.WithContext(ctx).Model(&model.ServiceReleaseRecord{}).Where("id = ?", releaseID).Updates(map[string]interface{}{
		"status":  model.TaskStatusFailed,
		"message": message,
	}).Error
}

func (s *Service) ensureTargetEnvironment(ctx context.Context, service *model.ServiceDefinition, targetID string) error {
	serviceEnvironment, err := envsvc.EnsureActive(ctx, s.db, service.Environment)
	if err != nil {
		return err
	}
	if serviceEnvironment == "" || strings.TrimSpace(targetID) == "" {
		return nil
	}
	var node model.DockerNode
	if err := s.db.WithContext(ctx).First(&node, "id = ?", targetID).Error; err != nil {
		return err
	}
	if strings.TrimSpace(node.Environment) == "" {
		return fmt.Errorf("target docker node environment is required when service environment is %s", serviceEnvironment)
	}
	nodeEnvironment, err := envsvc.EnsureActive(ctx, s.db, node.Environment)
	if err != nil {
		return err
	}
	if nodeEnvironment != serviceEnvironment {
		return fmt.Errorf("target docker node environment %s does not match service environment %s", nodeEnvironment, serviceEnvironment)
	}
	return nil
}

func (s *Service) recordFailedInstance(ctx context.Context, service *model.ServiceDefinition, versionID, version, imageTag, targetID, message string) error {
	instance := model.ServiceInstance{
		ID:           uuid.NewString(),
		ServiceID:    service.ID,
		VersionID:    versionID,
		Version:      version,
		Image:        service.Image,
		ImageTag:     imageTag,
		DockerNodeID: targetID,
		Environment:  service.Environment,
		Name:         service.Code,
		Status:       model.ServiceInstanceStatusFailed,
		LastError:    message,
	}
	return s.db.WithContext(ctx).Create(&instance).Error
}

func (s *Service) markLatestInstanceFailed(ctx context.Context, serviceID, message string) error {
	return s.db.WithContext(ctx).Model(&model.ServiceInstance{}).
		Where("service_id = ? AND status = ?", serviceID, model.ServiceInstanceStatusRunning).
		Updates(map[string]interface{}{"status": model.ServiceInstanceStatusFailed, "last_error": message}).Error
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
