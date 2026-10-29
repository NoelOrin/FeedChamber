package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"gorm.io/gorm"
)

type sqlReportRepo struct{ db db.SQLStore }

func (r *sqlReportRepo) Create(ctx context.Context, report *model.Report) error {
	return r.db.DB().WithContext(ctx).Create(report).Error
}

func (r *sqlReportRepo) FindByID(ctx context.Context, id string) (*model.Report, error) {
	var report model.Report
	return &report, WrapDBErr(r.db.DB().WithContext(ctx).First(&report, "id = ?", id).Error)
}

func (r *sqlReportRepo) FindPendingByTarget(ctx context.Context, targetType, targetID string) (*model.Report, error) {
	var report model.Report
	return &report, WrapDBErr(r.db.DB().WithContext(ctx).
		Where("target_type = ? AND target_id = ? AND status = 'pending'", targetType, targetID).
		Order("created_at DESC").
		First(&report).Error)
}

func (r *sqlReportRepo) IncrementCount(ctx context.Context, id, reporterID string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Report{}).Where("id = ?", id).Updates(map[string]any{
		"count":       gorm.Expr("count + 1"),
		"reporter_id": reporterID,
		"updated_at":  time.Now(),
	}).Error
}

func (r *sqlReportRepo) List(ctx context.Context, offset, limit int) ([]model.Report, error) {
	var reports []model.Report
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&reports).Error
	return reports, err
}

func (r *sqlReportRepo) ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.Report, error) {
	var reports []model.Report
	err := r.db.DB().WithContext(ctx).Where("status = ?", status).Order("created_at desc").Offset(offset).Limit(limit).Find(&reports).Error
	return reports, err
}

func (r *sqlReportRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Report{}).Count(&count).Error
	return count, err
}

func (r *sqlReportRepo) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Report{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *sqlReportRepo) UpdateStatus(ctx context.Context, id, status, actorID, action, note string) error {
	now := time.Now()
	return r.db.DB().WithContext(ctx).Model(&model.Report{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"action":     action,
		"handled_by": actorID,
		"handled_at": &now,
		"note":       note,
		"updated_at": now,
	}).Error
}

func (r *sqlReportRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Report{}, "id = ?", id).Error
}
