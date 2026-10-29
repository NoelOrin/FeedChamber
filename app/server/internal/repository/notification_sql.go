package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"

	"gorm.io/gorm"
)

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlNotificationRepo struct{ db db.SQLStore }

func (r *sqlNotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	if n.ID == "" {
		n.ID = pkg.NewID()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	return r.db.DB().WithContext(ctx).Create(n).Error
}

func (r *sqlNotificationRepo) FindByID(ctx context.Context, id string) (*model.Notification, error) {
	var n model.Notification
	return &n, WrapDBErr(r.db.DB().WithContext(ctx).First(&n, "id = ?", id).Error)
}

func (r *sqlNotificationRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Notification, error) {
	var notifications []model.Notification
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&notifications).Error
	return notifications, err
}

func (r *sqlNotificationRepo) ListByUserFiltered(ctx context.Context, userID, typ, read string, offset, limit int) ([]model.Notification, error) {
	q := r.db.DB().WithContext(ctx).Where("user_id = ?", userID)
	q = applyNotificationFilter(q, typ, read)
	var notifications []model.Notification
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&notifications).Error
	return notifications, err
}

func (r *sqlNotificationRepo) CountByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

func (r *sqlNotificationRepo) CountByUserFiltered(ctx context.Context, userID, typ, read string) (int64, error) {
	q := r.db.DB().WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", userID)
	q = applyNotificationFilter(q, typ, read)
	var count int64
	err := q.Count(&count).Error
	return count, err
}

func (r *sqlNotificationRepo) CountUnread(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

func (r *sqlNotificationRepo) MarkRead(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ?", id).
		UpdateColumn("is_read", true).Error
}

func (r *sqlNotificationRepo) MarkAllRead(ctx context.Context, userID string) error {
	return r.db.DB().WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		UpdateColumn("is_read", true).Error
}

func (r *sqlNotificationRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).
		Delete(&model.Notification{}, "id = ?", id).Error
}

func applyNotificationFilter(q *gorm.DB, typ, read string) *gorm.DB {
	if typ != "" && typ != "all" {
		q = q.Where("type = ?", typ)
	}
	switch read {
	case "unread":
		q = q.Where("is_read = ?", false)
	case "read":
		q = q.Where("is_read = ?", true)
	}
	return q
}
