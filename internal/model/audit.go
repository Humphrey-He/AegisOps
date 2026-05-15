package model

import "time"

type AuditResult string

const (
	AuditResultSuccess AuditResult = "success"
	AuditResultFailure AuditResult = "failure"
)

type AuditLog struct {
	ID           uint        `json:"id" gorm:"primaryKey"`
	UserID       *uint       `json:"userId" gorm:"index;index:idx_audit_logs_created_user_resource,priority:2"`
	Username     string      `json:"username" gorm:"size:64;index"`
	Action       string      `json:"action" gorm:"size:128;not null;index"`
	ResourceType string      `json:"resourceType" gorm:"size:64;index;index:idx_audit_logs_created_user_resource,priority:3"`
	ResourceID   string      `json:"resourceId" gorm:"size:64;index"`
	Result       AuditResult `json:"result" gorm:"size:32;not null;index"`
	Message      string      `json:"message" gorm:"size:512"`
	IPAddress    string      `json:"ipAddress" gorm:"size:64"`
	UserAgent    string      `json:"userAgent" gorm:"size:255"`
	TraceID      string      `json:"traceId" gorm:"size:128;index"`
	CreatedAt    time.Time   `json:"createdAt" gorm:"index;index:idx_audit_logs_created_user_resource,priority:1"`
}
