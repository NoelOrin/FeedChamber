package repository

import (
	"context"
	"errors"
	"time"

	"feedchamber/internal/db"
	"feedchamber/internal/model"

	"gorm.io/gorm"
)

type sqlThemeTokenRepo struct{ db db.SQLStore }

// Upsert 用「先查后写」实现按 token 名的幂等覆盖。
//
// 没有走 ON CONFLICT / ON DUPLICATE KEY：SQLite、MySQL、PostgreSQL 三家语法各异，
// 而这层只允许一份实现。表很小（几十行），两次往返的代价可以忽略。
func (r *sqlThemeTokenRepo) Upsert(ctx context.Context, t *model.ThemeToken) error {
	var existing model.ThemeToken
	err := r.db.DB().WithContext(ctx).First(&existing, "token = ?", t.Token).Error
	if err == nil {
		t.ID = existing.ID
		t.CreatedAt = existing.CreatedAt
		return WrapDBErr(r.db.DB().WithContext(ctx).Model(&model.ThemeToken{}).Where("token = ?", t.Token).Updates(map[string]any{
			"value":      t.Value,
			"mode":       t.Mode,
			"updated_by": t.UpdatedBy,
			"updated_at": time.Now(),
		}).Error)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return WrapDBErr(err)
	}
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(t).Error)
}

func (r *sqlThemeTokenRepo) GetByToken(ctx context.Context, token string) (*model.ThemeToken, error) {
	var t model.ThemeToken
	return &t, WrapDBErr(r.db.DB().WithContext(ctx).First(&t, "token = ?", token).Error)
}

func (r *sqlThemeTokenRepo) List(ctx context.Context) ([]model.ThemeToken, error) {
	var items []model.ThemeToken
	err := r.db.DB().WithContext(ctx).Order("token ASC").Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlThemeTokenRepo) DeleteByToken(ctx context.Context, token string) error {
	res := r.db.DB().WithContext(ctx).Delete(&model.ThemeToken{}, "token = ?", token)
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlThemeTokenRepo) DeleteAll(ctx context.Context) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Delete(&model.ThemeToken{}, "1 = 1").Error)
}
