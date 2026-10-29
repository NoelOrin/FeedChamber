package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/repository"
)

const (
	banGlobalCacheTTL = 30 * time.Second
	banSpaceCacheTTL  = 30 * time.Second
)

type bannedService struct {
	bannedRepo repository.BannedRepository
	kv         kvstore.Store
	txMgr      db.TransactionManager
}

func NewBannedService(bannedRepo repository.BannedRepository, kv kvstore.Store, txMgr db.TransactionManager) BannedService {
	return &bannedService{bannedRepo: bannedRepo, kv: kv, txMgr: txMgr}
}

func banGlobalCacheKey(userID string) string         { return "ban:" + userID }
func banSpaceCacheKey(userID, spaceID string) string { return "ban:" + userID + ":" + spaceID }

func (s *bannedService) cacheBanned(ctx context.Context, key string, banned *model.Banned, ttl time.Duration) {
	b, err := json.Marshal(banned)
	if err == nil {
		_ = s.kv.Set(ctx, key, string(b), ttl)
	}
}

func (s *bannedService) getCachedBanned(ctx context.Context, key string) (*model.Banned, bool) {
	val, err := s.kv.Get(ctx, key)
	if err != nil {
		return nil, false
	}
	var b model.Banned
	if err := json.Unmarshal([]byte(val), &b); err != nil {
		return nil, false
	}
	return &b, true
}

func (s *bannedService) Ban(ctx context.Context, userID, reason, createdBy string, isPermanent bool, durationMinutes *int) (*model.Banned, error) {
	if reason == "" {
		reason = "未提供原因"
	}
	if !isPermanent && (durationMinutes == nil || *durationMinutes <= 0) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "临时禁言必须指定有效时长")
	}
	now := time.Now()

	var expiresAt *time.Time
	if !isPermanent && durationMinutes != nil && *durationMinutes > 0 {
		dur := time.Duration(*durationMinutes) * time.Minute
		exp := now.Add(dur)
		expiresAt = &exp
	}

	var result *model.Banned
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		existing, err := s.bannedRepo.FindByUserIDAndScope(txCtx, userID, "global", "")
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}

		if existing != nil && existing.UserID != "" {
			existing.Reason = reason
			existing.IsPermanent = isPermanent
			existing.ExpiresAt = expiresAt
			existing.CreatedBy = createdBy
			existing.UpdatedAt = now
			if err := s.bannedRepo.Update(txCtx, existing); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			result = existing
			return nil
		}

		banned := model.NewBanned(userID, "global", "", reason, createdBy, isPermanent, expiresAt)
		if err := s.bannedRepo.Create(txCtx, banned); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		result = banned
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.kv.Delete(ctx, banGlobalCacheKey(userID))
	return result, nil
}

func (s *bannedService) BanInSpace(ctx context.Context, userID, spaceID, reason, createdBy string, isPermanent bool, durationMinutes *int) (*model.Banned, error) {
	if reason == "" {
		reason = "未提供原因"
	}
	if !isPermanent && (durationMinutes == nil || *durationMinutes <= 0) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "临时禁言必须指定有效时长")
	}
	now := time.Now()

	var expiresAt *time.Time
	if !isPermanent && durationMinutes != nil && *durationMinutes > 0 {
		dur := time.Duration(*durationMinutes) * time.Minute
		exp := now.Add(dur)
		expiresAt = &exp
	}

	var result *model.Banned
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		existing, err := s.bannedRepo.FindByUserIDAndScope(txCtx, userID, "space", spaceID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}

		if existing != nil && existing.UserID != "" {
			existing.Reason = reason
			existing.IsPermanent = isPermanent
			existing.ExpiresAt = expiresAt
			existing.CreatedBy = createdBy
			existing.UpdatedAt = now
			if err := s.bannedRepo.Update(txCtx, existing); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			result = existing
			return nil
		}

		banned := model.NewBanned(userID, "space", spaceID, reason, createdBy, isPermanent, expiresAt)
		if err := s.bannedRepo.Create(txCtx, banned); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		result = banned
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.kv.Delete(ctx, banSpaceCacheKey(userID, spaceID))
	return result, nil
}

func (s *bannedService) Unban(ctx context.Context, userID string) error {
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		_, err := s.bannedRepo.FindByUserIDAndScope(txCtx, userID, "global", "")
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "全局封禁记录不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return s.bannedRepo.DeleteByScope(txCtx, userID, "global", "")
	})
	if err != nil {
		return err
	}
	_ = s.kv.Delete(ctx, banGlobalCacheKey(userID))
	return nil
}

func (s *bannedService) UnbanInSpace(ctx context.Context, userID, spaceID string) error {
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		_, err := s.bannedRepo.FindByUserIDAndScope(txCtx, userID, "space", spaceID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "版面封禁记录不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return s.bannedRepo.DeleteByScope(txCtx, userID, "space", spaceID)
	})
	if err != nil {
		return err
	}
	_ = s.kv.Delete(ctx, banSpaceCacheKey(userID, spaceID))
	return nil
}

func (s *bannedService) GetByUserID(ctx context.Context, userID string) (*model.Banned, error) {
	banned, err := s.bannedRepo.FindByUserIDAndScope(ctx, userID, "global", "")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "封禁记录不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !banned.IsValid() {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "封禁记录不存在")
	}
	return banned, nil
}

func (s *bannedService) GetInSpace(ctx context.Context, userID, spaceID string) (*model.Banned, error) {
	banned, err := s.bannedRepo.FindByUserIDAndScope(ctx, userID, "space", spaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "封禁记录不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !banned.IsValid() {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "封禁记录不存在")
	}
	return banned, nil
}

func (s *bannedService) IsBanned(ctx context.Context, userID string) bool {
	cacheKey := banGlobalCacheKey(userID)
	if cached, ok := s.getCachedBanned(ctx, cacheKey); ok {
		return cached.IsValid()
	}
	banned, err := s.bannedRepo.FindByUserIDAndScope(ctx, userID, "global", "")
	if err != nil || banned == nil {
		return false
	}
	s.cacheBanned(ctx, cacheKey, banned, banGlobalCacheTTL)
	return banned.IsValid()
}

func (s *bannedService) IsBannedInSpace(ctx context.Context, userID, spaceID string) bool {
	if s.IsBanned(ctx, userID) {
		return true
	}
	cacheKey := banSpaceCacheKey(userID, spaceID)
	if cached, ok := s.getCachedBanned(ctx, cacheKey); ok {
		return cached.IsValid()
	}
	banned, err := s.bannedRepo.FindByUserIDAndScope(ctx, userID, "space", spaceID)
	if err != nil || banned == nil {
		return false
	}
	s.cacheBanned(ctx, cacheKey, banned, banSpaceCacheTTL)
	return banned.IsValid()
}

func (s *bannedService) List(ctx context.Context, page, pageSize int) ([]model.Banned, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	bans, err := s.bannedRepo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return bans, nil
}

func (s *bannedService) ListByScope(ctx context.Context, scope, spaceID string, page, pageSize int) ([]model.Banned, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	bans, err := s.bannedRepo.ListByScope(ctx, scope, spaceID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return bans, nil
}

func (s *bannedService) ListByUserID(ctx context.Context, userID string) ([]model.Banned, error) {
	bans, err := s.bannedRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return bans, nil
}
