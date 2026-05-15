package model

import "time"

type ScheduledJobStatus string

const (
	ScheduledJobStatusPending  ScheduledJobStatus = "PENDING"
	ScheduledJobStatusDisabled ScheduledJobStatus = "DISABLED"
)

type TaskDispatchSource string

const (
	TaskDispatchSourceManual    TaskDispatchSource = "MANUAL"
	TaskDispatchSourceSystem    TaskDispatchSource = "SYSTEM"
	TaskDispatchSourceScheduled TaskDispatchSource = "SCHEDULED"
)

type TaskDispatchStatus string

const (
	TaskDispatchStatusPending    TaskDispatchStatus = "PENDING"
	TaskDispatchStatusDispatched TaskDispatchStatus = "DISPATCHED"
	TaskDispatchStatusRunning    TaskDispatchStatus = "RUNNING"
	TaskDispatchStatusSuccess    TaskDispatchStatus = "SUCCESS"
	TaskDispatchStatusFailed     TaskDispatchStatus = "FAILED"
	TaskDispatchStatusCanceled   TaskDispatchStatus = "CANCELED"
	TaskDispatchStatusTimeout    TaskDispatchStatus = "TIMEOUT"
)

type ScheduledJob struct {
	ID              string     `gorm:"primaryKey;size:36" json:"id"`
	Name            string     `gorm:"size:128;not null;index" json:"name"`
	Type            string     `gorm:"size:64;not null;index" json:"type"`
	Enabled         bool       `gorm:"not null;default:true;index" json:"enabled"`
	CronExpr        string     `gorm:"size:128;not null" json:"cronExpr"`
	TargetType      string     `gorm:"size:64;index" json:"targetType"`
	TargetID        string     `gorm:"size:128;index" json:"targetId"`
	PayloadJSON     string     `gorm:"type:text" json:"payloadJson"`
	RetryPolicyJSON string     `gorm:"type:text" json:"retryPolicyJson"`
	TimeoutSeconds  int        `gorm:"not null;default:300" json:"timeoutSeconds"`
	ConcurrencyKey  string     `gorm:"size:255;index" json:"concurrencyKey"`
	LastRunAt       *time.Time `json:"lastRunAt"`
	NextRunAt       *time.Time `json:"nextRunAt"`
	CreatedBy       string     `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy       string     `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeletedAt       *time.Time `gorm:"index" json:"-"`
}

type TaskDispatch struct {
	ID             string             `gorm:"primaryKey;size:36" json:"id"`
	TaskID         string             `gorm:"size:36;not null;index" json:"taskId"`
	Source         TaskDispatchSource `gorm:"size:32;not null;index" json:"source"`
	JobID          string             `gorm:"size:36;index" json:"jobId"`
	Status         TaskDispatchStatus `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	RetryCount     int                `gorm:"not null;default:0" json:"retryCount"`
	MaxRetry       int                `gorm:"not null;default:0" json:"maxRetry"`
	TimeoutSeconds int                `gorm:"not null;default:300" json:"timeoutSeconds"`
	ConcurrencyKey string             `gorm:"size:255;index" json:"concurrencyKey"`
	LeaseOwner     string             `gorm:"size:128;index" json:"leaseOwner"`
	LeaseExpiresAt *time.Time         `json:"leaseExpiresAt"`
	QueuedAt       time.Time          `gorm:"index" json:"queuedAt"`
	StartedAt      *time.Time         `json:"startedAt"`
	FinishedAt     *time.Time         `json:"finishedAt"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}
