package repository

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlBadgeRepo struct{ db db.SQLStore }

func (r *sqlBadgeRepo) ListBadges(ctx context.Context) ([]model.Badge, error) {
	var items []model.Badge
	err := r.db.DB().WithContext(ctx).Order("created_at asc").Find(&items).Error
	return items, err
}

func (r *sqlBadgeRepo) GetByCode(ctx context.Context, code string) (*model.Badge, error) {
	var b model.Badge
	err := WrapDBErr(r.db.DB().WithContext(ctx).First(&b, "code = ?", code).Error)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *sqlBadgeRepo) UpsertBadge(ctx context.Context, b *model.Badge) error {
	var existing model.Badge
	err := WrapDBErr(r.db.DB().WithContext(ctx).First(&existing, "code = ?", b.Code).Error)
	if err == nil {
		existing.Name = b.Name
		existing.Description = b.Description
		existing.Icon = b.Icon
		return r.db.DB().WithContext(ctx).Save(&existing).Error
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.db.DB().WithContext(ctx).Create(b).Error
}

func (r *sqlBadgeRepo) Award(ctx context.Context, ub *model.UserBadge) error {
	var existing model.UserBadge
	err := WrapDBErr(r.db.DB().WithContext(ctx).First(&existing, "user_id = ? AND badge_id = ?", ub.UserID, ub.BadgeID).Error)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.db.DB().WithContext(ctx).Create(ub).Error
}

func (r *sqlBadgeRepo) HasBadge(ctx context.Context, userID, badgeID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.UserBadge{}).Where("user_id = ? AND badge_id = ?", userID, badgeID).Count(&count).Error
	return count > 0, err
}

func (r *sqlBadgeRepo) ListUserBadges(ctx context.Context, userID string) ([]model.Badge, error) {
	var items []model.Badge
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT b.* FROM badges b
		INNER JOIN user_badges ub ON ub.badge_id = b.id
		WHERE ub.user_id = ?
		ORDER BY ub.awarded_at desc
	`, userID).Scan(&items).Error
	return items, err
}
