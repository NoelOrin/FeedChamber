package model

import "time"

type Report struct {
	ID         string    `gorm:"primaryKey" json:"id" bson:"_id"`
	ReporterID string    `gorm:"index;not null" json:"reporter_id" bson:"reporter_id"`
	TargetType string    `gorm:"size:16;not null" json:"target_type" bson:"target_type"`
	TargetID   string    `gorm:"index;not null" json:"target_id" bson:"target_id"`
	Reason     string    `gorm:"size:500;not null" json:"reason" bson:"reason"`
	Status     string    `gorm:"size:16;default:pending" json:"status" bson:"status"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}

func (Report) TableName() string { return "reports" }
