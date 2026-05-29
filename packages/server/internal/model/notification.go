package model

import "time"

type Notification struct {
	ID          string    `gorm:"primaryKey" bson:"_id"`
	UserID      string    `gorm:"index;not null" bson:"user_id"`       // recipient
	Type        string    `gorm:"size:32;not null" bson:"type"`         // "comment", "like", "follow", "mention"
	Content     string    `gorm:"size:500" bson:"content"`              // display text
	ReferenceID string    `gorm:"index" bson:"reference_id"`            // post_id, comment_id, or user_id depending on type
	SenderID    string    `gorm:"index" bson:"sender_id"`               // who triggered the notification
	IsRead      bool      `gorm:"default:false" bson:"is_read"`
	CreatedAt   time.Time `bson:"created_at"`
}

func (Notification) TableName() string { return "notifications" }
