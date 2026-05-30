package model

import (
	"time"

	"gorm.io/gorm"
)

type Content struct {
	ID        string         `gorm:"primaryKey" bson:"_id"`
	AuthorID  string         `gorm:"not null;index" bson:"author_id"`
	Body      string         `gorm:"type:text;not null" bson:"body"`
	RootID    string         `gorm:"not null;index" bson:"root_id"`    // points to post's content_id; self-referencing for post body
	ParentID  string         `gorm:"index" bson:"parent_id"`           // direct parent content; empty for post body
	LikeCount int64          `gorm:"default:0" bson:"like_count"`
	DeletedAt gorm.DeletedAt `gorm:"index" bson:"deleted_at" swaggertype:"string"`
	CreatedAt time.Time      `bson:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at"`
}

func (Content) TableName() string { return "contents" }

// ContentListItem is the DTO for content with author info.
type ContentListItem struct {
	ID           string    `json:"id"`
	AuthorID     string    `json:"authorId"`
	AuthorName   string    `json:"authorName"`
	AuthorAvatar string    `json:"authorAvatar"`
	Body         string    `json:"body"`
	RootID       string    `json:"rootId"`
	ParentID     string    `json:"parentId"`
	LikeCount    int64     `json:"likeCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
