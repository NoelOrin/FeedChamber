package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"

	"gorm.io/gorm/clause"
)

// ── SQL ─────────────────────────────────────────────────────────────────────

type sqlFavRepo struct{ db db.SQLStore }

func (r *sqlFavRepo) Toggle(ctx context.Context, postID, userID string) (bool, error) {
	// delete-then-insert: 原子操作，消除 check-then-act TOCTOU 窗口
	result := r.db.DB().WithContext(ctx).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Delete(&model.PostFavorite{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return false, nil // 已收藏 → 取消收藏
	}

	// 未找到 → 创建
	fav := model.PostFavorite{
		ID:        pkg.NewID(),
		PostID:    postID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := r.db.DB().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&fav).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *sqlFavRepo) HasFavorited(ctx context.Context, postID, userID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.PostFavorite{}).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *sqlFavRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]string, error) {
	var ids []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.PostFavorite{}).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Pluck("post_id", &ids).Error
	return ids, err
}

func (r *sqlFavRepo) CountByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.PostFavorite{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}
