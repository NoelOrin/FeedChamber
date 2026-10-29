package service

import (
	"context"

	"feedchamber/internal/model"
)

type NotificationService interface {
	Create(ctx context.Context, userID, notifType, content, referenceID, senderID string) error
	List(ctx context.Context, userID string, page, pageSize int) ([]model.Notification, error)
	ListFiltered(ctx context.Context, userID string, page, pageSize int, typ, read string) ([]model.Notification, int64, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
	CountUnread(ctx context.Context, userID string) (int64, error)
	MarkRead(ctx context.Context, id, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, id, userID string) error
}
