package model

import "time"

type Upload struct {
	ID         string    `gorm:"primaryKey" json:"id" bson:"_id"`
	FileName   string    `gorm:"not null" json:"file_name" bson:"file_name"`
	StoredName string    `gorm:"not null" json:"stored_name" bson:"stored_name"`
	Path       string    `gorm:"not null" json:"path" bson:"path"`
	Size       int64     `gorm:"not null" json:"size" bson:"size"`
	MimeType   string    `gorm:"size:128" json:"mime_type" bson:"mime_type"`
	UploaderID string    `gorm:"index;not null" json:"uploader_id" bson:"uploader_id"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
}

func (Upload) TableName() string { return "uploads" }
