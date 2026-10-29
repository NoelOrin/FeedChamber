package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type DraftRepository interface {
	Upsert(ctx context.Context, draft *model.Draft) (*model.Draft, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Draft, int, error)
	GetByID(ctx context.Context, id, userID string) (*model.Draft, error)
	Delete(ctx context.Context, id, userID string) error
}

func NewDraftRepository(store db.Store) DraftRepository {
	return &sqlDraftRepo{db: store.SQL()}
}
