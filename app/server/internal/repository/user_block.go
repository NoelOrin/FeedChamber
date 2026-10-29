package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

type UserBlockRepository interface {
	Create(ctx context.Context, b *model.UserBlock) error
	Delete(ctx context.Context, blockerID, blockedID string) error
	Exists(ctx context.Context, blockerID, blockedID string) (bool, error)
	// EitherBlocked returns true if either user blocks the other.
	EitherBlocked(ctx context.Context, a, b string) (bool, error)
	ListBlockedIDs(ctx context.Context, blockerID string, offset, limit int) ([]string, error)
	CountBlocked(ctx context.Context, blockerID string) (int64, error)
}

func NewUserBlockRepository(store db.Store) UserBlockRepository {
	return &sqlUserBlockRepo{db: store.SQL()}
}
