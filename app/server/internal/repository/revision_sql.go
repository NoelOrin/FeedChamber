package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
)

type sqlRevisionRepo struct{ db db.SQLStore }

func (r *sqlRevisionRepo) Create(ctx context.Context, rev *model.Revision) error {
	if rev.ID == "" {
		rev.ID = pkg.NewID()
	}
	return r.db.DB().WithContext(ctx).Create(rev).Error
}

func (r *sqlRevisionRepo) ListByTarget(ctx context.Context, targetType, targetID string, offset, limit int) ([]model.Revision, int, error) {
	var items []model.Revision
	var total int64
	q := r.db.DB().WithContext(ctx).Model(&model.Revision{}).Where("target_type = ? AND target_id = ?", targetType, targetID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, int(total), err
}

func (r *sqlRevisionRepo) GetByID(ctx context.Context, id string) (*model.Revision, error) {
	var rev model.Revision
	err := r.db.DB().WithContext(ctx).First(&rev, "id = ?", id).Error
	return &rev, WrapDBErr(err)
}
