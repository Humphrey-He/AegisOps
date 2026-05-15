package model

import "time"

type ServiceStatus string

const (
	ServiceStatusDraft    ServiceStatus = "DRAFT"
	ServiceStatusActive   ServiceStatus = "ACTIVE"
	ServiceStatusArchived ServiceStatus = "ARCHIVED"
)

type ServiceInstanceStatus string

const (
	ServiceInstanceStatusPending  ServiceInstanceStatus = "PENDING"
	ServiceInstanceStatusRunning  ServiceInstanceStatus = "RUNNING"
	ServiceInstanceStatusStopped  ServiceInstanceStatus = "STOPPED"
	ServiceInstanceStatusFailed   ServiceInstanceStatus = "FAILED"
	ServiceInstanceStatusRollback ServiceInstanceStatus = "ROLLBACK"
)

type ServiceReleaseAction string

const (
	ServiceReleaseActionRelease  ServiceReleaseAction = "RELEASE"
	ServiceReleaseActionUpgrade  ServiceReleaseAction = "UPGRADE"
	ServiceReleaseActionRollback ServiceReleaseAction = "ROLLBACK"
)

type ServiceDefinition struct {
	ID             string        `gorm:"primaryKey;size:36" json:"id"`
	Name           string        `gorm:"size:128;not null;index" json:"name"`
	Code           string        `gorm:"size:128;not null;uniqueIndex" json:"code"`
	Group          string        `gorm:"size:128;index" json:"group"`
	Tags           string        `gorm:"type:text" json:"tags"`
	Description    string        `gorm:"size:512" json:"description"`
	RegistryID     string        `gorm:"size:36;index" json:"registryId"`
	Image          string        `gorm:"size:512;not null" json:"image"`
	DefaultTag     string        `gorm:"size:128" json:"defaultTag"`
	Ports          string        `gorm:"type:text" json:"ports"`
	Envs           string        `gorm:"type:text" json:"envs"`
	Mounts         string        `gorm:"type:text" json:"mounts"`
	ResourceLimits string        `gorm:"type:text" json:"resourceLimits"`
	TargetType     string        `gorm:"size:32;not null;default:DOCKER_NODE" json:"targetType"`
	TargetID       string        `gorm:"size:36;index" json:"targetId"`
	Status         ServiceStatus `gorm:"size:32;not null;default:DRAFT;index" json:"status"`
	CurrentVersion string        `gorm:"size:64;index" json:"currentVersion"`
	CreatedBy      string        `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy      string        `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	DeletedAt      *time.Time    `gorm:"index" json:"-"`
}

type ServiceVersion struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	ServiceID   string    `gorm:"size:36;not null;index" json:"serviceId"`
	Version     string    `gorm:"size:64;not null;index" json:"version"`
	Image       string    `gorm:"size:512;not null" json:"image"`
	ImageTag    string    `gorm:"size:128;not null" json:"imageTag"`
	ImageDigest string    `gorm:"size:255" json:"imageDigest"`
	Config      string    `gorm:"type:text" json:"config"`
	CreatedBy   string    `gorm:"size:36;index" json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ServiceInstance struct {
	ID           string                `gorm:"primaryKey;size:36" json:"id"`
	ServiceID    string                `gorm:"size:36;not null;index" json:"serviceId"`
	VersionID    string                `gorm:"size:36;index" json:"versionId"`
	Version      string                `gorm:"size:64;index" json:"version"`
	Image        string                `gorm:"size:512;not null" json:"image"`
	ImageTag     string                `gorm:"size:128;not null" json:"imageTag"`
	DockerNodeID string                `gorm:"size:36;index" json:"dockerNodeId"`
	ContainerID  string                `gorm:"size:128;index" json:"containerId"`
	Name         string                `gorm:"size:128;not null;index" json:"name"`
	Status       ServiceInstanceStatus `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	LastError    string                `gorm:"type:text" json:"lastError"`
	StartedAt    *time.Time            `json:"startedAt"`
	StoppedAt    *time.Time            `json:"stoppedAt"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
	DeletedAt    *time.Time            `gorm:"index" json:"-"`
}

type ServiceReleaseRecord struct {
	ID              string               `gorm:"primaryKey;size:36" json:"id"`
	ServiceID       string               `gorm:"size:36;not null;index;index:idx_service_release_records_service_status,priority:1" json:"serviceId"`
	TaskID          string               `gorm:"size:36;not null;index" json:"taskId"`
	Action          ServiceReleaseAction `gorm:"size:32;not null;index" json:"action"`
	FromVersionID   string               `gorm:"size:36;index" json:"fromVersionId"`
	FromVersion     string               `gorm:"size:64;index" json:"fromVersion"`
	TargetVersionID string               `gorm:"size:36;index" json:"targetVersionId"`
	TargetVersion   string               `gorm:"size:64;index" json:"targetVersion"`
	Status          TaskStatus           `gorm:"size:32;not null;default:PENDING;index;index:idx_service_release_records_service_status,priority:2" json:"status"`
	Message         string               `gorm:"type:text" json:"message"`
	CreatedBy       string               `gorm:"size:36;index" json:"createdBy"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
}
