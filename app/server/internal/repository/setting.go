package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type SettingRepository interface {
	Get(ctx context.Context, key string) (*model.Setting, error)
	GetAll(ctx context.Context) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	BatchSet(ctx context.Context, settings map[string]string) error
	Delete(ctx context.Context, key string) error
}

func NewSettingRepository(store db.Store) SettingRepository {
	return &sqlSettingRepo{db: store.SQL()}
}
