package model

import "time"

type Upload struct {
	ID         string    `gorm:"primaryKey" json:"id" bson:"_id"`
	FileName   string    `gorm:"not null" json:"file_name" bson:"file_name"`
	StoredName string    `gorm:"not null" json:"stored_name" bson:"stored_name"`
	Key        string    `gorm:"not null" json:"key" bson:"key"` // 对象存储 key
	Path       string    `json:"path" bson:"path"`               // 本地路径（local 模式）
	URL        string    `json:"url" bson:"url"`                 // 公开访问 URL
	Size       int64     `gorm:"not null" json:"size" bson:"size"`
	MimeType   string    `gorm:"size:128" json:"mime_type" bson:"mime_type"`
	UploaderID string    `gorm:"index;not null" json:"uploader_id" bson:"uploader_id"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
}

func (Upload) TableName() string { return "uploads" }
