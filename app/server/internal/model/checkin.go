package model

import "time"

type CheckIn struct {
	ID          string `gorm:"primaryKey" json:"id"`
	UserID      string `gorm:"index:idx_user_date,unique;not null" json:"user_id"`
	CheckInDate string `gorm:"index:idx_user_date,unique;size:10;not null" json:"check_in_date"`
	// PointsEarned 是旧版签到积分的遗留列，自 2026-10-29 起恒为 0，
	// 不再参与 XP 求值（XP 由行为聚合派生）。
	PointsEarned int       `gorm:"default:0" json:"points_earned"`
	Streak       int       `gorm:"default:0" json:"streak"`
	CreatedAt    time.Time `json:"created_at"`
}

func (CheckIn) TableName() string { return "check_ins" }
