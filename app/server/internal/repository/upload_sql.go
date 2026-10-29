package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlUploadRepo struct{ db db.SQLStore }

func (r *sqlUploadRepo) Create(ctx context.Context, upload *model.Upload) error {
	return r.db.DB().WithContext(ctx).Create(upload).Error
}

func (r *sqlUploadRepo) FindByID(ctx context.Context, id string) (*model.Upload, error) {
	var u model.Upload
	err := r.db.DB().WithContext(ctx).First(&u, "id = ?", id).Error
	return &u, WrapDBErr(err)
}

func (r *sqlUploadRepo) FindByKey(ctx context.Context, key string) (*model.Upload, error) {
	var u model.Upload
	err := r.db.DB().WithContext(ctx).First(&u, "key = ?", key).Error
	return &u, WrapDBErr(err)
}

func (r *sqlUploadRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Upload, error) {
	var uploads []model.Upload
	err := r.db.DB().WithContext(ctx).
		Where("uploader_id = ?", userID).
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Find(&uploads).Error
	return uploads, WrapDBErr(err)
}

func (r *sqlUploadRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Upload{}, "id = ?", id).Error
}
