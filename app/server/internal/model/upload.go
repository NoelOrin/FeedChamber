package model

import "time"

type Upload struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	FileName   string    `gorm:"not null" json:"file_name"`
	StoredName string    `gorm:"not null" json:"stored_name"`
	Key        string    `gorm:"not null" json:"key"` // 对象存储 key
	Path       string    `json:"path"`                // 本地路径（local 模式）
	URL        string    `json:"url"`                 // 公开访问 URL
	Size       int64     `gorm:"not null" json:"size"`
	MimeType   string    `gorm:"size:128" json:"mime_type"`
	UploaderID string    `gorm:"index;not null" json:"uploader_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Upload) TableName() string { return "uploads" }
