package model

import "time"

type TerminalSessionStatus string

const (
	TerminalSessionStatusConnected    TerminalSessionStatus = "CONNECTED"
	TerminalSessionStatusDisconnected TerminalSessionStatus = "DISCONNECTED"
)

type TerminalSession struct {
	ID        string                `gorm:"primaryKey;size:36" json:"id"`
	HostID    string                `gorm:"size:36;not null;index" json:"hostId"`
	HostName  string                `gorm:"size:128;not null" json:"hostName"`
	Status    TerminalSessionStatus `gorm:"size:32;not null;default:CONNECTED;index" json:"status"`
	CreatedBy string                `gorm:"size:36;index" json:"createdBy"`
	ClosedAt  *time.Time            `json:"closedAt"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
}
