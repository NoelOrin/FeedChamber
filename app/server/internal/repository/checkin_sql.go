package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"
)

type sqlCheckInRepo struct{ db db.SQLStore }

func (r *sqlCheckInRepo) Create(ctx context.Context, ci *model.CheckIn) error {
	if ci.ID == "" {
		ci.ID = pkg.NewID()
	}
	if ci.CreatedAt.IsZero() {
		ci.CreatedAt = time.Now()
	}
	return r.db.DB().WithContext(ctx).Create(ci).Error
}

func (r *sqlCheckInRepo) FindByUserAndDate(ctx context.Context, userID, date string) (*model.CheckIn, error) {
	var ci model.CheckIn
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ? AND check_in_date = ?", userID, date).
		First(&ci).Error
	return &ci, WrapDBErr(err)
}

func (r *sqlCheckInRepo) ListByUserAndMonth(ctx context.Context, userID, yearMonth string) ([]model.CheckIn, error) {
	var records []model.CheckIn
	// yearMonth format: "2006-01" — match by LIKE prefix on check_in_date
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ? AND check_in_date LIKE ?", userID, yearMonth+"%").
		Order("check_in_date ASC").
		Find(&records).Error
	return records, WrapDBErr(err)
}
