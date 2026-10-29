package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

type WebhookRepository interface {
	Create(ctx context.Context, w *model.Webhook) error
	Update(ctx context.Context, w *model.Webhook) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*model.Webhook, error)
	List(ctx context.Context, offset, limit int) ([]model.Webhook, int64, error)
	ListEnabled(ctx context.Context) ([]model.Webhook, error)
	CreateDelivery(ctx context.Context, d *model.WebhookDelivery) error
	UpdateDelivery(ctx context.Context, d *model.WebhookDelivery) error
	ListDeliveries(ctx context.Context, webhookID string, offset, limit int) ([]model.WebhookDelivery, int64, error)
	GetDelivery(ctx context.Context, id string) (*model.WebhookDelivery, error)
	ListPendingDeliveries(ctx context.Context, limit int) ([]model.WebhookDelivery, error)
	// CountPendingDeliveries returns the true backlog size. The worker's
	// ListPendingDeliveries is capped by its batch size, so the two must stay
	// distinct: feeding a batch length into a backlog gauge pins it at the cap.
	CountPendingDeliveries(ctx context.Context) (int64, error)
}

func NewWebhookRepository(store db.Store) WebhookRepository {
	return &sqlWebhookRepo{db: store.SQL()}
}
