package model

import "time"

type PostFavorite struct {
	ID        string    `gorm:"primaryKey" json:"id" bson:"_id"`
	PostID    string    `gorm:"index;not null" json:"post_id" bson:"post_id"`
	UserID    string    `gorm:"index;not null" json:"user_id" bson:"user_id"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

func (PostFavorite) TableName() string { return "post_favorites" }
