package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"time"
)

type sqlModerationRepo struct{ db db.SQLStore }

func (r *sqlModerationRepo) CreateModeration(ctx context.Context, m *model.ContentModeration) error {
	if m.ID == "" {
		m.ID = pkg.NewID()
	}
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	return r.db.DB().WithContext(ctx).Create(m).Error
}

func (r *sqlModerationRepo) GetModeration(ctx context.Context, id string) (*model.ContentModeration, error) {
	var m model.ContentModeration
	return &m, WrapDBErr(r.db.DB().WithContext(ctx).First(&m, "id = ?", id).Error)
}

func (r *sqlModerationRepo) ListModerations(ctx context.Context, status model.ModerationStatus, offset, limit int) ([]model.ContentModeration, int64, error) {
	var items []model.ContentModeration
	var total int64
	q := r.db.DB().WithContext(ctx).Model(&model.ContentModeration{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *sqlModerationRepo) UpdateModerationStatus(ctx context.Context, id string, status model.ModerationStatus, reviewerID string) error {
	now := time.Now()
	return r.db.DB().WithContext(ctx).Model(&model.ContentModeration{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"reviewer_id": reviewerID,
		"reviewed_at": &now,
		"updated_at":  now,
	}).Error
}

func (r *sqlModerationRepo) CreateRule(ctx context.Context, rule *model.ModerationRule) error {
	if rule.ID == "" {
		rule.ID = pkg.NewID()
	}
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	return r.db.DB().WithContext(ctx).Create(rule).Error
}

func (r *sqlModerationRepo) GetRule(ctx context.Context, id string) (*model.ModerationRule, error) {
	var rule model.ModerationRule
	return &rule, WrapDBErr(r.db.DB().WithContext(ctx).First(&rule, "id = ?", id).Error)
}

func (r *sqlModerationRepo) ListRules(ctx context.Context, offset, limit int) ([]model.ModerationRule, int64, error) {
	var items []model.ModerationRule
	var total int64
	if err := r.db.DB().WithContext(ctx).Model(&model.ModerationRule{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.DB().WithContext(ctx).Order("priority DESC, created_at ASC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *sqlModerationRepo) ListEnabledRules(ctx context.Context, scene model.ModerationScene) ([]model.ModerationRule, error) {
	var items []model.ModerationRule
	err := r.db.DB().WithContext(ctx).
		Where("enabled = ? AND scene = ?", true, scene).
		Order("priority desc, created_at asc").
		Find(&items).Error
	return items, err
}

func (r *sqlModerationRepo) UpdateRule(ctx context.Context, rule *model.ModerationRule) error {
	now := time.Now()
	rule.UpdatedAt = now
	return r.db.DB().WithContext(ctx).Model(&model.ModerationRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"name":        rule.Name,
		"scene":       rule.Scene,
		"type":        rule.Type,
		"pattern":     rule.Pattern,
		"action":      rule.Action,
		"priority":    rule.Priority,
		"enabled":     rule.Enabled,
		"description": rule.Description,
		"updated_at":  now,
	}).Error
}

func (r *sqlModerationRepo) DeleteRule(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.ModerationRule{}, "id = ?", id).Error
}

func (r *sqlModerationRepo) GetConfig(ctx context.Context, key string) (*model.ModerationConfig, error) {
	var c model.ModerationConfig
	return &c, WrapDBErr(r.db.DB().WithContext(ctx).First(&c, "key = ?", key).Error)
}

func (r *sqlModerationRepo) SetConfig(ctx context.Context, key, value string) error {
	return r.db.DB().WithContext(ctx).
		Where(model.ModerationConfig{Key: key}).
		Assign(model.ModerationConfig{Value: value}).
		FirstOrCreate(&model.ModerationConfig{}).Error
}

func (r *sqlModerationRepo) ListConfigs(ctx context.Context) ([]model.ModerationConfig, error) {
	var items []model.ModerationConfig
	err := r.db.DB().WithContext(ctx).Find(&items).Error
	return items, err
}
