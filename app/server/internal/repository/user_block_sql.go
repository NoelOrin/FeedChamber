package repository

import (
	"context"
	"errors"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

type sqlUserBlockRepo struct{ db db.SQLStore }

func (r *sqlUserBlockRepo) Create(ctx context.Context, b *model.UserBlock) error {
	var existing model.UserBlock
	err := WrapDBErr(r.db.DB().WithContext(ctx).
		First(&existing, "blocker_id = ? AND blocked_id = ?", b.BlockerID, b.BlockedID).Error)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.db.DB().WithContext(ctx).Create(b).Error
}

func (r *sqlUserBlockRepo) Delete(ctx context.Context, blockerID, blockedID string) error {
	return r.db.DB().WithContext(ctx).
		Where("blocker_id = ? AND blocked_id = ?", blockerID, blockedID).
		Delete(&model.UserBlock{}).Error
}

func (r *sqlUserBlockRepo) Exists(ctx context.Context, blockerID, blockedID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.UserBlock{}).
		Where("blocker_id = ? AND blocked_id = ?", blockerID, blockedID).
		Count(&count).Error
	return count > 0, err
}

func (r *sqlUserBlockRepo) EitherBlocked(ctx context.Context, a, b string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.UserBlock{}).
		Where("(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)", a, b, b, a).
		Count(&count).Error
	return count > 0, err
}

func (r *sqlUserBlockRepo) ListBlockedIDs(ctx context.Context, blockerID string, offset, limit int) ([]string, error) {
	var ids []string
	err := r.db.DB().WithContext(ctx).Model(&model.UserBlock{}).
		Where("blocker_id = ?", blockerID).
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Pluck("blocked_id", &ids).Error
	return ids, err
}

func (r *sqlUserBlockRepo) CountBlocked(ctx context.Context, blockerID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.UserBlock{}).
		Where("blocker_id = ?", blockerID).Count(&count).Error
	return count, err
}
