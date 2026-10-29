package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"gorm.io/gorm"
)

type sqlMotionRepo struct{ db db.SQLStore }

func (r *sqlMotionRepo) Create(ctx context.Context, m *model.Motion) error {
	return r.db.DB().WithContext(ctx).Create(m).Error
}

func (r *sqlMotionRepo) FindByID(ctx context.Context, id string) (*model.Motion, error) {
	var m model.Motion
	return &m, WrapDBErr(r.db.DB().WithContext(ctx).First(&m, "id = ?", id).Error)
}

func (r *sqlMotionRepo) FindPendingByTarget(ctx context.Context, targetType, targetID string) (*model.Motion, error) {
	var m model.Motion
	return &m, WrapDBErr(r.db.DB().WithContext(ctx).
		Where("target_type = ? AND target_id = ? AND status = 'pending'", targetType, targetID).
		Order("created_at DESC").
		First(&m).Error)
}

func (r *sqlMotionRepo) List(ctx context.Context, status string, targetType string, offset, limit int) ([]model.Motion, int64, error) {
	var (
		items []model.Motion
		total int64
	)
	q := r.db.DB().WithContext(ctx).Model(&model.Motion{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, WrapDBErr(err)
	}
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, WrapDBErr(err)
}

func (r *sqlMotionRepo) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Motion{}).Where("status = ?", status).Count(&count).Error
	return count, WrapDBErr(err)
}

func (r *sqlMotionRepo) AddVote(ctx context.Context, v *model.MotionVote) error {
	// 唯一约束 (motion_id, voter_id) 防重复投票；冲突直接返回 ErrDuplicate。
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(v).Error)
}

func (r *sqlMotionRepo) FindVote(ctx context.Context, motionID, voterID string) (*model.MotionVote, error) {
	var v model.MotionVote
	return &v, WrapDBErr(r.db.DB().WithContext(ctx).
		Where("motion_id = ? AND voter_id = ?", motionID, voterID).
		First(&v).Error)
}

func (r *sqlMotionRepo) IncrementTally(ctx context.Context, motionID string, approveDelta, rejectDelta float64, voteDelta int) error {
	updates := map[string]any{"updated_at": time.Now()}
	if approveDelta != 0 {
		updates["approve_weight"] = gorm.Expr("approve_weight + ?", approveDelta)
	}
	if rejectDelta != 0 {
		updates["reject_weight"] = gorm.Expr("reject_weight + ?", rejectDelta)
	}
	if voteDelta != 0 {
		updates["vote_count"] = gorm.Expr("vote_count + ?", voteDelta)
	}
	return WrapDBErr(r.db.DB().WithContext(ctx).Model(&model.Motion{}).Where("id = ?", motionID).Updates(updates).Error)
}

func (r *sqlMotionRepo) UpdateStatus(ctx context.Context, id, status string, resolvedAt *time.Time) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Model(&model.Motion{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"resolved_at": resolvedAt,
		"updated_at":  time.Now(),
	}).Error)
}

func (r *sqlMotionRepo) MarkAbuse(ctx context.Context, id string, abuse bool) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Model(&model.Motion{}).Where("id = ?", id).Updates(map[string]any{
		"abuse_flag": abuse,
		"updated_at": time.Now(),
	}).Error)
}

func (r *sqlMotionRepo) Delete(ctx context.Context, id string) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Delete(&model.Motion{}, "id = ?", id).Error)
}

func (r *sqlMotionRepo) ListExpiredPending(ctx context.Context, before time.Time, limit int) ([]model.Motion, error) {
	var items []model.Motion
	err := r.db.DB().WithContext(ctx).
		Where("status = ? AND created_at <= ?", model.MotionStatusPending, before).
		Order("created_at asc").
		Limit(limit).
		Find(&items).Error
	return items, WrapDBErr(err)
}
