package service

import (
	"context"
	"errors"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/sirupsen/logrus"
)

type favoriteService struct {
	repo     repository.FavoriteRepository
	postRepo repository.PostLookupRepository
	trustSvc TrustService
}

// SetTrustService 注入等级评估服务（改变被收藏指标后触发重算）。
func (s *favoriteService) SetTrustService(svc TrustService) { s.trustSvc = svc }

func NewFavoriteService(repo repository.FavoriteRepository, postRepo repository.PostLookupRepository) FavoriteService {
	return &favoriteService{
		repo:     repo,
		postRepo: postRepo,
	}
}

func (s *favoriteService) Toggle(ctx context.Context, postID, userID string) (bool, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	favorited, err := s.repo.Toggle(ctx, postID, userID)
	if err != nil {
		return false, err
	}
	// G-02: 同步更新帖子的 favorite_count 计数器
	if favorited {
		if err := s.postRepo.IncrementFavoriteCount(ctx, postID); err != nil {
			logrus.Errorf("favorite: increment favorite_count drift for post %s: %v", postID, err)
		}
	} else {
		if err := s.postRepo.DecrementFavoriteCount(ctx, postID); err != nil {
			logrus.Errorf("favorite: decrement favorite_count drift for post %s: %v", postID, err)
		}
	}
	// 收藏 / 取消收藏都会改变作者的「被收藏」指标。
	if s.trustSvc != nil && post != nil {
		if _, err := s.trustSvc.Recompute(ctx, post.AuthorID); err != nil {
			logrus.Warnf("trust: recompute %s after favorite toggle: %v", post.AuthorID, err)
		}
	}
	return favorited, nil
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
