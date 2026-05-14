package model

import "time"

type HostStatus string

const (
	HostStatusUnknown HostStatus = "UNKNOWN"
	HostStatusOnline  HostStatus = "ONLINE"
	HostStatusOffline HostStatus = "OFFLINE"
)

type Host struct {
	ID          string     `gorm:"primaryKey;size:36" json:"id"`
	Name        string     `gorm:"size:128;not null;index" json:"name"`
	Address     string     `gorm:"size:255;not null;index" json:"address"`
	SSHPort     int        `gorm:"not null;default:22" json:"sshPort"`
	SSHUser     string     `gorm:"size:128;not null" json:"sshUser"`
	SSHSecretID string     `gorm:"size:36;index" json:"sshSecretId"`
	Group       string     `gorm:"size:128;index" json:"group"`
	Tags        string     `gorm:"type:text" json:"tags"`
	Status      HostStatus `gorm:"size:32;not null;default:UNKNOWN;index" json:"status"`
	LastTestAt  *time.Time `json:"lastTestAt"`
	CreatedBy   string     `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy   string     `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `gorm:"index" json:"-"`
}
