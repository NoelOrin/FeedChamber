package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlPluginSourceRepo struct{ db db.SQLStore }

func (r *sqlPluginSourceRepo) Create(ctx context.Context, s *model.PluginSource) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(s).Error)
}

func (r *sqlPluginSourceRepo) Update(ctx context.Context, s *model.PluginSource) error {
	res := r.db.DB().WithContext(ctx).Model(&model.PluginSource{}).Where("id = ?", s.ID).Updates(map[string]any{
		"name":        s.Name,
		"kind":        s.Kind,
		"url":         s.URL,
		"status":      s.Status,
		"approved_by": s.ApprovedBy,
		"updated_at":  s.UpdatedAt,
	})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	// 更新 0 行说明 id 不存在。返回 ErrNotFound 而不是 nil：
	// 审批一个不存在的来源若静默成功，审计里会留下一条「已审批」的假记录。
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlPluginSourceRepo) Get(ctx context.Context, id string) (*model.PluginSource, error) {
	var s model.PluginSource
	return &s, WrapDBErr(r.db.DB().WithContext(ctx).First(&s, "id = ?", id).Error)
}

func (r *sqlPluginSourceRepo) List(ctx context.Context) ([]model.PluginSource, error) {
	var items []model.PluginSource
	err := r.db.DB().WithContext(ctx).Order("created_at DESC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlPluginSourceRepo) Delete(ctx context.Context, id string) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Delete(&model.PluginSource{}, "id = ?", id).Error)
}
