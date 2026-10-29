package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type BadgeRepository interface {
	ListBadges(ctx context.Context) ([]model.Badge, error)
	GetByCode(ctx context.Context, code string) (*model.Badge, error)
	UpsertBadge(ctx context.Context, b *model.Badge) error
	Award(ctx context.Context, ub *model.UserBadge) error
	HasBadge(ctx context.Context, userID, badgeID string) (bool, error)
	ListUserBadges(ctx context.Context, userID string) ([]model.Badge, error)
}

func NewBadgeRepository(store db.Store) BadgeRepository {
	return &sqlBadgeRepo{db: store.SQL()}
}
