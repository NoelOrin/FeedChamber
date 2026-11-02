package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type AuditLogRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, offset, limit int, actorID, action string) ([]model.AuditLog, int64, error)
}

func NewAuditLogRepository(store db.Store) AuditLogRepository {
	return &sqlAuditLogRepo{db: store.SQL()}
}
