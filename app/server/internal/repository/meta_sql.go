package repository

import (
	"context"
	"errors"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sqlMetaRepo struct{ db db.SQLStore }

func (r *sqlMetaRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var m model.Meta
	err := r.db.TxDB(ctx).First(&m, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return m.Value, true, nil
}

func (r *sqlMetaRepo) Set(ctx context.Context, key, value string) error {
	return r.db.TxDB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&model.Meta{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
	}).Error
}

func (r *sqlMetaRepo) Delete(ctx context.Context, key string) error {
	return r.db.TxDB(ctx).Delete(&model.Meta{}, "key = ?", key).Error
}
