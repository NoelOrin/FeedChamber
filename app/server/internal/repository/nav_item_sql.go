package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlNavItemRepo struct{ db db.SQLStore }

func (r *sqlNavItemRepo) Create(ctx context.Context, n *model.NavItem) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(n).Error)
}

func (r *sqlNavItemRepo) Update(ctx context.Context, n *model.NavItem) error {
	res := r.db.DB().WithContext(ctx).Model(&model.NavItem{}).Where("id = ?", n.ID).Updates(map[string]any{
		"label":      n.Label,
		"href":       n.Href,
		"icon":       n.Icon,
		"sort":       n.Sort,
		"visibility": n.Visibility,
		"min_level":  n.MinLevel,
		"enabled":    n.Enabled,
		"updated_by": n.UpdatedBy,
		"updated_at": n.UpdatedAt,
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlNavItemRepo) Get(ctx context.Context, id string) (*model.NavItem, error) {
	var n model.NavItem
	return &n, WrapDBErr(r.db.DB().WithContext(ctx).First(&n, "id = ?", id).Error)
}

func (r *sqlNavItemRepo) List(ctx context.Context) ([]model.NavItem, error) {
	var items []model.NavItem
	err := r.db.DB().WithContext(ctx).Order("sort ASC, created_at ASC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlNavItemRepo) ListEnabled(ctx context.Context) ([]model.NavItem, error) {
	var items []model.NavItem
	err := r.db.DB().WithContext(ctx).
		Where("enabled = ?", true).
		Order("sort ASC, created_at ASC").
		Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlNavItemRepo) Delete(ctx context.Context, id string) error {
	res := r.db.DB().WithContext(ctx).Delete(&model.NavItem{}, "id = ?", id)
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
