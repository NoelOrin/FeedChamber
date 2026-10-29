package service

import (
	"context"

	"orinforum/internal/model"
)

type CheckInStatus struct {
	CheckedInToday bool `json:"checked_in_today"`
	CurrentStreak  int  `json:"current_streak"`
	MaxStreak      int  `json:"max_streak"`
	TotalCheckIns  int  `json:"total_checkins"`
	TotalXP        int  `json:"total_xp"`
}

type CheckInService interface {
	CheckIn(ctx context.Context, userID string) (*model.CheckIn, error)
	GetStatus(ctx context.Context, userID string) (*CheckInStatus, error)
	GetHistory(ctx context.Context, userID string, year, month int) ([]model.CheckIn, error)
	// SetTrustService 注入等级评估服务。
	SetTrustService(svc TrustService)
}
