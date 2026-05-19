package model

import "time"

type DockerNodeStatus string

const (
	DockerNodeStatusUnknown DockerNodeStatus = "UNKNOWN"
	DockerNodeStatusOnline  DockerNodeStatus = "ONLINE"
	DockerNodeStatusOffline DockerNodeStatus = "OFFLINE"
)

type DockerAuthType string

const (
	DockerAuthTypeNone  DockerAuthType = "NONE"
	DockerAuthTypeTLS   DockerAuthType = "TLS"
	DockerAuthTypeToken DockerAuthType = "TOKEN"
)

type DockerNode struct {
	ID          string           `gorm:"primaryKey;size:36" json:"id"`
	Name        string           `gorm:"size:128;not null;index" json:"name"`
	Endpoint    string           `gorm:"size:255;not null" json:"endpoint"`
	AuthType    DockerAuthType   `gorm:"size:32;not null;default:NONE" json:"authType"`
	SecretID    string           `gorm:"size:36;index" json:"secretId"`
	Environment string           `gorm:"size:64;index" json:"environment"`
	Description string           `gorm:"size:512" json:"description"`
	Status      DockerNodeStatus `gorm:"size:32;not null;default:UNKNOWN;index" json:"status"`
	LastTestAt  *time.Time       `json:"lastTestAt"`
	CreatedBy   string           `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy   string           `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	DeletedAt   *time.Time       `gorm:"index" json:"-"`
}
