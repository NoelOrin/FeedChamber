package model

import "time"

type ContentLike struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	ContentID string    `gorm:"not null;uniqueIndex:idx_content_user" json:"content_id"`
	UserID    string    `gorm:"not null;uniqueIndex:idx_content_user" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (ContentLike) TableName() string { return "content_likes" }
