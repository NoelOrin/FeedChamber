package model

import (
	"time"

	"gorm.io/gorm"
)

type Tag struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"uniqueIndex;not null" json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

func (Tag) TableName() string { return "tags" }

type PostTag struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	PostID string `gorm:"index;not null" json:"post_id"`
	TagID  string `gorm:"index;not null" json:"tag_id"`
}

func (PostTag) TableName() string { return "post_tags" }
