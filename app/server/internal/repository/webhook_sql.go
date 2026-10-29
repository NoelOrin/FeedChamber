package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type sqlWebhookRepo struct{ db db.SQLStore }

func (r *sqlWebhookRepo) Create(ctx context.Context, w *model.Webhook) error {
	return r.db.DB().WithContext(ctx).Create(w).Error
}
func (r *sqlWebhookRepo) Update(ctx context.Context, w *model.Webhook) error {
	return r.db.DB().WithContext(ctx).Save(w).Error
}
func (r *sqlWebhookRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Where("id = ?", id).Delete(&model.Webhook{}).Error
}
func (r *sqlWebhookRepo) Get(ctx context.Context, id string) (*model.Webhook, error) {
	var w model.Webhook
	err := WrapDBErr(r.db.DB().WithContext(ctx).First(&w, "id = ?", id).Error)
	if err != nil {
		return nil, err
	}
	return &w, nil
}
func (r *sqlWebhookRepo) List(ctx context.Context, offset, limit int) ([]model.Webhook, int64, error) {
	var items []model.Webhook
	var total int64
	q := r.db.DB().WithContext(ctx).Model(&model.Webhook{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}
func (r *sqlWebhookRepo) ListEnabled(ctx context.Context) ([]model.Webhook, error) {
	var items []model.Webhook
	err := r.db.DB().WithContext(ctx).Where("enabled = ?", true).Find(&items).Error
	return items, err
}
func (r *sqlWebhookRepo) CreateDelivery(ctx context.Context, d *model.WebhookDelivery) error {
	return r.db.DB().WithContext(ctx).Create(d).Error
}
func (r *sqlWebhookRepo) UpdateDelivery(ctx context.Context, d *model.WebhookDelivery) error {
	return r.db.DB().WithContext(ctx).Save(d).Error
}
func (r *sqlWebhookRepo) ListDeliveries(ctx context.Context, webhookID string, offset, limit int) ([]model.WebhookDelivery, int64, error) {
	var items []model.WebhookDelivery
	var total int64
	q := r.db.DB().WithContext(ctx).Model(&model.WebhookDelivery{}).Where("webhook_id = ?", webhookID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}
func (r *sqlWebhookRepo) GetDelivery(ctx context.Context, id string) (*model.WebhookDelivery, error) {
	var d model.WebhookDelivery
	err := WrapDBErr(r.db.DB().WithContext(ctx).First(&d, "id = ?", id).Error)
	if err != nil {
		return nil, err
	}
	return &d, nil
}
func (r *sqlWebhookRepo) ListPendingDeliveries(ctx context.Context, limit int) ([]model.WebhookDelivery, error) {
	var items []model.WebhookDelivery
	err := r.db.DB().WithContext(ctx).Where("status = ? AND attempts < ?", "pending", 5).Order("created_at asc").Limit(limit).Find(&items).Error
	return items, err
}

// CountPendingDeliveries counts the whole pending backlog with the same
// predicate as ListPendingDeliveries. Rows with a future NextRetryAt are
// included on purpose: they are still undelivered work waiting on the clock,
// and excluding them would make a retry storm look like an empty queue.
func (r *sqlWebhookRepo) CountPendingDeliveries(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.DB().WithContext(ctx).Model(&model.WebhookDelivery{}).
		Where("status = ? AND attempts < ?", "pending", 5).
		Count(&n).Error
	return n, err
}
