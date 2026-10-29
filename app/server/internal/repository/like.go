package repository

import (
	"context"
	"orinforum/internal/db"
)

type LikeRepository interface {
	ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error)
	HasContentLiked(ctx context.Context, contentID, userID string) (bool, error)
	CountContentLikes(ctx context.Context, contentID string) (int64, error)
}

func NewLikeRepository(store db.Store) LikeRepository {
	return &sqlLikeRepo{db: store.SQL()}
}
