package repository

import (
	"context"
	"feedchamber/internal/db"
)

type FavoriteRepository interface {
	Toggle(ctx context.Context, postID, userID string) (bool, error)
	HasFavorited(ctx context.Context, postID, userID string) (bool, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]string, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
}

func NewFavoriteRepository(store db.Store) FavoriteRepository {
	return &sqlFavRepo{db: store.SQL()}
}
