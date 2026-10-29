package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlMCPServerRepo struct{ db db.SQLStore }

func (r *sqlMCPServerRepo) Create(ctx context.Context, s *model.MCPServer) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(s).Error)
}

func (r *sqlMCPServerRepo) Update(ctx context.Context, s *model.MCPServer) error {
	// headers 只在显式非空时覆盖：接口层返回的是脱敏值，若调用方回传脱敏值
	// 会把真实凭据冲掉。空串表示「保持原样」。
	updates := map[string]any{
		"name":            s.Name,
		"endpoint":        s.Endpoint,
		"transport":       s.Transport,
		"purpose":         s.Purpose,
		"enabled":         s.Enabled,
		"moderation_tool": s.ModerationTool,
		"updated_by":      s.UpdatedBy,
		"updated_at":      s.UpdatedAt,
	}
	if s.Headers != "" {
		updates["headers"] = s.Headers
	}
	res := r.db.DB().WithContext(ctx).Model(&model.MCPServer{}).Where("id = ?", s.ID).Updates(updates)
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlMCPServerRepo) Get(ctx context.Context, id string) (*model.MCPServer, error) {
	var s model.MCPServer
	return &s, WrapDBErr(r.db.DB().WithContext(ctx).First(&s, "id = ?", id).Error)
}

func (r *sqlMCPServerRepo) List(ctx context.Context) ([]model.MCPServer, error) {
	var items []model.MCPServer
	err := r.db.DB().WithContext(ctx).Order("created_at ASC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlMCPServerRepo) ListEnabledByPurpose(ctx context.Context, purpose string) ([]model.MCPServer, error) {
	var items []model.MCPServer
	err := r.db.DB().WithContext(ctx).
		Where("enabled = ? AND purpose = ?", true, purpose).
		Order("created_at ASC").
		Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlMCPServerRepo) Delete(ctx context.Context, id string) error {
	res := r.db.DB().WithContext(ctx).Delete(&model.MCPServer{}, "id = ?", id)
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
