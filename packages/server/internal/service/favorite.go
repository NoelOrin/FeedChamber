package service

import (
	"context"
	"orinforum/internal/repository"
)

type FavoriteService interface {
	Toggle(ctx context.Context, postID, userID string) (bool, error)
	HasFavorited(ctx context.Context, postID, userID string) (bool, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]string, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
}

type favoriteService struct {
	repo repository.FavoriteRepository
}

func NewFavoriteService(repo repository.FavoriteRepository) FavoriteService {
	return &favoriteService{repo: repo}
}

func (s *favoriteService) Toggle(ctx context.Context, postID, userID string) (bool, error) {
	return s.repo.Toggle(ctx, postID, userID)
}

func (s *favoriteService) HasFavorited(ctx context.Context, postID, userID string) (bool, error) {
	return s.repo.HasFavorited(ctx, postID, userID)
}

func (s *favoriteService) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]string, error) {
	offset := (page - 1) * pageSize
	return s.repo.ListByUser(ctx, userID, offset, pageSize)
}

func (s *favoriteService) CountByUser(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountByUser(ctx, userID)
}
