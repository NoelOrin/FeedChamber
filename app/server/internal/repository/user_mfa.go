package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type UserMFARepository interface {
	Get(ctx context.Context, userID string) (*model.UserMFA, error)
	Upsert(ctx context.Context, m *model.UserMFA) error
	Delete(ctx context.Context, userID string) error
}

func NewUserMFARepository(store db.Store) UserMFARepository {
	return &sqlUserMFARepo{db: store.SQL()}
}
