package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type LikeRepository interface {
	ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error)
	HasContentLiked(ctx context.Context, contentID, userID string) (bool, error)
	CountContentLikes(ctx context.Context, contentID string) (int64, error)
}

func NewLikeRepository(store db.Store) LikeRepository {
	if store.SQL() != nil {
		return &sqlLikeRepo{db: store.SQL()}
	}
	return &mongoLikeRepo{store: store.Mongo()}
}

// ── SQL ──────────────────────────────────────────────────────────────────────

type sqlLikeRepo struct{ db db.SQLStore }

func (r *sqlLikeRepo) ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error) {
	var like model.ContentLike
	err := r.db.DB().WithContext(ctx).Where("content_id = ? AND user_id = ?", contentID, userID).First(&like).Error
	if err == nil {
		r.db.DB().WithContext(ctx).Delete(&like)
		return false, nil
	}
	like = model.ContentLike{ID: pkg.NewID(), ContentID: contentID, UserID: userID, CreatedAt: time.Now()}
	if err := r.db.DB().WithContext(ctx).Create(&like).Error; err != nil {
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

// ── MongoDB ──────────────────────────────────────────────────────────────────

type mongoLikeRepo struct{ store db.MongoStore }

func (r *mongoLikeRepo) ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error) {
	filter := bson.M{"content_id": contentID, "user_id": userID}
	res, err := r.store.Collection("content_likes").DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	if res.DeletedCount > 0 {
		return false, nil
	}
	like := model.ContentLike{ID: pkg.NewID(), ContentID: contentID, UserID: userID, CreatedAt: time.Now()}
	_, err = r.store.Collection("content_likes").InsertOne(ctx, like)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *mongoLikeRepo) HasContentLiked(ctx context.Context, contentID, userID string) (bool, error) {
	count, err := r.store.Collection("content_likes").CountDocuments(ctx, bson.M{"content_id": contentID, "user_id": userID})
	return count > 0, err
}

func (r *mongoLikeRepo) CountContentLikes(ctx context.Context, contentID string) (int64, error) {
	count, err := r.store.Collection("content_likes").CountDocuments(ctx, bson.M{"content_id": contentID})
	return count, err
}
