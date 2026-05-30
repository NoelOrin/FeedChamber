package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type AdminRepository interface {
	CountUsers(ctx context.Context) (int64, error)
	CountPosts(ctx context.Context) (int64, error)
	CountComments(ctx context.Context) (int64, error)
	CountCategories(ctx context.Context) (int64, error)
	CountTodayUsers(ctx context.Context) (int64, error)
	CountTodayPosts(ctx context.Context) (int64, error)
	CountTodayComments(ctx context.Context) (int64, error)
}

func NewAdminRepository(store db.Store) AdminRepository {
	if store.SQL() != nil {
		return &sqlAdminRepo{db: store.SQL()}
	}
	return &mongoAdminRepo{store: store.Mongo()}
}

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlAdminRepo struct{ db db.SQLStore }

func (r *sqlAdminRepo) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountPosts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Post{}).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountComments(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Content{}).Where("id != root_id").Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountCategories(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Category{}).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountTodayUsers(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.DB().WithContext(ctx).Model(&model.User{}).
		Where("created_at >= ?", today).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountTodayPosts(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.DB().WithContext(ctx).Model(&model.Post{}).
		Where("created_at >= ?", today).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountTodayComments(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.DB().WithContext(ctx).Model(&model.Content{}).
		Where("id != root_id AND created_at >= ?", today).Count(&count).Error
	return count, err
}

// ── MongoDB implementation ──────────────────────────────────────────────────

type mongoAdminRepo struct{ store db.MongoStore }

func (r *mongoAdminRepo) CountUsers(ctx context.Context) (int64, error) {
	return r.store.Collection("users").CountDocuments(ctx, bson.M{})
}

func (r *mongoAdminRepo) CountPosts(ctx context.Context) (int64, error) {
	return r.store.Collection("posts").CountDocuments(ctx, bson.M{})
}

func (r *mongoAdminRepo) CountComments(ctx context.Context) (int64, error) {
	return r.store.Collection("contents").CountDocuments(ctx, bson.M{"$expr": bson.M{"$ne": bson.A{"$_id", "$root_id"}}})
}

func (r *mongoAdminRepo) CountCategories(ctx context.Context) (int64, error) {
	return r.store.Collection("categories").CountDocuments(ctx, bson.M{})
}

func (r *mongoAdminRepo) CountTodayUsers(ctx context.Context) (int64, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return r.store.Collection("users").CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": today},
	})
}

func (r *mongoAdminRepo) CountTodayPosts(ctx context.Context) (int64, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return r.store.Collection("posts").CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": today},
	})
}

func (r *mongoAdminRepo) CountTodayComments(ctx context.Context) (int64, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return r.store.Collection("contents").CountDocuments(ctx, bson.M{
		"$expr": bson.M{"$ne": bson.A{"$_id", "$root_id"}},
		"created_at": bson.M{"$gte": today},
	})
}
