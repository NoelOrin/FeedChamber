package model

import "time"

// Banned 禁言记录表
type Banned struct {
	ID        string    `gorm:"primaryKey" json:"id" bson:"_id"`
	UserID    string    `gorm:"index;not null" json:"user_id" bson:"user_id"`
	Reason    string    `gorm:"size:500" json:"reason" bson:"reason"`
	IsPermanent bool    `gorm:"default:false" json:"is_permanent" bson:"is_permanent"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
	CreatedBy  string    `gorm:"size:64" json:"created_by" bson:"created_by"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}

func (Banned) TableName() string { return "banned" }
