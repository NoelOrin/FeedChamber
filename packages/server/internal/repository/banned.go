package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type BannedRepository interface {
	FindByUserID(ctx context.Context, userID string) (*model.Banned, error)
	Create(ctx context.Context, banned *model.Banned) error
	Update(ctx context.Context, banned *model.Banned) error
	Delete(ctx context.Context, userID string) error
	List(ctx context.Context, offset, limit int) ([]model.Banned, error)
}

func NewBannedRepository(store db.Store) BannedRepository {
	if store.SQL() != nil {
		return &sqlBannedRepo{db: store.SQL()}
	}
	return &mongoBannedRepo{store: store.Mongo()}
}

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlBannedRepo struct{ db db.SQLStore }

func (r *sqlBannedRepo) FindByUserID(ctx context.Context, userID string) (*model.Banned, error) {
	var b model.Banned
	return &b, r.db.DB().WithContext(ctx).Where("user_id = ?", userID).First(&b).Error
}

func (r *sqlBannedRepo) Create(ctx context.Context, banned *model.Banned) error {
	return r.db.DB().WithContext(ctx).Create(banned).Error
}

func (r *sqlBannedRepo) Update(ctx context.Context, banned *model.Banned) error {
	return r.db.DB().WithContext(ctx).Save(banned).Error
}

func (r *sqlBannedRepo) Delete(ctx context.Context, userID string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Banned{}, "user_id = ?", userID).Error
}

func (r *sqlBannedRepo) List(ctx context.Context, offset, limit int) ([]model.Banned, error) {
	var bans []model.Banned
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&bans).Error
	return bans, err
}

// ── MongoDB implementation ──────────────────────────────────────────────────

type mongoBannedRepo struct{ store db.MongoStore }

func (r *mongoBannedRepo) FindByUserID(ctx context.Context, userID string) (*model.Banned, error) {
	var b model.Banned
	err := r.store.Collection("banned").FindOne(ctx, bson.M{"user_id": userID}).Decode(&b)
	return &b, err
}

func (r *mongoBannedRepo) Create(ctx context.Context, banned *model.Banned) error {
	_, err := r.store.Collection("banned").InsertOne(ctx, banned)
	return err
}

func (r *mongoBannedRepo) Update(ctx context.Context, banned *model.Banned) error {
	_, err := r.store.Collection("banned").ReplaceOne(ctx, bson.M{"user_id": banned.UserID}, banned)
	return err
}

func (r *mongoBannedRepo) Delete(ctx context.Context, userID string) error {
	_, err := r.store.Collection("banned").DeleteOne(ctx, bson.M{"user_id": userID})
	return err
}

func (r *mongoBannedRepo) List(ctx context.Context, offset, limit int) ([]model.Banned, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("banned").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var bans []model.Banned
	err = cursor.All(ctx, &bans)
	return bans, err
}
