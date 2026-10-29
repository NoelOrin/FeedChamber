package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sqlSettingRepo struct{ db db.SQLStore }

func (r *sqlSettingRepo) Get(ctx context.Context, key string) (*model.Setting, error) {
	var s model.Setting
	err := r.db.DB().WithContext(ctx).First(&s, "key = ?", key).Error
	if err != nil {
		// WrapDBErr maps the driver's not-found to ErrNotFound so the service
		// layer can tell "this key was never written" apart from "the database
		// is down". Returning the raw gorm error here used to collapse both
		// into NOT_FOUND, which made a broken DB look like a missing setting
		// and silently disabled whatever the key gated.
		return nil, WrapDBErr(err)
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

func (r *sqlSettingRepo) BatchSet(ctx context.Context, settings map[string]string) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range settings {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
			}).Create(&model.Setting{
				Key:       key,
				Value:     value,
				UpdatedAt: time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *sqlSettingRepo) Delete(ctx context.Context, key string) error {
	return r.db.DB().WithContext(ctx).Where("key = ?", key).Delete(&model.Setting{}).Error
}
