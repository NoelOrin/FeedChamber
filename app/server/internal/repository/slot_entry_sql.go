package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlSlotEntryRepo struct{ db db.SQLStore }

func (r *sqlSlotEntryRepo) Create(ctx context.Context, e *model.SlotEntry) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(e).Error)
}

func (r *sqlSlotEntryRepo) Update(ctx context.Context, e *model.SlotEntry) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Save(e).Error)
}

func (r *sqlSlotEntryRepo) Get(ctx context.Context, id string) (*model.SlotEntry, error) {
	var e model.SlotEntry
	return &e, WrapDBErr(r.db.DB().WithContext(ctx).First(&e, "id = ?", id).Error)
}

func (r *sqlSlotEntryRepo) List(ctx context.Context, slot string) ([]model.SlotEntry, error) {
	var items []model.SlotEntry
	q := r.db.DB().WithContext(ctx).Order("slot ASC, sort ASC, created_at ASC")
	if slot != "" {
		q = q.Where("slot = ?", slot)
	}
	return items, WrapDBErr(q.Find(&items).Error)
}

func (r *sqlSlotEntryRepo) ListEnabled(ctx context.Context) ([]model.SlotEntry, error) {
	var items []model.SlotEntry
	err := r.db.DB().WithContext(ctx).
		Where("enabled = ?", true).
		Order("slot ASC, sort ASC, created_at ASC").
		Find(&items).Error
	return items, WrapDBErr(err)
}

func (r *sqlSlotEntryRepo) Delete(ctx context.Context, id string) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Delete(&model.SlotEntry{}, "id = ?", id).Error)
}

func (r *sqlSlotEntryRepo) DeleteBySlotAndOwner(ctx context.Context, slot, pluginID string) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).
		Where("slot = ? AND owner_plugin_id = ?", slot, pluginID).
		Delete(&model.SlotEntry{}).Error)
}
