package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type ReportRepository interface {
	Create(ctx context.Context, report *model.Report) error
	FindByID(ctx context.Context, id string) (*model.Report, error)
	FindPendingByTarget(ctx context.Context, targetType, targetID string) (*model.Report, error)
	IncrementCount(ctx context.Context, id, reporterID string) error
	List(ctx context.Context, offset, limit int) ([]model.Report, error)
	ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.Report, error)
	Count(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	UpdateStatus(ctx context.Context, id, status, actorID, action, note string) error
	Delete(ctx context.Context, id string) error
}

func NewReportRepository(store db.Store) ReportRepository {
	return &sqlReportRepo{db: store.SQL()}
}
