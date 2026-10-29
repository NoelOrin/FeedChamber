package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlPluginRepo struct{ db db.SQLStore }

func (r *sqlPluginRepo) Create(ctx context.Context, p *model.Plugin) error {
	return r.db.DB().WithContext(ctx).Create(p).Error
}

func (r *sqlPluginRepo) Update(ctx context.Context, p *model.Plugin) error {
	return r.db.DB().WithContext(ctx).Model(&model.Plugin{}).Where("id = ?", p.ID).Updates(map[string]any{
		"name":        p.Name,
		"version":     p.Version,
		"author":      p.Author,
		"description": p.Description,
		"homepage":    p.Homepage,
		"settings":    p.Settings,
		"enabled":     p.Enabled,
		"updated_at":  p.UpdatedAt,
	}).Error
}

func (r *sqlPluginRepo) Get(ctx context.Context, id string) (*model.Plugin, error) {
	var p model.Plugin
	return &p, WrapDBErr(r.db.DB().WithContext(ctx).First(&p, "id = ?", id).Error)
}

func (r *sqlPluginRepo) List(ctx context.Context) ([]model.Plugin, error) {
	var items []model.Plugin
	err := r.db.DB().WithContext(ctx).Order("name ASC").Find(&items).Error
	return items, err
}

func (r *sqlPluginRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Plugin{}, "id = ?", id).Error
}
