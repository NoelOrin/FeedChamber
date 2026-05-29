package repository

import (
	"context"
	"errors"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"
)

type FollowRepository interface {
	Follow(ctx context.Context, followerID, followingID string) error
	Unfollow(ctx context.Context, followerID, followingID string) error
	IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
	GetFollowers(ctx context.Context, userID string, offset, limit int) ([]string, error)
	GetFollowing(ctx context.Context, userID string, offset, limit int) ([]string, error)
	CountFollowers(ctx context.Context, userID string) (int64, error)
	CountFollowing(ctx context.Context, userID string) (int64, error)
}

func NewFollowRepository(store db.Store) FollowRepository {
	if store.SQL() != nil {
		return &sqlFollowRepo{db: store.SQL()}
	}
	return &mongoFollowRepo{store: store.Mongo()}
}

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlFollowRepo struct{ db db.SQLStore }

func (r *sqlFollowRepo) Follow(ctx context.Context, followerID, followingID string) error {
	// Check if already following
	var existing model.UserFollow
	err := r.db.DB().WithContext(ctx).
		First(&existing, "follower_id = ? AND following_id = ?", followerID, followingID).Error
	if err == nil {
		return nil // already following, idempotent
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	follow := &model.UserFollow{
		ID:          uuid.New().String(),
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

// ── MongoDB implementation ──────────────────────────────────────────────────

type mongoFollowRepo struct{ store db.MongoStore }

func (r *mongoFollowRepo) Follow(ctx context.Context, followerID, followingID string) error {
	// Check if already following
	err := r.store.Collection("user_follows").FindOne(ctx, bson.M{
		"follower_id":  followerID,
		"following_id": followingID,
	}).Err()
	if err == nil {
		return nil // already following, idempotent
	}

	follow := &model.UserFollow{
		ID:          uuid.New().String(),
		FollowerID:  followerID,
		FollowingID: followingID,
		CreatedAt:   time.Now(),
	}
	_, err = r.store.Collection("user_follows").InsertOne(ctx, follow)
	return err
}

func (r *mongoFollowRepo) Unfollow(ctx context.Context, followerID, followingID string) error {
	_, err := r.store.Collection("user_follows").DeleteOne(ctx, bson.M{
		"follower_id":  followerID,
		"following_id": followingID,
	})
	return err
}

func (r *mongoFollowRepo) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	err := r.store.Collection("user_follows").FindOne(ctx, bson.M{
		"follower_id":  followerID,
		"following_id": followingID,
	}).Err()
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *mongoFollowRepo) GetFollowers(ctx context.Context, userID string, offset, limit int) ([]string, error) {
	opts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit)).SetSort(bson.M{"created_at": -1})
	cursor, err := r.store.Collection("user_follows").Find(ctx, bson.M{
		"following_id": userID,
	}, opts)
	if err != nil {
		return nil, err
	}
	var follows []model.UserFollow
	if err = cursor.All(ctx, &follows); err != nil {
		return nil, err
	}
	ids := make([]string, len(follows))
	for i, f := range follows {
		ids[i] = f.FollowerID
	}
	return ids, nil
}

func (r *mongoFollowRepo) GetFollowing(ctx context.Context, userID string, offset, limit int) ([]string, error) {
	opts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit)).SetSort(bson.M{"created_at": -1})
	cursor, err := r.store.Collection("user_follows").Find(ctx, bson.M{
		"follower_id": userID,
	}, opts)
	if err != nil {
		return nil, err
	}
	var follows []model.UserFollow
	if err = cursor.All(ctx, &follows); err != nil {
		return nil, err
	}
	ids := make([]string, len(follows))
	for i, f := range follows {
		ids[i] = f.FollowingID
	}
	return ids, nil
}

func (r *mongoFollowRepo) CountFollowers(ctx context.Context, userID string) (int64, error) {
	count, err := r.store.Collection("user_follows").CountDocuments(ctx, bson.M{
		"following_id": userID,
	})
	return count, err
}

func (r *mongoFollowRepo) CountFollowing(ctx context.Context, userID string) (int64, error) {
	count, err := r.store.Collection("user_follows").CountDocuments(ctx, bson.M{
		"follower_id": userID,
	})
	return count, err
}
