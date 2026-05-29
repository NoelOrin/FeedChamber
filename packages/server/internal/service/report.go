package service

import (
	"context"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"time"

	"github.com/google/uuid"
)

type ReportService interface {
	Create(ctx context.Context, reporterID, targetType, targetID, reason string) (*model.Report, error)
	List(ctx context.Context, status string, page, pageSize int) ([]model.Report, error)
	UpdateStatus(ctx context.Context, id, status string) error
	Delete(ctx context.Context, id string) error
}

type reportService struct {
	repo repository.ReportRepository
}

func NewReportService(repo repository.ReportRepository) ReportService {
	return &reportService{repo: repo}
}

func (s *reportService) Create(ctx context.Context, reporterID, targetType, targetID, reason string) (*model.Report, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "targetType must be 'post' or 'comment'")
	}
	report := &model.Report{
		ID:         uuid.NewString(),
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		Status:     "pending",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	return report, s.repo.Create(ctx, report)
}

func (s *reportService) List(ctx context.Context, status string, page, pageSize int) ([]model.Report, error) {
	offset := (page - 1) * pageSize
	if status != "" {
		return s.repo.ListByStatus(ctx, status, offset, pageSize)
	}
	return s.repo.List(ctx, offset, pageSize)
}

func (s *reportService) UpdateStatus(ctx context.Context, id, status string) error {
	if status != "pending" && status != "resolved" && status != "dismissed" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "status must be 'pending', 'resolved', or 'dismissed'")
	}
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *reportService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
