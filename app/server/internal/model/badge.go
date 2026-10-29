package model

import "time"

type Badge struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"uniqueIndex;size:64;not null" json:"code"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Icon        string    `gorm:"size:64" json:"icon"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Badge) TableName() string { return "badges" }

type UserBadge struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"uniqueIndex:idx_user_badge;not null" json:"user_id"`
	BadgeID   string    `gorm:"uniqueIndex:idx_user_badge;not null" json:"badge_id"`
	AwardedAt time.Time `json:"awarded_at"`
}

func (UserBadge) TableName() string { return "user_badges" }

const (
	BadgeFirstPost = "first_post"
	BadgeStreak7   = "streak_7"
	BadgeLiked10   = "liked_10"
)
