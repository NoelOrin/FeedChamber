package service

import (
	"context"
	"orinforum/internal/repository"
)

type LikeService interface {
	TogglePostLike(ctx context.Context, postID, userID string) (bool, error)
	ToggleCommentLike(ctx context.Context, commentID, userID string) (bool, error)
	HasPostLiked(ctx context.Context, postID, userID string) (bool, error)
	HasCommentLiked(ctx context.Context, commentID, userID string) (bool, error)
	CountPostLikes(ctx context.Context, postID string) (int64, error)
	CountCommentLikes(ctx context.Context, commentID string) (int64, error)
}

type likeService struct {
	repo repository.LikeRepository
}

func NewLikeService(repo repository.LikeRepository) LikeService {
	return &likeService{repo: repo}
}

func (s *likeService) TogglePostLike(ctx context.Context, postID, userID string) (bool, error) {
	return s.repo.TogglePostLike(ctx, postID, userID)
}

func (s *likeService) ToggleCommentLike(ctx context.Context, commentID, userID string) (bool, error) {
	return s.repo.ToggleCommentLike(ctx, commentID, userID)
}

func (s *likeService) HasPostLiked(ctx context.Context, postID, userID string) (bool, error) {
	return s.repo.HasPostLiked(ctx, postID, userID)
}

func (s *likeService) HasCommentLiked(ctx context.Context, commentID, userID string) (bool, error) {
	return s.repo.HasCommentLiked(ctx, commentID, userID)
}

func (s *likeService) CountPostLikes(ctx context.Context, postID string) (int64, error) {
	return s.repo.CountPostLikes(ctx, postID)
}

func (s *likeService) CountCommentLikes(ctx context.Context, commentID string) (int64, error) {
	return s.repo.CountCommentLikes(ctx, commentID)
}
