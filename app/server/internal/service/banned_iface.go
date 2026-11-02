package service

import (
	"context"

	"feedchamber/internal/model"
)

type BannedService interface {
	// 全局禁言
	Ban(ctx context.Context, userID, reason, createdBy string, isPermanent bool, durationMinutes *int) (*model.Banned, error)
	Unban(ctx context.Context, userID string) error
	GetByUserID(ctx context.Context, userID string) (*model.Banned, error)

	// 版面禁言
	BanInSpace(ctx context.Context, userID, spaceID, reason, createdBy string, isPermanent bool, durationMinutes *int) (*model.Banned, error)
	UnbanInSpace(ctx context.Context, userID, spaceID string) error
	GetInSpace(ctx context.Context, userID, spaceID string) (*model.Banned, error)

	// 通用方法
	IsBanned(ctx context.Context, userID string) bool
	IsBannedInSpace(ctx context.Context, userID, spaceID string) bool
	// List / ListByScope 返回 (items, total, error)：handler 用 total 构造分页信封。
	List(ctx context.Context, page, pageSize int) ([]model.Banned, int64, error)
	ListByScope(ctx context.Context, scope, spaceID string, page, pageSize int) ([]model.Banned, int64, error)
	ListByUserID(ctx context.Context, userID string) ([]model.Banned, error)
}
