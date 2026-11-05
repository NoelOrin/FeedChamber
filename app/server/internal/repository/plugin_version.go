package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// PluginVersionRepository 是插件安装/升级历史。
type PluginVersionRepository interface {
	Create(ctx context.Context, v *model.PluginVersion) error
	ListByPlugin(ctx context.Context, pluginID string) ([]model.PluginVersion, error)
	Get(ctx context.Context, id string) (*model.PluginVersion, error)
	// DeleteByPlugin 在卸载时清理该插件的全部历史。
	DeleteByPlugin(ctx context.Context, pluginID string) error
}

func NewPluginVersionRepository(store db.Store) PluginVersionRepository {
	return &sqlPluginVersionRepo{db: store.SQL()}
}
