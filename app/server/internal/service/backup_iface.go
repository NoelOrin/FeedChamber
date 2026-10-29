package service

import (
	"context"
	"io"

	"feedchamber/internal/model"
)

type BackupService interface {
	Create(ctx context.Context, actorID, name string) (*model.Backup, error)
	Cleanup(ctx context.Context) (int, error)
	Restore(ctx context.Context, id, actorID string) error
	List(ctx context.Context, page, pageSize int) ([]model.Backup, int, error)
	Download(ctx context.Context, id string) (*model.Backup, io.ReadCloser, error)
	Delete(ctx context.Context, id string) error
}
