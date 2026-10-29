package service

import (
	"context"

	"feedchamber/internal/model"
)

type ReportService interface {
	Create(ctx context.Context, reporterID, targetType, targetID, flagType, reason string) (*model.Report, error)
	GetByID(ctx context.Context, id string) (*model.Report, error)
	List(ctx context.Context, status string, page, pageSize int) ([]model.Report, int64, error)
	UpdateStatus(ctx context.Context, id, status, action, actorID, note string) error
	Delete(ctx context.Context, id string) error
}
