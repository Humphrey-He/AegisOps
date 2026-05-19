package model

import "time"

type NginxNodeStatus string

const (
	NginxNodeStatusUnknown NginxNodeStatus = "UNKNOWN"
	NginxNodeStatusOnline  NginxNodeStatus = "ONLINE"
	NginxNodeStatusOffline NginxNodeStatus = "OFFLINE"
)

type NginxNode struct {
	ID            string          `gorm:"primaryKey;size:36" json:"id"`
	Name          string          `gorm:"size:128;not null;index" json:"name"`
	HostID        string          `gorm:"size:36;not null;index" json:"hostId"`
	Environment   string          `gorm:"size:64;index" json:"environment"`
	ConfigPath    string          `gorm:"size:512;not null" json:"configPath"`
	TestCommand   string          `gorm:"size:512;not null" json:"testCommand"`
	ReloadCommand string          `gorm:"size:512;not null" json:"reloadCommand"`
	Description   string          `gorm:"size:512" json:"description"`
	Status        NginxNodeStatus `gorm:"size:32;not null;default:UNKNOWN;index" json:"status"`
	LastTestAt    *time.Time      `json:"lastTestAt"`
	CreatedBy     string          `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy     string          `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	DeletedAt     *time.Time      `gorm:"index" json:"-"`
	Host          *Host           `gorm:"foreignKey:HostID" json:"host,omitempty"`
}

type NginxConfigStatus string

const (
	NginxConfigStatusDraft  NginxConfigStatus = "DRAFT"
	NginxConfigStatusActive NginxConfigStatus = "ACTIVE"
)

type NginxConfigVersion struct {
	ID        string            `gorm:"primaryKey;size:36" json:"id"`
	NodeID    string            `gorm:"size:36;not null;index;index:idx_nginx_configs_node_active,priority:1" json:"nodeId"`
	Version   string            `gorm:"size:128;not null;index" json:"version"`
	Content   string            `gorm:"type:text;not null" json:"content"`
	Checksum  string            `gorm:"size:64;not null;index" json:"checksum"`
	Status    NginxConfigStatus `gorm:"size:32;not null;default:DRAFT;index;index:idx_nginx_configs_node_active,priority:2" json:"status"`
	Message   string            `gorm:"size:512" json:"message"`
	CreatedBy string            `gorm:"size:36;index" json:"createdBy"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	DeletedAt *time.Time        `gorm:"index" json:"-"`
	NginxNode *NginxNode        `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}
