package service

import (
	"context"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type userBlockService struct {
	repo     repository.UserBlockRepository
	userRepo repository.UserRepository
}

func NewUserBlockService(repo repository.UserBlockRepository, userRepo repository.UserRepository) UserBlockService {
	return &userBlockService{repo: repo, userRepo: userRepo}
}

func (s *userBlockService) Block(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == "" || blockedID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "用户无效")
	}
	if blockerID == blockedID {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "不能屏蔽自己")
	}
	if _, err := s.userRepo.FindByID(ctx, blockedID); err != nil {
		if err == repository.ErrNotFound {
			return pkg.NewAppError(pkg.NOT_FOUND, "用户不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	b := &model.UserBlock{
		ID:        pkg.NewID(),
		BlockerID: blockerID,
		BlockedID: blockedID,
		CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, b); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *userBlockService) Unblock(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == "" || blockedID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "用户无效")
	}
	if err := s.repo.Delete(ctx, blockerID, blockedID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *userBlockService) IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error) {
	ok, err := s.repo.Exists(ctx, blockerID, blockedID)
	if err != nil {
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return ok, nil
}

func (s *userBlockService) EitherBlocked(ctx context.Context, a, b string) (bool, error) {
	ok, err := s.repo.EitherBlocked(ctx, a, b)
	if err != nil {
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return ok, nil
}

func (s *userBlockService) ListBlockedIDs(ctx context.Context, blockerID string, page, pageSize int) ([]string, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	ids, err := s.repo.ListBlockedIDs(ctx, blockerID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	total, err := s.repo.CountBlocked(ctx, blockerID)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return ids, int(total), nil
}
