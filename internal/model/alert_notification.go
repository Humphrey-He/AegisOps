package model

import "time"

type NotificationChannelType string

const (
	NotificationChannelTypeTelegram NotificationChannelType = "telegram"
	NotificationChannelTypeWecom    NotificationChannelType = "wecom"
	NotificationChannelTypeEmail    NotificationChannelType = "email"
)

type NotificationRecordStatus string

const (
	NotificationRecordStatusPending NotificationRecordStatus = "PENDING"
	NotificationRecordStatusSuccess NotificationRecordStatus = "SUCCESS"
	NotificationRecordStatusFailed  NotificationRecordStatus = "FAILED"
)

type AlertEventSeverity string

const (
	AlertEventSeverityInfo     AlertEventSeverity = "INFO"
	AlertEventSeverityWarning  AlertEventSeverity = "WARNING"
	AlertEventSeverityCritical AlertEventSeverity = "CRITICAL"
)

type AlertEventStatus string

const (
	AlertEventStatusOpen     AlertEventStatus = "OPEN"
	AlertEventStatusAcked    AlertEventStatus = "ACKED"
	AlertEventStatusResolved AlertEventStatus = "RESOLVED"
)

type HealthCheckStrategyType string

const (
	HealthCheckStrategyHTTP    HealthCheckStrategyType = "HTTP"
	HealthCheckStrategyTCP     HealthCheckStrategyType = "TCP"
	HealthCheckStrategyCommand HealthCheckStrategyType = "COMMAND"
)

type HealthCheckStatus string

const (
	HealthCheckStatusSuccess HealthCheckStatus = "SUCCESS"
	HealthCheckStatusFailed  HealthCheckStatus = "FAILED"
)

type HostAvailabilityStatus string

const (
	HostAvailabilityStatusOnline      HostAvailabilityStatus = "ONLINE"
	HostAvailabilityStatusUnreachable HostAvailabilityStatus = "UNREACHABLE"
)

type NotificationChannel struct {
	ID              string                   `gorm:"primaryKey;size:36" json:"id"`
	Name            string                   `gorm:"size:128;not null;index" json:"name"`
	Type            NotificationChannelType  `gorm:"size:32;not null;index" json:"type"`
	Enabled         bool                     `gorm:"not null;default:true;index" json:"enabled"`
	Language        string                   `gorm:"size:16;not null;default:zh-CN" json:"language"`
	ConfigEncrypted string                   `gorm:"type:text" json:"-"`
	Config          string                   `gorm:"-" json:"config,omitempty"`
	DefaultTarget   string                   `gorm:"size:512" json:"defaultTarget"`
	LastStatus      NotificationRecordStatus `gorm:"size:32;index" json:"lastStatus"`
	LastError       string                   `gorm:"type:text" json:"lastError"`
	LastSentAt      *time.Time               `json:"lastSentAt"`
	CreatedBy       string                   `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy       string                   `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt       time.Time                `json:"createdAt"`
	UpdatedAt       time.Time                `json:"updatedAt"`
	DeletedAt       *time.Time               `gorm:"index" json:"-"`
}

type AlertRule struct {
	ID                  string     `gorm:"primaryKey;size:36" json:"id"`
	Name                string     `gorm:"size:128;not null;index" json:"name"`
	EventType           string     `gorm:"size:128;not null;index" json:"eventType"`
	ResourceType        string     `gorm:"size:64;index" json:"resourceType"`
	ResourceScope       string     `gorm:"type:text" json:"resourceScope"`
	ChannelIDs          string     `gorm:"type:text" json:"channelIds"`
	Language            string     `gorm:"size:16" json:"language"`
	Enabled             bool       `gorm:"not null;default:true;index" json:"enabled"`
	DedupeWindowSeconds int        `gorm:"not null;default:300" json:"dedupeWindowSeconds"`
	RequireAck          bool       `gorm:"not null;default:false" json:"requireAck"`
	CreatedBy           string     `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy           string     `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	DeletedAt           *time.Time `gorm:"index" json:"-"`
}

