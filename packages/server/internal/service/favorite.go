package service

import (
	"context"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type FavoriteService interface {
	Toggle(ctx context.Context, postID, userID string) (bool, error)
	HasFavorited(ctx context.Context, postID, userID string) (bool, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]string, int, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
}

type favoriteService struct {
	repo     repository.FavoriteRepository
	postRepo repository.PostRepository
}

func NewFavoriteService(repo repository.FavoriteRepository, postRepo repository.PostRepository) FavoriteService {
	return &favoriteService{
		repo:     repo,
		postRepo: postRepo,
	}
}

func (s *favoriteService) Toggle(ctx context.Context, postID, userID string) (bool, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return false, err
	}
	if post == nil {
		return false, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	return s.repo.Toggle(ctx, postID, userID)
}

func (s *favoriteService) HasFavorited(ctx context.Context, postID, userID string) (bool, error) {
	return s.repo.HasFavorited(ctx, postID, userID)
}

func (s *favoriteService) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]string, int, error) {
	offset := (page - 1) * pageSize
	items, err := s.repo.ListByUser(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return items, int(total), nil
}

func (s *favoriteService) CountByUser(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountByUser(ctx, userID)
}
