package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type UserPreferenceRepository interface {
	Get(ctx context.Context, userID, key string) (string, error)
	Set(ctx context.Context, userID, key, value string) error
	ListByKey(ctx context.Context, key, value string, offset, limit int) ([]model.UserPreference, error)
	ListUsersWithKeyDefault(ctx context.Context, key, defaultValue string, offset, limit int) ([]string, error)
}

func NewUserPreferenceRepository(store db.Store) UserPreferenceRepository {
	return &sqlUserPrefRepo{db: store.SQL()}
}
