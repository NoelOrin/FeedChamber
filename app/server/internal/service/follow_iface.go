package service

import (
	"context"
)

type FollowService interface {
	Follow(ctx context.Context, followerID, followingID string) error
	Unfollow(ctx context.Context, followerID, followingID string) error
	IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
	GetFollowers(ctx context.Context, userID string, page, pageSize int) ([]string, error)
	GetFollowing(ctx context.Context, userID string, page, pageSize int) ([]string, error)
	CountFollowers(ctx context.Context, userID string) (int64, error)
	CountFollowing(ctx context.Context, userID string) (int64, error)
}
