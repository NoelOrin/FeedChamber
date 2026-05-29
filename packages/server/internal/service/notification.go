package service

import (
	"context"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/google/uuid"
	"time"
)

type NotificationService interface {
	Create(ctx context.Context, userID, notifType, content, referenceID, senderID string) error
	List(ctx context.Context, userID string, page, pageSize int) ([]model.Notification, error)
	CountUnread(ctx context.Context, userID string) (int64, error)
	MarkRead(ctx context.Context, id, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
}

type notificationService struct {
	repo repository.NotificationRepository
}

func NewNotificationService(repo repository.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) Create(ctx context.Context, userID, notifType, content, referenceID, senderID string) error {
	n := &model.Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		Type:        notifType,
		Content:     content,
		ReferenceID: referenceID,
		SenderID:    senderID,
		IsRead:      false,
		CreatedAt:   time.Now(),
	}
	return s.repo.Create(ctx, n)
}

func (s *notificationService) List(ctx context.Context, userID string, page, pageSize int) ([]model.Notification, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.ListByUser(ctx, userID, offset, pageSize)
}

func (s *notificationService) CountUnread(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *notificationService) MarkRead(ctx context.Context, id, userID string) error {
	// Fetch the notification and verify ownership
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "notification not found")
	}
	if n.UserID != userID {
		return pkg.NewAppError(pkg.NOT_FOUND, "notification not found")
	}
	return s.repo.MarkRead(ctx, id)
}

func (s *notificationService) MarkAllRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllRead(ctx, userID)
}
