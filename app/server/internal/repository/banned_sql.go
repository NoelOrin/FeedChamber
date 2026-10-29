package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlBannedRepo struct{ db db.SQLStore }

func (r *sqlBannedRepo) FindByUserID(ctx context.Context, userID string) (*model.Banned, error) {
	var b model.Banned
	return &b, WrapDBErr(r.db.DB().WithContext(ctx).Where("user_id = ?", userID).First(&b).Error)
}

func (r *sqlBannedRepo) FindByUserIDAndScope(ctx context.Context, userID, scope, spaceID string) (*model.Banned, error) {
	var b model.Banned
	query := r.db.DB().WithContext(ctx).Where("user_id = ? AND scope = ?", userID, scope)

	if scope == "space" {
		if spaceID == "" {
			return nil, nil
		}
		query = query.Where("space_id = ?", spaceID)
	} else {
		query = query.Where("space_id IS NULL OR space_id = ''")
	}

	return &b, WrapDBErr(query.First(&b).Error)
}

func (r *sqlBannedRepo) Create(ctx context.Context, banned *model.Banned) error {
	return r.db.DB().WithContext(ctx).Create(banned).Error
}

func (r *sqlBannedRepo) Update(ctx context.Context, banned *model.Banned) error {
	return r.db.DB().WithContext(ctx).Where(SQLSoftDeleteCondition()).Save(banned).Error
}

func (r *sqlBannedRepo) Delete(ctx context.Context, userID string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Banned{}, "user_id = ?", userID).Error
}

func (r *sqlBannedRepo) DeleteByScope(ctx context.Context, userID, scope, spaceID string) error {
	query := r.db.DB().WithContext(ctx).Where("user_id = ? AND scope = ?", userID, scope)

	if scope == "space" {
		query = query.Where("space_id = ?", spaceID)
	} else {
		query = query.Where("space_id IS NULL OR space_id = ''")
	}

	return query.Delete(&model.Banned{}).Error
}

func (r *sqlBannedRepo) List(ctx context.Context, offset, limit int) ([]model.Banned, error) {
	var bans []model.Banned
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&bans).Error
	return bans, err
}

func (r *sqlBannedRepo) ListByScope(ctx context.Context, scope, spaceID string, offset, limit int) ([]model.Banned, error) {
	var bans []model.Banned
	query := r.db.DB().WithContext(ctx).Where("scope = ?", scope).Order("created_at desc")

	if scope == "space" && spaceID != "" {
		query = query.Where("space_id = ?", spaceID)
	} else if scope == "global" {
		query = query.Where("space_id IS NULL OR space_id = ''")
	}

	err := query.Offset(offset).Limit(limit).Find(&bans).Error
	return bans, err
}

func (r *sqlBannedRepo) ListByUserID(ctx context.Context, userID string) ([]model.Banned, error) {
	var bans []model.Banned
	err := r.db.DB().WithContext(ctx).Where("user_id = ?", userID).Order("scope, space_id").Find(&bans).Error
	return bans, err
}
