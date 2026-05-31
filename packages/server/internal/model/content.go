package model

import (
	"time"

	"gorm.io/gorm"
)

type Content struct {
	ID        string         `gorm:"primaryKey" json:"id" bson:"_id"`
	AuthorID  string         `gorm:"not null;index" json:"author_id" bson:"author_id"`
	Body      string         `gorm:"type:text;not null" json:"body" bson:"body"`
	RootID    string         `gorm:"not null;index" json:"root_id" bson:"root_id"`
	ParentID  string         `gorm:"index" json:"parent_id" bson:"parent_id"`
	LikeCount int64          `gorm:"default:0" json:"like_count" bson:"like_count"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" bson:"deleted_at" swaggertype:"string"`
	CreatedAt time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time      `json:"updated_at" bson:"updated_at"`
}

func (Content) TableName() string { return "contents" }

// ContentListItem is the DTO for content with author info.
type ContentListItem struct {
	ID           string    `json:"id"`
	AuthorID     string    `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar string    `json:"author_avatar"`
	Body         string    `json:"body"`
	RootID       string    `json:"root_id"`
	ParentID     string    `json:"parent_id"`
	LikeCount    int64     `json:"like_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
