package model

import "time"

type SecretType string

const (
	SecretTypeSSHPassword   SecretType = "SSH_PASSWORD"
	SecretTypeSSHPrivateKey SecretType = "SSH_PRIVATE_KEY"
	SecretTypeDockerTLS     SecretType = "DOCKER_TLS"
	SecretTypeDockerToken   SecretType = "DOCKER_TOKEN"
	SecretTypeWebhook       SecretType = "WEBHOOK"
	SecretTypeAPIToken      SecretType = "API_TOKEN"
	SecretTypeSMTP          SecretType = "SMTP"
)

type SecretStatus string

const (
	SecretStatusActive   SecretStatus = "ACTIVE"
	SecretStatusDisabled SecretStatus = "DISABLED"
)

type Secret struct {
	ID            string       `gorm:"primaryKey;size:36" json:"id"`
	Name          string       `gorm:"size:128;not null;index" json:"name"`
	Type          SecretType   `gorm:"size:32;not null;index" json:"type"`
	Description   string       `gorm:"size:512" json:"description"`
	Purpose       string       `gorm:"size:128;index" json:"purpose"`
	Status        SecretStatus `gorm:"size:32;not null;default:ACTIVE;index" json:"status"`
	Ciphertext    string       `gorm:"type:text;not null" json:"-"`
	Nonce         string       `gorm:"size:128;not null" json:"-"`
	MaskedValue   string       `gorm:"size:256" json:"maskedValue"`
	KeyVersion    int          `gorm:"not null;default:1" json:"keyVersion"`
	LastRotatedAt *time.Time   `json:"lastRotatedAt"`
	ExpiresAt     *time.Time   `json:"expiresAt"`
	CreatedBy     string       `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy     string       `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
	DeletedAt     *time.Time   `gorm:"index" json:"-"`
}

type SecretReference struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	SecretID     string    `gorm:"size:36;not null;index;index:idx_secret_refs_resource,priority:2" json:"secretId"`
	ResourceType string    `gorm:"size:64;not null;index;index:idx_secret_refs_resource,priority:1" json:"resourceType"`
	ResourceID   string    `gorm:"size:128;not null;index;index:idx_secret_refs_resource,priority:3" json:"resourceId"`
	FieldName    string    `gorm:"size:128;not null" json:"fieldName"`
	CreatedBy    string    `gorm:"size:36;index" json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
}

type SecretReadAudit struct {
	ID           string      `gorm:"primaryKey;size:36" json:"id"`
	SecretID     string      `gorm:"size:36;not null;index" json:"secretId"`
	ResourceType string      `gorm:"size:64;index" json:"resourceType"`
	ResourceID   string      `gorm:"size:128;index" json:"resourceId"`
	Action       string      `gorm:"size:128;not null;index" json:"action"`
	OperatorID   string      `gorm:"size:36;index" json:"operatorId"`
	TaskID       string      `gorm:"size:36;index" json:"taskId"`
	Result       AuditResult `gorm:"size:32;not null;index" json:"result"`
	ErrorMessage string      `gorm:"type:text" json:"errorMessage"`
	CreatedAt    time.Time   `gorm:"index" json:"createdAt"`
}
