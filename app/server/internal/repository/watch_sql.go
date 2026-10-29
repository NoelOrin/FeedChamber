package repository

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlWatchRepo struct{ db db.SQLStore }

func (r *sqlWatchRepo) Upsert(ctx context.Context, w *model.Watch) error {
	var existing model.Watch
	err := WrapDBErr(r.db.DB().WithContext(ctx).
		First(&existing, "user_id = ? AND target_type = ? AND target_id = ?", w.UserID, w.TargetType, w.TargetID).Error)
	if err == nil {
		existing.Level = w.Level
		w.ID = existing.ID
		w.CreatedAt = existing.CreatedAt
		return r.db.DB().WithContext(ctx).Save(&existing).Error
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.db.DB().WithContext(ctx).Create(w).Error
}

func (r *sqlWatchRepo) Delete(ctx context.Context, userID, targetType, targetID string) error {
	return r.db.DB().WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&model.Watch{}).Error
}

func (r *sqlWatchRepo) Get(ctx context.Context, userID, targetType, targetID string) (*model.Watch, error) {
	var w model.Watch
	err := WrapDBErr(r.db.DB().WithContext(ctx).
		First(&w, "user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).Error)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *sqlWatchRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Watch, error) {
	var items []model.Watch
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *sqlWatchRepo) CountByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Watch{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *sqlWatchRepo) ListActiveUserIDs(ctx context.Context, targetType, targetID string) ([]string, error) {
	var ids []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.Watch{}).
		Where("target_type = ? AND target_id = ? AND level <> ?", targetType, targetID, model.WatchLevelMuted).
		Pluck("user_id", &ids).Error
	return ids, err
}
