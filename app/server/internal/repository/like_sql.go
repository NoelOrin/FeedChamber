package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"

	"gorm.io/gorm/clause"
)

// ── SQL ──────────────────────────────────────────────────────────────────────

type sqlLikeRepo struct{ db db.SQLStore }

func (r *sqlLikeRepo) ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error) {
	// delete-then-insert: 原子操作，消除 check-then-act TOCTOU 窗口
	result := r.db.DB().WithContext(ctx).
		Where("content_id = ? AND user_id = ?", contentID, userID).
		Delete(&model.ContentLike{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return false, nil // 已点赞 → 取消点赞
	}

	// 未找到 → 创建（未点赞 → 点赞）
	like := model.ContentLike{ID: pkg.NewID(), ContentID: contentID, UserID: userID, CreatedAt: time.Now()}
	if err := r.db.DB().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&like).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *sqlLikeRepo) HasContentLiked(ctx context.Context, contentID, userID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.ContentLike{}).Where("content_id = ? AND user_id = ?", contentID, userID).Count(&count).Error
	return count > 0, err
}

func (r *sqlLikeRepo) CountContentLikes(ctx context.Context, contentID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.ContentLike{}).Where("content_id = ?", contentID).Count(&count).Error
	return count, err
}
