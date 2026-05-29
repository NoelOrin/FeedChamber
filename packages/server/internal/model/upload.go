package model

import "time"

type Upload struct {
	ID         string    `gorm:"primaryKey" bson:"_id"`
	FileName   string    `gorm:"not null" bson:"file_name"`      // original filename
	StoredName string    `gorm:"not null" bson:"stored_name"`    // uuid-based name on disk
	Path       string    `gorm:"not null" bson:"path"`           // relative path
	Size       int64     `gorm:"not null" bson:"size"`           // bytes
	MimeType   string    `gorm:"size:128" bson:"mime_type"`      // e.g. "image/png"
	UploaderID string    `gorm:"index;not null" bson:"uploader_id"`
	CreatedAt  time.Time `bson:"created_at"`
}

func (Upload) TableName() string { return "uploads" }
