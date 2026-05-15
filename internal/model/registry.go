package model

import "time"

type RegistryAuthType string

const (
	RegistryAuthTypeNone  RegistryAuthType = "NONE"
	RegistryAuthTypeBasic RegistryAuthType = "BASIC"
	RegistryAuthTypeToken RegistryAuthType = "TOKEN"
)

type RegistryStatus string

const (
	RegistryStatusUnknown RegistryStatus = "UNKNOWN"
	RegistryStatusOnline  RegistryStatus = "ONLINE"
	RegistryStatusOffline RegistryStatus = "OFFLINE"
)

type Registry struct {
	ID          string           `gorm:"primaryKey;size:36" json:"id"`
	Name        string           `gorm:"size:128;not null;index" json:"name"`
	URL         string           `gorm:"size:512;not null" json:"url"`
	AuthType    RegistryAuthType `gorm:"size:32;not null;default:NONE" json:"authType"`
	SecretID    string           `gorm:"size:36;index" json:"secretId"`
	Description string           `gorm:"size:512" json:"description"`
	Status      RegistryStatus   `gorm:"size:32;not null;default:UNKNOWN;index" json:"status"`
	LastTestAt  *time.Time       `json:"lastTestAt"`
	CreatedBy   string           `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy   string           `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	DeletedAt   *time.Time       `gorm:"index" json:"-"`
}

func AllModels() []any {
	return []any{
		&User{},
		&Role{},
		&Permission{},
		&UserRole{},
		&RolePermission{},
		&AuditLog{},
		&Secret{},
		&SecretReference{},
		&SecretReadAudit{},
		&Host{},
		&Task{},
		&TaskStep{},
		&TaskLog{},
		&DockerNode{},
		&TerminalSession{},
		&Registry{},
		&ServiceDefinition{},
		&ServiceVersion{},
		&ServiceInstance{},
		&ServiceReleaseRecord{},
		&NginxNode{},
		&NginxConfigVersion{},
		&MockDockerContainer{},
		&NotificationChannel{},
		&AlertRule{},
		&AlertEvent{},
		&NotificationRecord{},
		&ServiceHealthCheck{},
		&HostAvailabilityCheck{},
		&ExportJob{},
		&BackupRecord{},
		&ScheduledJob{},
		&TaskDispatch{},
	}
}
