package repository

import (
	"context"
	"feedchamber/internal/db"
)

type FollowRepository interface {
	Follow(ctx context.Context, followerID, followingID string) error
	Unfollow(ctx context.Context, followerID, followingID string) error
	IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
	GetFollowers(ctx context.Context, userID string, offset, limit int) ([]string, error)
	GetFollowing(ctx context.Context, userID string, offset, limit int) ([]string, error)
	CountFollowers(ctx context.Context, userID string) (int64, error)
	CountFollowing(ctx context.Context, userID string) (int64, error)
}

func NewFollowRepository(store db.Store) FollowRepository {
	return &sqlFollowRepo{db: store.SQL()}
}