type AlertEvent struct {
	ID               string             `gorm:"primaryKey;size:36" json:"id"`
	EventType        string             `gorm:"size:128;not null;index" json:"eventType"`
	ResourceType     string             `gorm:"size:64;index" json:"resourceType"`
	ResourceID       string             `gorm:"size:128;index" json:"resourceId"`
	TaskID           string             `gorm:"size:36;index" json:"taskId"`
	ReleaseID        string             `gorm:"size:36;index" json:"releaseId"`
	Severity         AlertEventSeverity `gorm:"size:32;not null;default:WARNING;index" json:"severity"`
	Status           AlertEventStatus   `gorm:"size:32;not null;default:OPEN;index" json:"status"`
	Summary          string             `gorm:"size:512" json:"summary"`
	Detail           string             `gorm:"type:text" json:"detail"`
	DedupeKey        string             `gorm:"size:255;index" json:"dedupeKey"`
	Suggestion       string             `gorm:"type:text" json:"suggestion"`
	FirstTriggeredAt time.Time          `json:"firstTriggeredAt"`
	LastTriggeredAt  time.Time          `json:"lastTriggeredAt"`
	AckedAt          *time.Time         `json:"ackedAt"`
	AckedBy          string             `gorm:"size:36;index" json:"ackedBy"`
	ResolvedAt       *time.Time         `json:"resolvedAt"`
	ResolvedBy       string             `gorm:"size:36;index" json:"resolvedBy"`
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`
}

type NotificationRecord struct {
	ID                string                   `gorm:"primaryKey;size:36" json:"id"`
	EventID           string                   `gorm:"size:36;index" json:"eventId"`
	ChannelID         string                   `gorm:"size:36;index" json:"channelId"`
	ChannelName       string                   `gorm:"size:128" json:"channelName"`
	ChannelType       NotificationChannelType  `gorm:"size:32;index" json:"channelType"`
	Status            NotificationRecordStatus `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	ProviderMessageID string                   `gorm:"size:255" json:"providerMessageId"`
	ResponseExcerpt   string                   `gorm:"type:text" json:"responseExcerpt"`
	ErrorMessage      string                   `gorm:"type:text" json:"errorMessage"`
	CreatedAt         time.Time                `json:"createdAt"`
	FinishedAt        *time.Time               `json:"finishedAt"`
}

type ServiceHealthCheck struct {
	ID           string                  `gorm:"primaryKey;size:36" json:"id"`
	ServiceID    string                  `gorm:"size:36;not null;index" json:"serviceId"`
	ReleaseID    string                  `gorm:"size:36;index" json:"releaseId"`
	TaskID       string                  `gorm:"size:36;index" json:"taskId"`
	StrategyType HealthCheckStrategyType `gorm:"size:32;not null;index" json:"strategyType"`
	Target       string                  `gorm:"size:512" json:"target"`
	Status       HealthCheckStatus       `gorm:"size:32;not null;index" json:"status"`
	HTTPStatus   int                     `json:"httpStatus"`
	LatencyMs    int64                   `json:"latencyMs"`
	Output       string                  `gorm:"type:text" json:"output"`
	ErrorMessage string                  `gorm:"type:text" json:"errorMessage"`
	StartedAt    time.Time               `json:"startedAt"`
	FinishedAt   time.Time               `json:"finishedAt"`
	CreatedAt    time.Time               `json:"createdAt"`
}

type HostAvailabilityCheck struct {
	ID            string                 `gorm:"primaryKey;size:36" json:"id"`
	HostID        string                 `gorm:"size:36;not null;index" json:"hostId"`
	TaskID        string                 `gorm:"size:36;index" json:"taskId"`
	Status        HostAvailabilityStatus `gorm:"size:32;not null;index" json:"status"`
	FailureReason string                 `gorm:"type:text" json:"failureReason"`
	StartedAt     time.Time              `json:"startedAt"`
	FinishedAt    time.Time              `json:"finishedAt"`
	CreatedAt     time.Time              `json:"createdAt"`
}
