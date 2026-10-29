package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"gorm.io/gorm"
)

type sqlInviteRepo struct{ db db.SQLStore }

func (r *sqlInviteRepo) Create(ctx context.Context, invite *model.Invite) error {
	return r.db.DB().WithContext(ctx).Create(invite).Error
}

func (r *sqlInviteRepo) FindByCode(ctx context.Context, code string) (*model.Invite, error) {
	var inv model.Invite
	err := r.db.DB().WithContext(ctx).Where("code = ?", code).First(&inv).Error
	return &inv, WrapDBErr(err)
}

func (r *sqlInviteRepo) FindByID(ctx context.Context, id string) (*model.Invite, error) {
	var inv model.Invite
	err := r.db.DB().WithContext(ctx).Where("id = ?", id).First(&inv).Error
	return &inv, WrapDBErr(err)
}

func (r *sqlInviteRepo) List(ctx context.Context, offset, limit int) ([]model.Invite, int64, error) {
	var (
		items []model.Invite
		total int64
	)
	q := r.db.DB().WithContext(ctx).Model(&model.Invite{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *sqlInviteRepo) ConsumeByCode(ctx context.Context, code string) error {
	now := time.Now()
	result := db.GetTxDB(ctx, r.db.DB()).WithContext(ctx).
		Model(&model.Invite{}).
		Where("code = ?", code).
		Where("max_uses = 0 OR used_count < max_uses").
		Where("expires_at IS NULL OR expires_at > ?", now).
		UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	return r.classifyConsumeFailure(ctx, code, now)
}

func (r *sqlInviteRepo) classifyConsumeFailure(ctx context.Context, code string, now time.Time) error {
	inv, err := r.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	if inv.ExpiresAt != nil && !inv.ExpiresAt.After(now) {
		return ErrInviteExpired
	}
	if inv.MaxUses > 0 && inv.UsedCount >= inv.MaxUses {
		return ErrInviteExhausted
	}
	// 并发下条件已不成立时，按用尽处理，避免超用。
	return ErrInviteExhausted
}

func (r *sqlInviteRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Invite{}, "id = ?", id).Error
}
