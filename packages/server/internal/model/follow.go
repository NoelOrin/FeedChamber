package model

import "time"

type UserFollow struct {
	ID          string    `gorm:"primaryKey" bson:"_id"`
	FollowerID  string    `gorm:"index;not null" bson:"follower_id"`
	FollowingID string    `gorm:"index;not null" bson:"following_id"`
	CreatedAt   time.Time `bson:"created_at"`
}

func (UserFollow) TableName() string { return "user_follows" }
