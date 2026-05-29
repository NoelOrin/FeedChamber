package model

import "time"

type Report struct {
	ID         string    `gorm:"primaryKey" bson:"_id"`
	ReporterID string    `gorm:"index;not null" bson:"reporter_id"`
	TargetType string    `gorm:"size:16;not null" bson:"target_type"` // "post" or "comment"
	TargetID   string    `gorm:"index;not null" bson:"target_id"`
	Reason     string    `gorm:"size:500;not null" bson:"reason"`
	Status     string    `gorm:"size:16;default:pending" bson:"status"` // "pending", "resolved", "dismissed"
	CreatedAt  time.Time `bson:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

func (Report) TableName() string { return "reports" }
