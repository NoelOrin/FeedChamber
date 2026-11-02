package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type UploadRepository interface {
	Create(ctx context.Context, upload *model.Upload) error
	FindByID(ctx context.Context, id string) (*model.Upload, error)
	FindByKey(ctx context.Context, key string) (*model.Upload, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Upload, error)
	Delete(ctx context.Context, id string) error
}

func NewUploadRepository(store db.Store) UploadRepository {
	return &sqlUploadRepo{db: store.SQL()}
}
