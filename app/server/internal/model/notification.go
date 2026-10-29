package model

import "time"

type Notification struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	UserID      string    `gorm:"index;not null" json:"user_id"`
	Type        string    `gorm:"size:32;not null" json:"type"`
	Content     string    `gorm:"size:500" json:"content"`
	ReferenceID string    `gorm:"index" json:"reference_id"`
	SenderID    string    `gorm:"index" json:"sender_id"`
	IsRead      bool      `gorm:"default:false" json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }
