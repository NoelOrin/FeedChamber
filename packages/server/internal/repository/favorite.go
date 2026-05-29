package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type FavoriteRepository interface {
	Toggle(ctx context.Context, postID, userID string) (bool, error)
	HasFavorited(ctx context.Context, postID, userID string) (bool, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]string, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
}

func NewFavoriteRepository(store db.Store) FavoriteRepository {
	if store.SQL() != nil {
		return &sqlFavRepo{db: store.SQL()}
	}
	return &mongoFavRepo{store: store.Mongo()}
}

// ── SQL ─────────────────────────────────────────────────────────────────────

type sqlFavRepo struct{ db db.SQLStore }

func (r *sqlFavRepo) Toggle(ctx context.Context, postID, userID string) (bool, error) {
	var fav model.PostFavorite
	err := r.db.DB().WithContext(ctx).
		Where("post_id = ? AND user_id = ?", postID, userID).
		First(&fav).Error
	if err == nil {
		// exists → delete
		if err := r.db.DB().WithContext(ctx).Delete(&fav).Error; err != nil {
			return false, err
		}
		return false, nil
	}
	// not found → create
	fav = model.PostFavorite{
		ID:        uuid.NewString(),
		PostID:    postID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := r.db.DB().WithContext(ctx).Create(&fav).Error; err != nil {
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

// ── MongoDB ─────────────────────────────────────────────────────────────────

type mongoFavRepo struct{ store db.MongoStore }

func (r *mongoFavRepo) Toggle(ctx context.Context, postID, userID string) (bool, error) {
	coll := r.store.Collection("post_favorites")
	filter := bson.M{"post_id": postID, "user_id": userID}

	res, err := coll.DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	if res.DeletedCount > 0 {
		return false, nil
	}

	// not found → create
	fav := model.PostFavorite{
		ID:        uuid.NewString(),
		PostID:    postID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	_, err = coll.InsertOne(ctx, fav)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *mongoFavRepo) HasFavorited(ctx context.Context, postID, userID string) (bool, error) {
	count, err := r.store.Collection("post_favorites").
		CountDocuments(ctx, bson.M{"post_id": postID, "user_id": userID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *mongoFavRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]string, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := r.store.Collection("post_favorites").Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	var favs []model.PostFavorite
	if err := cursor.All(ctx, &favs); err != nil {
		return nil, err
	}
	ids := make([]string, len(favs))
	for i, f := range favs {
		ids[i] = f.PostID
	}
	return ids, nil
}

func (r *mongoFavRepo) CountByUser(ctx context.Context, userID string) (int64, error) {
	return r.store.Collection("post_favorites").
		CountDocuments(ctx, bson.M{"user_id": userID})
}
