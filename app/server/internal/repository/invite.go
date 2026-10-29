package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

// InviteRepository 邀请码数据访问。
type InviteRepository interface {
	Create(ctx context.Context, invite *model.Invite) error
	FindByCode(ctx context.Context, code string) (*model.Invite, error)
	FindByID(ctx context.Context, id string) (*model.Invite, error)
	List(ctx context.Context, offset, limit int) ([]model.Invite, int64, error)
	// ConsumeByCode 原子占用一次邀请码使用次数。
	// 成功返回 nil；不存在返回 ErrNotFound；过期返回 ErrInviteExpired；用尽返回 ErrInviteExhausted。
	ConsumeByCode(ctx context.Context, code string) error
	Delete(ctx context.Context, id string) error
}

func NewInviteRepository(store db.Store) InviteRepository {
	return &sqlInviteRepo{db: store.SQL()}
}
