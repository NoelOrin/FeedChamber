package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

type sqlAuditLogRepo struct{ db db.SQLStore }

func (r *sqlAuditLogRepo) Create(ctx context.Context, log *model.AuditLog) error {
	return r.db.DB().WithContext(ctx).Create(log).Error
}

func (r *sqlAuditLogRepo) List(ctx context.Context, offset, limit int, actorID, action string) ([]model.AuditLog, int64, error) {
	q := r.db.DB().WithContext(ctx).Model(&model.AuditLog{})
	if actorID != "" {
		q = q.Where("actor_id = ?", actorID)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AuditLog
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
