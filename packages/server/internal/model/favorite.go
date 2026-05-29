package model

import "time"

type PostFavorite struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	PostID    string    `gorm:"index;not null" bson:"post_id"`
	UserID    string    `gorm:"index;not null" bson:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
}

func (PostFavorite) TableName() string { return "post_favorites" }
