package service

import (
	"context"
	"errors"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"gorm.io/gorm"
)

type BannedService interface {
	Ban(ctx context.Context, userID, reason, createdBy string, isPermanent bool, duration *time.Duration) (*model.Banned, error)
	Unban(ctx context.Context, userID string) error
	GetByUserID(ctx context.Context, userID string) (*model.Banned, error)
	List(ctx context.Context, page, pageSize int) ([]model.Banned, error)
	IsBanned(ctx context.Context, userID string) bool
}

type bannedService struct {
	bannedRepo repository.BannedRepository
}

func NewBannedService(bannedRepo repository.BannedRepository) BannedService {
	return &bannedService{bannedRepo: bannedRepo}
}

func (s *bannedService) Ban(ctx context.Context, userID, reason, createdBy string, isPermanent bool, duration *time.Duration) (*model.Banned, error) {
	now := time.Now()

	var expiresAt *time.Time
	if !isPermanent && duration != nil {
		exp := now.Add(*duration)
		expiresAt = &exp
	}

	// 检查是否已有禁言记录
	existing, err := s.bannedRepo.FindByUserID(ctx, userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if existing != nil && existing.UserID != "" {
		// 更新已有记录
		existing.Reason = reason
		existing.IsPermanent = isPermanent
		existing.ExpiresAt = expiresAt
		existing.CreatedBy = createdBy
		existing.UpdatedAt = now
		if err := s.bannedRepo.Update(ctx, existing); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return existing, nil
	}

	banned := &model.Banned{
		ID:          pkg.NewID(),
		UserID:      userID,
		Reason:      reason,
		IsPermanent: isPermanent,
		ExpiresAt:   expiresAt,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.bannedRepo.Create(ctx, banned); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return banned, nil
}

func (s *bannedService) Unban(ctx context.Context, userID string) error {
	_, err := s.bannedRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "banned record not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.bannedRepo.Delete(ctx, userID)
}

func (s *bannedService) GetByUserID(ctx context.Context, userID string) (*model.Banned, error) {
	banned, err := s.bannedRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "banned record not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !banned.IsPermanent && banned.ExpiresAt != nil && banned.ExpiresAt.Before(time.Now()) {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "banned record not found")
	}
	return banned, nil
}

func (s *bannedService) List(ctx context.Context, page, pageSize int) ([]model.Banned, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	bans, err := s.bannedRepo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return bans, nil
}

func (s *bannedService) IsBanned(ctx context.Context, userID string) bool {
	banned, err := s.bannedRepo.FindByUserID(ctx, userID)
	if err != nil || banned == nil {
		return false
	}
	if banned.IsPermanent {
		return true
	}
	if banned.ExpiresAt != nil && banned.ExpiresAt.After(time.Now()) {
		return true
	}
	return false
}
