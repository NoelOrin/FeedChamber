package service

import (
	"context"
	"time"

	"orinforum/internal/model"
)

// CreateInviteRequest 创建邀请码请求。
type CreateInviteRequest struct {
	MaxUses   int        `json:"max_uses"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// Code 可选；为空时由服务端生成。
	Code string `json:"code,omitempty"`
}

// InviteService 邀请码业务接口。
type InviteService interface {
	Create(ctx context.Context, creatorID string, req *CreateInviteRequest) (*model.Invite, error)
	List(ctx context.Context, page, pageSize int) ([]model.Invite, int64, error)
	// ConsumeValid 校验邀请码可用并占用一次（UsedCount++）。
	ConsumeValid(ctx context.Context, code string) error
	Delete(ctx context.Context, id string) error
}
