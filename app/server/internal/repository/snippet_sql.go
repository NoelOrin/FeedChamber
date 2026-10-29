package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlSnippetRepo struct{ db db.SQLStore }

func (r *sqlSnippetRepo) Create(ctx context.Context, s *model.Snippet) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(s).Error)
}

func (r *sqlSnippetRepo) Update(ctx context.Context, s *model.Snippet) error {
	res := r.db.DB().WithContext(ctx).Model(&model.Snippet{}).Where("id = ?", s.ID).Updates(map[string]any{
		"kind":       s.Kind,
		"position":   s.Position,
		"content":    s.Content,
		"enabled":    s.Enabled,
		"sort":       s.Sort,
		"updated_by": s.UpdatedBy,
		"updated_at": s.UpdatedAt,
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlSnippetRepo) Get(ctx context.Context, id string) (*model.Snippet, error) {
	var s model.Snippet
	return &s, WrapDBErr(r.db.DB().WithContext(ctx).First(&s, "id = ?", id).Error)
}

func (r *sqlSnippetRepo) List(ctx context.Context) ([]model.Snippet, error) {
	var items []model.Snippet
	err := r.db.DB().WithContext(ctx).Order("sort ASC, created_at ASC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlSnippetRepo) ListEnabled(ctx context.Context, position string) ([]model.Snippet, error) {
	var items []model.Snippet
	err := r.db.DB().WithContext(ctx).
		Where("enabled = ? AND position = ?", true, position).
		Order("sort ASC, created_at ASC").
		Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlSnippetRepo) Delete(ctx context.Context, id string) error {
	res := r.db.DB().WithContext(ctx).Delete(&model.Snippet{}, "id = ?", id)
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlSnippetRepo) SetEnabled(ctx context.Context, id string, enabled bool, updatedBy string) error {
	res := r.db.DB().WithContext(ctx).Model(&model.Snippet{}).Where("id = ?", id).Updates(map[string]any{
		"enabled":    enabled,
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
