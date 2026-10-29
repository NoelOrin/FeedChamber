package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type MotionRepository interface {
	Create(ctx context.Context, m *model.Motion) error
	FindByID(ctx context.Context, id string) (*model.Motion, error)
	FindPendingByTarget(ctx context.Context, targetType, targetID string) (*model.Motion, error)
	List(ctx context.Context, status string, targetType string, offset, limit int) ([]model.Motion, int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	AddVote(ctx context.Context, v *model.MotionVote) error
	FindVote(ctx context.Context, motionID, voterID string) (*model.MotionVote, error)
	IncrementTally(ctx context.Context, motionID string, approveDelta, rejectDelta float64, voteDelta int) error
	UpdateStatus(ctx context.Context, id, status string, resolvedAt *time.Time) error
	MarkAbuse(ctx context.Context, id string, abuse bool) error
	Delete(ctx context.Context, id string) error
	// ListExpiredPending 返回仍在 pending 且 created_at <= before 的动议，
	// 供后台 ticker 结算（开放期满但未达法定人数）。
	ListExpiredPending(ctx context.Context, before time.Time, limit int) ([]model.Motion, error)
}

func NewMotionRepository(store db.Store) MotionRepository {
	return &sqlMotionRepo{db: store.SQL()}
}
