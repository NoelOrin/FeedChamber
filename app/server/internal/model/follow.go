package model

import "time"

type UserFollow struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	FollowerID  string    `gorm:"not null;uniqueIndex:idx_follow_pair" json:"follower_id"`
	FollowingID string    `gorm:"not null;uniqueIndex:idx_follow_pair" json:"following_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (UserFollow) TableName() string { return "user_follows" }
