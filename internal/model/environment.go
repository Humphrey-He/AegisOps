package model

import "time"

type EnvironmentStatus string

const (
	EnvironmentStatusActive   EnvironmentStatus = "ACTIVE"
	EnvironmentStatusDisabled EnvironmentStatus = "DISABLED"
)

type Environment struct {
	ID          string            `gorm:"primaryKey;size:36" json:"id"`
	Name        string            `gorm:"size:128;not null;index" json:"name"`
	Code        string            `gorm:"size:64;not null;uniqueIndex" json:"code"`
	Description string            `gorm:"size:512" json:"description"`
	Status      EnvironmentStatus `gorm:"size:32;not null;default:ACTIVE;index" json:"status"`
	SortOrder   int               `gorm:"not null;default:0;index" json:"sortOrder"`
	CreatedBy   string            `gorm:"size:36;index" json:"createdBy"`
	UpdatedBy   string            `gorm:"size:36;index" json:"updatedBy"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	DeletedAt   *time.Time        `gorm:"index" json:"-"`
}
