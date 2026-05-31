package model

import "time"

type UserFollow struct {
	ID          string    `gorm:"primaryKey" json:"id" bson:"_id"`
	FollowerID  string    `gorm:"index;not null" json:"follower_id" bson:"follower_id"`
	FollowingID string    `gorm:"index;not null" json:"following_id" bson:"following_id"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
}

func (UserFollow) TableName() string { return "user_follows" }
