package model

import "time"

type SecretType string

const (
	SecretTypeSSHPassword   SecretType = "SSH_PASSWORD"
	SecretTypeSSHPrivateKey SecretType = "SSH_PRIVATE_KEY"
	SecretTypeDockerTLS     SecretType = "DOCKER_TLS"
	SecretTypeDockerToken   SecretType = "DOCKER_TOKEN"
)

type Secret struct {
	ID          string     `gorm:"primaryKey;size:36" json:"id"`
	Name        string     `gorm:"size:128;not null;index" json:"name"`
	Type        SecretType `gorm:"size:32;not null;index" json:"type"`
	Description string     `gorm:"size:512" json:"description"`
	Ciphertext  string     `gorm:"type:text;not null" json:"-"`
	Nonce       string     `gorm:"size:128;not null" json:"-"`
	MaskedValue string     `gorm:"size:256" json:"maskedValue"`
	CreatedBy   string     `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy   string     `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `gorm:"index" json:"-"`
}
