package model

import "time"

type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "PENDING"
	TaskStatusRunning  TaskStatus = "RUNNING"
	TaskStatusSuccess  TaskStatus = "SUCCESS"
	TaskStatusFailed   TaskStatus = "FAILED"
	TaskStatusCanceled TaskStatus = "CANCELED"
)

type Task struct {
	ID         string     `gorm:"primaryKey;size:36" json:"id"`
	Type       string     `gorm:"size:64;not null;index" json:"type"`
	Title      string     `gorm:"size:255;not null" json:"title"`
	Status     TaskStatus `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	TargetType string     `gorm:"size:64;index" json:"targetType"`
	TargetID   string     `gorm:"size:128;index" json:"targetId"`
	Payload    string     `gorm:"type:text" json:"payload"`
	Result     string     `gorm:"type:text" json:"result"`
	Error      string     `gorm:"type:text" json:"error"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	CreatedBy  string     `gorm:"size:36;index" json:"createdBy"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	DeletedAt  *time.Time `gorm:"index" json:"-"`
	Steps      []TaskStep `gorm:"foreignKey:TaskID" json:"steps,omitempty"`
	Logs       []TaskLog  `gorm:"foreignKey:TaskID" json:"logs,omitempty"`
}

type TaskStep struct {
	ID         string     `gorm:"primaryKey;size:36" json:"id"`
	TaskID     string     `gorm:"size:36;not null;index" json:"taskId"`
	Name       string     `gorm:"size:128;not null" json:"name"`
	Status     TaskStatus `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	SortOrder  int        `gorm:"not null;default:0" json:"sortOrder"`
	Result     string     `gorm:"type:text" json:"result"`
	Error      string     `gorm:"type:text" json:"error"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type TaskLogLevel string

const (
	TaskLogLevelInfo  TaskLogLevel = "INFO"
	TaskLogLevelWarn  TaskLogLevel = "WARN"
	TaskLogLevelError TaskLogLevel = "ERROR"
)

type TaskLog struct {
	ID        string       `gorm:"primaryKey;size:36" json:"id"`
	TaskID    string       `gorm:"size:36;not null;index" json:"taskId"`
	StepID    string       `gorm:"size:36;index" json:"stepId"`
	Level     TaskLogLevel `gorm:"size:16;not null;default:INFO" json:"level"`
	Message   string       `gorm:"type:text;not null" json:"message"`
	CreatedAt time.Time    `gorm:"index" json:"createdAt"`
}
