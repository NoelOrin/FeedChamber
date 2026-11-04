package repository

import (
	"context"
	"time"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlPluginInstallRepo struct{ db db.SQLStore }

func (r *sqlPluginInstallRepo) Create(ctx context.Context, in *model.PluginInstall) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(in).Error)
}

func (r *sqlPluginInstallRepo) Update(ctx context.Context, in *model.PluginInstall) error {
	res := r.db.DB().WithContext(ctx).Model(&model.PluginInstall{}).Where("plugin_id = ?", in.PluginID).Updates(map[string]any{
		"source_id":       in.SourceID,
		"dir":             in.Dir,
		"entry":           in.Entry,
		"status":          in.Status,
		"caps":            in.Caps,
		"restarts":        in.Restarts,
		"last_error":      in.LastError,
		"last_started_at": in.LastStartedAt,
		"updated_at":      in.UpdatedAt,
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlPluginInstallRepo) GetByPluginID(ctx context.Context, pluginID string) (*model.PluginInstall, error) {
	var in model.PluginInstall
	return &in, WrapDBErr(r.db.DB().WithContext(ctx).First(&in, "plugin_id = ?", pluginID).Error)
}

func (r *sqlPluginInstallRepo) List(ctx context.Context) ([]model.PluginInstall, error) {
	var items []model.PluginInstall
	err := r.db.DB().WithContext(ctx).Order("created_at ASC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlPluginInstallRepo) ListByStatus(ctx context.Context, status string) ([]model.PluginInstall, error) {
	var items []model.PluginInstall
	err := r.db.DB().WithContext(ctx).Where("status = ?", status).Order("created_at ASC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlPluginInstallRepo) UpdateStatus(ctx context.Context, pluginID, status, lastError string) error {
	res := r.db.DB().WithContext(ctx).Model(&model.PluginInstall{}).Where("plugin_id = ?", pluginID).Updates(map[string]any{
		"status":     status,
		"last_error": lastError,
		"updated_at": time.Now(),
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlPluginInstallRepo) Delete(ctx context.Context, pluginID string) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Delete(&model.PluginInstall{}, "plugin_id = ?", pluginID).Error)
}
