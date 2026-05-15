package model

import "time"

type MockDockerContainerStatus string

const (
	MockDockerContainerRunning MockDockerContainerStatus = "running"
	MockDockerContainerExited  MockDockerContainerStatus = "exited"
	MockDockerContainerPaused  MockDockerContainerStatus = "paused"
)

type MockDockerContainer struct {
	ID           string                    `gorm:"primaryKey;size:128" json:"id"`
	NodeID       string                    `gorm:"size:36;not null;index" json:"nodeId"`
	ServiceID    string                    `gorm:"size:36;index" json:"serviceId"`
	Name         string                    `gorm:"size:128;not null;index" json:"name"`
	Image        string                    `gorm:"size:512;not null" json:"image"`
	Status       MockDockerContainerStatus `gorm:"size:32;not null;default:running;index" json:"status"`
	Ports        string                    `gorm:"type:text" json:"ports"`
	RestartCount int                       `gorm:"not null;default:0" json:"restartCount"`
	Logs         string                    `gorm:"type:text" json:"logs"`
	CreatedAt    time.Time                 `json:"createdAt"`
	UpdatedAt    time.Time                 `json:"updatedAt"`
	DeletedAt    *time.Time                `gorm:"index" json:"-"`
}
