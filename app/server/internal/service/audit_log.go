package service

import (
	"context"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type auditLogService struct {
	repo repository.AuditLogRepository
}

func NewAuditLogService(repo repository.AuditLogRepository) AuditLogService {
	return &auditLogService{repo: repo}
}

func (s *auditLogService) Log(ctx context.Context, actorID, action, target, detail string) error {
	return s.Record(ctx, actorID, action, target, detail)
}

func (s *auditLogService) Record(ctx context.Context, actorID, action, target, detail string) error {
	log := &model.AuditLog{
		ID:        pkg.NewID(),
		ActorID:   actorID,
		Action:    action,
		Target:    target,
		Detail:    detail,
		CreatedAt: time.Now(),
	}
	return s.repo.Create(ctx, log)
}

func (s *auditLogService) List(ctx context.Context, page, pageSize int, actorID, action string) ([]model.AuditLog, int64, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	offset := (page - 1) * pageSize
	logs, total, err := s.repo.List(ctx, offset, pageSize, actorID, action)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if logs == nil {
		logs = []model.AuditLog{}
	}
	return logs, total, nil
}
