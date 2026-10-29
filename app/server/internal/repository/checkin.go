package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type CheckInRepository interface {
	Create(ctx context.Context, ci *model.CheckIn) error
	FindByUserAndDate(ctx context.Context, userID, date string) (*model.CheckIn, error)
	ListByUserAndMonth(ctx context.Context, userID, yearMonth string) ([]model.CheckIn, error)
}

func NewCheckInRepository(store db.Store) CheckInRepository {
	return &sqlCheckInRepo{db: store.SQL()}
}
