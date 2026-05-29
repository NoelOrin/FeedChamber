package service

import (
	"context"

	"orinforum/internal/pkg"
	"orinforum/internal/repository"
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

type followService struct {
	repo repository.FollowRepository
}

func NewFollowService(repo repository.FollowRepository) FollowService {
	return &followService{repo: repo}
}

func (s *followService) Follow(ctx context.Context, followerID, followingID string) error {
	if followerID == followingID {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "cannot follow yourself")
	}
	return s.repo.Follow(ctx, followerID, followingID)
}

func (s *followService) Unfollow(ctx context.Context, followerID, followingID string) error {
	return s.repo.Unfollow(ctx, followerID, followingID)
}

func (s *followService) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	return s.repo.IsFollowing(ctx, followerID, followingID)
}

func (s *followService) GetFollowers(ctx context.Context, userID string, page, pageSize int) ([]string, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.GetFollowers(ctx, userID, (page-1)*pageSize, pageSize)
}

func (s *followService) GetFollowing(ctx context.Context, userID string, page, pageSize int) ([]string, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.GetFollowing(ctx, userID, (page-1)*pageSize, pageSize)
}

func (s *followService) CountFollowers(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountFollowers(ctx, userID)
}

func (s *followService) CountFollowing(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountFollowing(ctx, userID)
}
