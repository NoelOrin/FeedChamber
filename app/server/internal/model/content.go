package model

import (
	"time"

	"gorm.io/gorm"
)

type Content struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	AuthorID     string         `gorm:"not null;index" json:"author_id"`
	Body         string         `gorm:"type:text;not null" json:"body"`
	RootID       string         `gorm:"not null;index" json:"root_id"`
	ParentID     string         `gorm:"index" json:"parent_id"`
	LikeCount    int64          `gorm:"default:0" json:"like_count"`
	Status       string         `gorm:"size:16;default:approved;index" json:"status"`
	Folded       bool           `gorm:"default:false" json:"folded"` // 动议 fold 结果：内容降权（不删）
	WatchedUntil *time.Time     `json:"watched_until,omitempty"`     // 动议 watch 结果：限时观察标记
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
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
	Status       string    `json:"status"`
	PostID       string    `json:"post_id"`
	PostTitle    string    `json:"post_title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
