package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type BannedRepository interface {
	FindByUserID(ctx context.Context, userID string) (*model.Banned, error)
	FindByUserIDAndScope(ctx context.Context, userID, scope, spaceID string) (*model.Banned, error)
	Create(ctx context.Context, banned *model.Banned) error
	Update(ctx context.Context, banned *model.Banned) error
	Delete(ctx context.Context, userID string) error
	DeleteByScope(ctx context.Context, userID, scope, spaceID string) error
	List(ctx context.Context, offset, limit int) ([]model.Banned, error)
	ListByScope(ctx context.Context, scope, spaceID string, offset, limit int) ([]model.Banned, error)
	ListByUserID(ctx context.Context, userID string) ([]model.Banned, error)
	// Count / CountByScope 与上面的列表方法成对出现：列表接口的分页条需要真实总数。
	Count(ctx context.Context) (int64, error)
	CountByScope(ctx context.Context, scope, spaceID string) (int64, error)
}

func NewBannedRepository(store db.Store) BannedRepository {
	return &sqlBannedRepo{db: store.SQL()}
}
