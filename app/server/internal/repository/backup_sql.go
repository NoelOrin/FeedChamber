package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlBackupRepo struct{ db db.SQLStore }

func (r *sqlBackupRepo) Create(ctx context.Context, b *model.Backup) error {
	return r.db.DB().WithContext(ctx).Create(b).Error
}

func (r *sqlBackupRepo) ListAll(ctx context.Context) ([]model.Backup, error) {
	var items []model.Backup
	err := r.db.DB().WithContext(ctx).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *sqlBackupRepo) List(ctx context.Context, offset, limit int) ([]model.Backup, int, error) {
	var items []model.Backup
	var total int64
	if err := r.db.DB().WithContext(ctx).Model(&model.Backup{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.DB().WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, int(total), err
}

func (r *sqlBackupRepo) Get(ctx context.Context, id string) (*model.Backup, error) {
	var b model.Backup
	err := r.db.DB().WithContext(ctx).First(&b, "id = ?", id).Error
	return &b, WrapDBErr(err)
}

func (r *sqlBackupRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Backup{}, "id = ?", id).Error
}
