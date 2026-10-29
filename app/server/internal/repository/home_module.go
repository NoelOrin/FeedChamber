package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// HomeModuleRepository 负责首页模块配置的持久化。
type HomeModuleRepository interface {
	Create(ctx context.Context, m *model.HomeModule) error
	Update(ctx context.Context, m *model.HomeModule) error
	Get(ctx context.Context, id string) (*model.HomeModule, error)
	List(ctx context.Context) ([]model.HomeModule, error)
	ListEnabled(ctx context.Context) ([]model.HomeModule, error)
	Delete(ctx context.Context, id string) error
}

func NewHomeModuleRepository(store db.Store) HomeModuleRepository {
	return &sqlHomeModuleRepo{db: store.SQL()}
}
