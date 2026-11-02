package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// ThemeTokenRepository 负责主题 token 覆盖的持久化。
type ThemeTokenRepository interface {
	// Upsert 按 (token, mode) 写入覆盖：同一 token 的明暗两套覆盖各自一行，
	// 同名同模式只更新值，不堆行。
	Upsert(ctx context.Context, t *model.ThemeToken) error
	// DeleteByTokenMode 只删某一模式的覆盖；Mode 为空表示删该 token 的全部模式。
	DeleteByTokenMode(ctx context.Context, token, mode string) error
	GetByToken(ctx context.Context, token string) (*model.ThemeToken, error)
	List(ctx context.Context) ([]model.ThemeToken, error)
	DeleteByToken(ctx context.Context, token string) error
	DeleteAll(ctx context.Context) error
}

func NewThemeTokenRepository(store db.Store) ThemeTokenRepository {
	return &sqlThemeTokenRepo{db: store.SQL()}
}
