package model

import "time"

type PostLike struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	PostID    string    `gorm:"index;not null" bson:"post_id"`
	UserID    string    `gorm:"index;not null" bson:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
}

func (PostLike) TableName() string { return "post_likes" }

type CommentLike struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	CommentID string    `gorm:"index;not null" bson:"comment_id"`
	UserID    string    `gorm:"index;not null" bson:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
}

func (CommentLike) TableName() string { return "comment_likes" }
