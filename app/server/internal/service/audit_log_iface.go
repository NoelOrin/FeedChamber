package service

import (
	"context"

	"feedchamber/internal/model"
)

type AuditLogService interface {
	// Log is the historical method name used by existing call sites.
	Log(ctx context.Context, actorID, action, target, detail string) error
	// Record is an alias for Log (Task 8 naming).
	Record(ctx context.Context, actorID, action, target, detail string) error
	List(ctx context.Context, page, pageSize int, actorID, action string) ([]model.AuditLog, int64, error)
}
