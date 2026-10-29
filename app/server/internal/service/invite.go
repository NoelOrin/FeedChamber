package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type inviteService struct {
	repo repository.InviteRepository
}

func NewInviteService(repo repository.InviteRepository) InviteService {
	return &inviteService{repo: repo}
}

func (s *inviteService) Create(ctx context.Context, creatorID string, req *CreateInviteRequest) (*model.Invite, error) {
	if req == nil {
		req = &CreateInviteRequest{}
	}
	if req.MaxUses < 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "max_uses 不能为负数")
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "过期时间不能早于当前时间")
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		generated, err := generateInviteCode(8)
		if err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		code = generated
	}
	if len(code) < 4 || len(code) > 32 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "邀请码长度须为 4-32 位")
	}

	// 冲突检查
	if existing, err := s.repo.FindByCode(ctx, code); err == nil && existing != nil {
		return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "邀请码已存在")
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	inv := model.NewInvite(code, creatorID, req.MaxUses, req.ExpiresAt)
	if err := s.repo.Create(ctx, inv); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate") {
			return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "邀请码已存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return inv, nil
}

func (s *inviteService) List(ctx context.Context, page, pageSize int) ([]model.Invite, int64, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	items, total, err := s.repo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return items, total, nil
}

func (s *inviteService) ConsumeValid(ctx context.Context, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "请填写邀请码")
	}
	if err := s.repo.ConsumeByCode(ctx, code); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return pkg.NewAppError(pkg.INVALID_PARAMS, "邀请码无效")
		case errors.Is(err, repository.ErrInviteExpired):
			return pkg.NewAppError(pkg.INVALID_PARAMS, "邀请码已过期")
		case errors.Is(err, repository.ErrInviteExhausted):
			return pkg.NewAppError(pkg.INVALID_PARAMS, "邀请码已达使用上限")
		default:
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

func (s *inviteService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "邀请码不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func generateInviteCode(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
