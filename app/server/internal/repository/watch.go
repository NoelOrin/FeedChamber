package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type WatchRepository interface {
	Upsert(ctx context.Context, w *model.Watch) error
	Delete(ctx context.Context, userID, targetType, targetID string) error
	Get(ctx context.Context, userID, targetType, targetID string) (*model.Watch, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Watch, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
	// ListActiveUserIDs returns user IDs watching target whose level is not muted.
	ListActiveUserIDs(ctx context.Context, targetType, targetID string) ([]string, error)
}

func NewWatchRepository(store db.Store) WatchRepository {
	return &sqlWatchRepo{db: store.SQL()}
}
