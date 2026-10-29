package model

import "time"

type PostFavorite struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	PostID    string    `gorm:"not null;uniqueIndex:idx_post_user" json:"post_id"`
	UserID    string    `gorm:"not null;uniqueIndex:idx_post_user" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (PostFavorite) TableName() string { return "post_favorites" }
