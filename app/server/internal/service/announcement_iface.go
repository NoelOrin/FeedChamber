package service

import (
	"context"

	"feedchamber/internal/model"
)

type AnnouncementService interface {
	Create(ctx context.Context, ann *model.Announcement) (*model.Announcement, error)
	Update(ctx context.Context, ann *model.Announcement) (*model.Announcement, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*model.Announcement, error)
	List(ctx context.Context, page, pageSize int) ([]model.Announcement, error)
	ListPublished(ctx context.Context, page, pageSize int) ([]model.Announcement, error)
	Publish(ctx context.Context, id string) error
	Unpublish(ctx context.Context, id string) error
	IncrementView(ctx context.Context, id string) error
}
