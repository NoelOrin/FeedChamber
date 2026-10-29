package service

import (
	"context"

	"orinforum/internal/model"
)

type RevisionService interface {
	List(ctx context.Context, targetType, targetID string, page, pageSize int) ([]model.Revision, int, error)
	RestorePost(ctx context.Context, revisionID, actorID, actorRole string) (*model.Post, error)
	RestoreComment(ctx context.Context, revisionID, actorID, actorRole string) (*model.Content, error)
}
