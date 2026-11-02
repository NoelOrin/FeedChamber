package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlCustomPageRepo struct{ db db.SQLStore }

func (r *sqlCustomPageRepo) Create(ctx context.Context, p *model.CustomPage) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(p).Error)
}

func (r *sqlCustomPageRepo) Update(ctx context.Context, p *model.CustomPage) error {
	res := r.db.DB().WithContext(ctx).Model(&model.CustomPage{}).Where("id = ?", p.ID).Updates(map[string]any{
		"slug":       p.Slug,
		"title":      p.Title,
		"content":    p.Content,
		"status":     p.Status,
		"updated_by": p.UpdatedBy,
		"updated_at": p.UpdatedAt,
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlCustomPageRepo) Get(ctx context.Context, id string) (*model.CustomPage, error) {
	var p model.CustomPage
	return &p, WrapDBErr(r.db.DB().WithContext(ctx).First(&p, "id = ?", id).Error)
}

func (r *sqlCustomPageRepo) GetBySlug(ctx context.Context, slug string) (*model.CustomPage, error) {
	var p model.CustomPage
	return &p, WrapDBErr(r.db.DB().WithContext(ctx).First(&p, "slug = ?", slug).Error)
}

func (r *sqlCustomPageRepo) List(ctx context.Context, offset, limit int) ([]model.CustomPage, error) {
	var items []model.CustomPage
	err := r.db.DB().WithContext(ctx).
		Order("updated_at DESC").
		Offset(offset).Limit(limit).
		Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlCustomPageRepo) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.DB().WithContext(ctx).Model(&model.CustomPage{}).Count(&total).Error
	return total, WrapDBErr(err)
}

func (r *sqlCustomPageRepo) ListPublished(ctx context.Context) ([]model.CustomPage, error) {
	var items []model.CustomPage
	err := r.db.DB().WithContext(ctx).
		Where("status = ?", model.CustomPagePublished).
		Order("updated_at DESC").
		Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlCustomPageRepo) Delete(ctx context.Context, id string) error {
	res := r.db.DB().WithContext(ctx).Delete(&model.CustomPage{}, "id = ?", id)
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlCustomPageRepo) SetStatus(ctx context.Context, id, status, updatedBy string) error {
	res := r.db.DB().WithContext(ctx).Model(&model.CustomPage{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_by": updatedBy,
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
