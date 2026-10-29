package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"
)

type sqlDraftRepo struct{ db db.SQLStore }

func (r *sqlDraftRepo) Upsert(ctx context.Context, d *model.Draft) (*model.Draft, error) {
	if d.ID == "" {
		d.ID = pkg.NewID()
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = time.Now()
	}
	var existing model.Draft
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ? AND kind = ? AND (target_id = ? OR (? = '' AND target_id = ''))", d.UserID, d.Kind, d.TargetID, d.TargetID).
		Order("updated_at DESC").
		First(&existing).Error
	if err == nil {
		existing.Title = d.Title
		existing.Body = d.Body
		existing.SpaceID = d.SpaceID
		existing.Tags = d.Tags
		existing.UpdatedAt = d.UpdatedAt
		if err := r.db.DB().WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err := r.db.DB().WithContext(ctx).Create(d).Error; err != nil {
		return nil, err
	}
	return d, nil
}

func (r *sqlDraftRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Draft, int, error) {
	var items []model.Draft
	var total int64
	q := r.db.DB().WithContext(ctx).Model(&model.Draft{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, int(total), err
}

func (r *sqlDraftRepo) GetByID(ctx context.Context, id, userID string) (*model.Draft, error) {
	var d model.Draft
	err := r.db.DB().WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&d).Error
	return &d, WrapDBErr(err)
}

func (r *sqlDraftRepo) Delete(ctx context.Context, id, userID string) error {
	return r.db.DB().WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.Draft{}).Error
}
