package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlUserMFARepo struct{ db db.SQLStore }

func (r *sqlUserMFARepo) Get(ctx context.Context, userID string) (*model.UserMFA, error) {
	var m model.UserMFA
	err := WrapDBErr(r.db.DB().WithContext(ctx).First(&m, "user_id = ?", userID).Error)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *sqlUserMFARepo) Upsert(ctx context.Context, m *model.UserMFA) error {
	return r.db.DB().WithContext(ctx).Save(m).Error
}

func (r *sqlUserMFARepo) Delete(ctx context.Context, userID string) error {
	return r.db.DB().WithContext(ctx).Where("user_id = ?", userID).Delete(&model.UserMFA{}).Error
}
