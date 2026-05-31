package model

import "time"

type ContentLike struct {
	ID        string    `gorm:"primaryKey" json:"id" bson:"_id"`
	ContentID string    `gorm:"index;not null" json:"content_id" bson:"content_id"`
	UserID    string    `gorm:"index;not null" json:"user_id" bson:"user_id"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

func (ContentLike) TableName() string { return "content_likes" }
