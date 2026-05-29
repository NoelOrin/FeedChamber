package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UploadRepository interface {
	Create(ctx context.Context, upload *model.Upload) error
	FindByID(ctx context.Context, id string) (*model.Upload, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Upload, error)
	Delete(ctx context.Context, id string) error
}

func NewUploadRepository(store db.Store) UploadRepository {
	if store.SQL() != nil {
		return &sqlUploadRepo{db: store.SQL()}
	}
	return &mongoUploadRepo{store: store.Mongo()}
}

// ── SQL ──────────────────────────────────────────────────────────────────────

type sqlUploadRepo struct{ db db.SQLStore }

func (r *sqlUploadRepo) Create(ctx context.Context, upload *model.Upload) error {
	return r.db.DB().WithContext(ctx).Create(upload).Error
}

func (r *sqlUploadRepo) FindByID(ctx context.Context, id string) (*model.Upload, error) {
	var u model.Upload
	err := r.db.DB().WithContext(ctx).First(&u, "id = ?", id).Error
	return &u, err
}

func (r *sqlUploadRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Upload, error) {
	var uploads []model.Upload
	err := r.db.DB().WithContext(ctx).
		Where("uploader_id = ?", userID).
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Find(&uploads).Error
	return uploads, err
}

func (r *sqlUploadRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Upload{}, "id = ?", id).Error
}

// ── MongoDB ──────────────────────────────────────────────────────────────────

type mongoUploadRepo struct{ store db.MongoStore }

func (r *mongoUploadRepo) Create(ctx context.Context, upload *model.Upload) error {
	_, err := r.store.Collection("uploads").InsertOne(ctx, upload)
	return err
}

func (r *mongoUploadRepo) FindByID(ctx context.Context, id string) (*model.Upload, error) {
	var u model.Upload
	err := r.store.Collection("uploads").FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	return &u, err
}

func (r *mongoUploadRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Upload, error) {
	filter := bson.M{"uploader_id": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := r.store.Collection("uploads").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var uploads []model.Upload
	err = cursor.All(ctx, &uploads)
	return uploads, err
}

func (r *mongoUploadRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("uploads").DeleteOne(ctx, bson.M{"_id": id})
	return err
}
