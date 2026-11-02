package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlHomeModuleRepo struct{ db db.SQLStore }

func (r *sqlHomeModuleRepo) Create(ctx context.Context, m *model.HomeModule) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(m).Error)
}

func (r *sqlHomeModuleRepo) Update(ctx context.Context, m *model.HomeModule) error {
	res := r.db.DB().WithContext(ctx).Model(&model.HomeModule{}).Where("id = ?", m.ID).Updates(map[string]any{
		"kind":       m.Kind,
		"title":      m.Title,
		"sort":       m.Sort,
		"visibility": m.Visibility,
		"min_level":  m.MinLevel,
		"config":     m.Config,
		"enabled":    m.Enabled,
		"updated_by": m.UpdatedBy,
		"updated_at": m.UpdatedAt,
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlHomeModuleRepo) Get(ctx context.Context, id string) (*model.HomeModule, error) {
	var m model.HomeModule
	return &m, WrapDBErr(r.db.DB().WithContext(ctx).First(&m, "id = ?", id).Error)
}

func (r *sqlHomeModuleRepo) List(ctx context.Context) ([]model.HomeModule, error) {
	var items []model.HomeModule
	err := r.db.DB().WithContext(ctx).Order("sort ASC, created_at ASC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlHomeModuleRepo) ListEnabled(ctx context.Context) ([]model.HomeModule, error) {
	var items []model.HomeModule
	err := r.db.DB().WithContext(ctx).
		Where("enabled = ?", true).
		Order("sort ASC, created_at ASC").
		Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlHomeModuleRepo) Delete(ctx context.Context, id string) error {
	res := r.db.DB().WithContext(ctx).Delete(&model.HomeModule{}, "id = ?", id)
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
