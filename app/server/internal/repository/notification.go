package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	FindByID(ctx context.Context, id string) (*model.Notification, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Notification, error)
	ListByUserFiltered(ctx context.Context, userID, typ, read string, offset, limit int) ([]model.Notification, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
	CountByUserFiltered(ctx context.Context, userID, typ, read string) (int64, error)
	CountUnread(ctx context.Context, userID string) (int64, error)
	MarkRead(ctx context.Context, id string) error
	MarkAllRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, id string) error
}

func NewNotificationRepository(store db.Store) NotificationRepository {
	return &sqlNotificationRepo{db: store.SQL()}
}
