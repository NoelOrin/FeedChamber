package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlPluginVersionRepo struct{ db db.SQLStore }

func (r *sqlPluginVersionRepo) Create(ctx context.Context, v *model.PluginVersion) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(v).Error)
}

func (r *sqlPluginVersionRepo) ListByPlugin(ctx context.Context, pluginID string) ([]model.PluginVersion, error) {
	var items []model.PluginVersion
	err := r.db.DB().WithContext(ctx).
		Where("plugin_id = ?", pluginID).
		Order("created_at DESC").Find(&items).Error
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return items, nil
}

func (r *sqlPluginVersionRepo) Get(ctx context.Context, id string) (*model.PluginVersion, error) {
	var v model.PluginVersion
	err := r.db.DB().WithContext(ctx).Where("id = ?", id).First(&v).Error
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return &v, nil
}

func (r *sqlPluginVersionRepo) DeleteByPlugin(ctx context.Context, pluginID string) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).
		Where("plugin_id = ?", pluginID).Delete(&model.PluginVersion{}).Error)
}
