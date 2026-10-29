package service

import (
	"context"

	"feedchamber/internal/model"
)

type DraftService interface {
	Save(ctx context.Context, userID, kind, targetID, title, body, spaceID string, tags []string) (*model.Draft, error)
	List(ctx context.Context, userID string, page, pageSize int) ([]model.Draft, int, error)
	Delete(ctx context.Context, userID, id string) error
}
