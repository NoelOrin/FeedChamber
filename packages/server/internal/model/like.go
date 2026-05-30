package model

import "time"

type ContentLike struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	ContentID string    `gorm:"index;not null" bson:"content_id"`
	UserID    string    `gorm:"index;not null" bson:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
}

func (ContentLike) TableName() string { return "content_likes" }
