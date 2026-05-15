package model

import "time"

type ExportJobType string

const (
	ExportJobTypeResource ExportJobType = "resource"
	ExportJobTypeRecords  ExportJobType = "records"
	ExportJobTypeIncident ExportJobType = "incident"
	ExportJobTypeBackup   ExportJobType = "backup"
)

type ExportJobStatus string

const (
	ExportJobStatusPending ExportJobStatus = "PENDING"
	ExportJobStatusRunning ExportJobStatus = "RUNNING"
	ExportJobStatusSuccess ExportJobStatus = "SUCCESS"
	ExportJobStatusFailed  ExportJobStatus = "FAILED"
)

type BackupRecordType string

const (
	BackupRecordTypeManual    BackupRecordType = "manual"
	BackupRecordTypeScheduled BackupRecordType = "scheduled"
)

type ExportJob struct {
	ID           string          `gorm:"primaryKey;size:36" json:"id"`
	Type         ExportJobType   `gorm:"size:32;not null;index" json:"type"`
	Status       ExportJobStatus `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	ResourceType string          `gorm:"size:64;index" json:"resourceType"`
	ResourceID   string          `gorm:"size:128;index" json:"resourceId"`
	FiltersJSON  string          `gorm:"type:text" json:"filtersJson"`
	FileName     string          `gorm:"size:255" json:"fileName"`
	FilePath     string          `gorm:"size:1024" json:"-"`
	FileSize     int64           `json:"fileSize"`
	ContentType  string          `gorm:"size:128" json:"contentType"`
	Masked       bool            `gorm:"not null;default:true;index" json:"masked"`
	CreatedBy    string          `gorm:"size:36;index" json:"createdBy"`
	ErrorMessage string          `gorm:"type:text" json:"errorMessage"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	FinishedAt   *time.Time      `json:"finishedAt"`
}

type BackupRecord struct {
	ID           string           `gorm:"primaryKey;size:36" json:"id"`
	Type         BackupRecordType `gorm:"size:32;not null;default:manual;index" json:"type"`
	Status       ExportJobStatus  `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	FileName     string           `gorm:"size:255" json:"fileName"`
	FilePath     string           `gorm:"size:1024" json:"-"`
	FileSize     int64            `json:"fileSize"`
	Checksum     string           `gorm:"size:128;index" json:"checksum"`
	ManifestJSON string           `gorm:"type:text" json:"manifestJson"`
	Masked       bool             `gorm:"not null;default:true;index" json:"masked"`
	CreatedBy    string           `gorm:"size:36;index" json:"createdBy"`
	ErrorMessage string           `gorm:"type:text" json:"errorMessage"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
	FinishedAt   *time.Time       `json:"finishedAt"`
}
