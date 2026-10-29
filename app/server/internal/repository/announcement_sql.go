package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"gorm.io/gorm"
)

type sqlAnnouncementRepo struct{ db db.SQLStore }

func (r *sqlAnnouncementRepo) FindByID(ctx context.Context, id string) (*model.Announcement, error) {
	var a model.Announcement
	return &a, WrapDBErr(r.db.DB().WithContext(ctx).Where("id = ?", id).First(&a).Error)
}

func (r *sqlAnnouncementRepo) List(ctx context.Context, offset, limit int) ([]model.Announcement, error) {
	var announcements []model.Announcement
	err := r.db.DB().WithContext(ctx).Order("sort desc, created_at desc").Offset(offset).Limit(limit).Find(&announcements).Error
	return announcements, err
}

func (r *sqlAnnouncementRepo) ListPublished(ctx context.Context, offset, limit int) ([]model.Announcement, error) {
	var announcements []model.Announcement
	now := time.Now()
	err := r.db.DB().WithContext(ctx).
		Where("status = ? AND (expires_at IS NULL OR expires_at > ?)", model.AnnStatusPublished, now).
		Order("is_pinned desc, sort desc, created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&announcements).Error
	return announcements, err
}

func (r *sqlAnnouncementRepo) Create(ctx context.Context, ann *model.Announcement) error {
	return r.db.DB().WithContext(ctx).Create(ann).Error
}

func (r *sqlAnnouncementRepo) Update(ctx context.Context, ann *model.Announcement) error {
	return r.db.DB().WithContext(ctx).Where(SQLSoftDeleteCondition()).Save(ann).Error
}

func (r *sqlAnnouncementRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Announcement{}, "id = ?", id).Error
}

func (r *sqlAnnouncementRepo) IncrementViewCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Announcement{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}
