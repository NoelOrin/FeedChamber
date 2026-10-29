package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type BackupRepository interface {
	Create(ctx context.Context, b *model.Backup) error
	ListAll(ctx context.Context) ([]model.Backup, error)
	List(ctx context.Context, offset, limit int) ([]model.Backup, int, error)
	Get(ctx context.Context, id string) (*model.Backup, error)
	Delete(ctx context.Context, id string) error
}

func NewBackupRepository(store db.Store) BackupRepository {
	return &sqlBackupRepo{db: store.SQL()}
}
