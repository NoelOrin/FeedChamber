package model

import "time"

type Backup struct {
	ID              string    `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:128;not null" json:"name"`
	FileKey         string    `gorm:"size:512;not null" json:"file_key"`
	Size            int64     `json:"size"`
	Checksum        string    `gorm:"size:64" json:"checksum"`
	UploadsKey      string    `gorm:"size:512" json:"uploads_key,omitempty"`
	UploadsSize     int64     `json:"uploads_size,omitempty"`
	UploadsChecksum string    `gorm:"size:64" json:"uploads_checksum,omitempty"`
	Status          string    `gorm:"size:16;default:ready" json:"status"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

func (Backup) TableName() string { return "backups" }
