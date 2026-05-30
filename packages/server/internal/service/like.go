package service

import (
	"context"
	"orinforum/internal/repository"
)

type LikeService interface {
	ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error)
	HasContentLiked(ctx context.Context, contentID, userID string) (bool, error)
	CountContentLikes(ctx context.Context, contentID string) (int64, error)
}

type likeService struct {
	repo repository.LikeRepository
}

func NewLikeService(repo repository.LikeRepository) LikeService {
	return &likeService{repo: repo}
}

func (s *likeService) ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error) {
	return s.repo.ToggleContentLike(ctx, contentID, userID)
}

func (s *likeService) HasContentLiked(ctx context.Context, contentID, userID string) (bool, error) {
	return s.repo.HasContentLiked(ctx, contentID, userID)
}

func (s *likeService) CountContentLikes(ctx context.Context, contentID string) (int64, error) {
	return s.repo.CountContentLikes(ctx, contentID)
}
