package repository

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"time"
)

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlFollowRepo struct{ db db.SQLStore }

func (r *sqlFollowRepo) Follow(ctx context.Context, followerID, followingID string) error {
	// Check if already following
	var existing model.UserFollow
	err := WrapDBErr(r.db.DB().WithContext(ctx).
		First(&existing, "follower_id = ? AND following_id = ?", followerID, followingID).Error)
	if err == nil {
		return nil // already following, idempotent
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}

	follow := &model.UserFollow{
		ID:          pkg.NewID(),
		FollowerID:  followerID,
		FollowingID: followingID,
		CreatedAt:   time.Now(),
	}
	return r.db.DB().WithContext(ctx).Create(follow).Error
}

func (r *sqlFollowRepo) Unfollow(ctx context.Context, followerID, followingID string) error {
	return r.db.DB().WithContext(ctx).
		Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Delete(&model.UserFollow{}).Error
}

func (r *sqlFollowRepo) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.UserFollow{}).
		Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Count(&count).Error
	return count > 0, err
}

func (r *sqlFollowRepo) GetFollowers(ctx context.Context, userID string, offset, limit int) ([]string, error) {
	var ids []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.UserFollow{}).
		Where("following_id = ?", userID).
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Pluck("follower_id", &ids).Error
	return ids, err
}

func (r *sqlFollowRepo) GetFollowing(ctx context.Context, userID string, offset, limit int) ([]string, error) {
	var ids []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.UserFollow{}).
		Where("follower_id = ?", userID).
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Pluck("following_id", &ids).Error
	return ids, err
}

func (r *sqlFollowRepo) CountFollowers(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.UserFollow{}).
		Where("following_id = ?", userID).
		Count(&count).Error
	return count, err
}

func (r *sqlFollowRepo) CountFollowing(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.UserFollow{}).
		Where("follower_id = ?", userID).
		Count(&count).Error
	return count, err
}
