package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// ThemeTokenRepository 负责主题 token 覆盖的持久化。
type ThemeTokenRepository interface {
	// Upsert 按 token 名写入覆盖：同名 token 只保留一条（更新值），
	// 而不是每次保存都堆一行——否则「当前覆盖了哪些 token」会无限膨胀。
	Upsert(ctx context.Context, t *model.ThemeToken) error
	GetByToken(ctx context.Context, token string) (*model.ThemeToken, error)
	List(ctx context.Context) ([]model.ThemeToken, error)
	DeleteByToken(ctx context.Context, token string) error
	DeleteAll(ctx context.Context) error
}

func NewThemeTokenRepository(store db.Store) ThemeTokenRepository {
	return &sqlThemeTokenRepo{db: store.SQL()}
}
