package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm/clause"
)

type SettingRepository interface {
	Get(ctx context.Context, key string) (*model.Setting, error)
	GetAll(ctx context.Context) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

func NewSettingRepository(store db.Store) SettingRepository {
	if store.SQL() != nil {
		return &sqlSettingRepo{db: store.SQL()}
	}
	return &mongoSettingRepo{store: store.Mongo()}
}

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlSettingRepo struct{ db db.SQLStore }

func (r *sqlSettingRepo) Get(ctx context.Context, key string) (*model.Setting, error) {
	var s model.Setting
	err := r.db.DB().WithContext(ctx).First(&s, "key = ?", key).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *sqlSettingRepo) GetAll(ctx context.Context) (map[string]string, error) {
	var settings []model.Setting
	err := r.db.DB().WithContext(ctx).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *sqlSettingRepo) Set(ctx context.Context, key, value string) error {
	return r.db.DB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&model.Setting{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
	}).Error
}

func (r *sqlSettingRepo) Delete(ctx context.Context, key string) error {
	return r.db.DB().WithContext(ctx).Where("key = ?", key).Delete(&model.Setting{}).Error
}

// ── MongoDB implementation ──────────────────────────────────────────────────

type mongoSettingRepo struct{ store db.MongoStore }

func (r *mongoSettingRepo) Get(ctx context.Context, key string) (*model.Setting, error) {
	var s model.Setting
	err := r.store.Collection("settings").FindOne(ctx, bson.M{"_id": key}).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *mongoSettingRepo) GetAll(ctx context.Context) (map[string]string, error) {
	cursor, err := r.store.Collection("settings").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var settings []model.Setting
	if err := cursor.All(ctx, &settings); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *mongoSettingRepo) Set(ctx context.Context, key, value string) error {
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.store.Collection("settings").UpdateOne(
		ctx,
		bson.M{"_id": key},
		bson.M{"$set": bson.M{"value": value, "updated_at": time.Now()}},
		opts,
	)
	return err
}

func (r *mongoSettingRepo) Delete(ctx context.Context, key string) error {
	_, err := r.store.Collection("settings").DeleteOne(ctx, bson.M{"_id": key})
	return err
}
