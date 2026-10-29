package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type RevisionRepository interface {
	Create(ctx context.Context, rev *model.Revision) error
	ListByTarget(ctx context.Context, targetType, targetID string, offset, limit int) ([]model.Revision, int, error)
	GetByID(ctx context.Context, id string) (*model.Revision, error)
}

func NewRevisionRepository(store db.Store) RevisionRepository {
	return &sqlRevisionRepo{db: store.SQL()}
}
