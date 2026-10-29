package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type AnnouncementRepository interface {
	FindByID(ctx context.Context, id string) (*model.Announcement, error)
	List(ctx context.Context, offset, limit int) ([]model.Announcement, error)
	ListPublished(ctx context.Context, offset, limit int) ([]model.Announcement, error)
	Create(ctx context.Context, ann *model.Announcement) error
	Update(ctx context.Context, ann *model.Announcement) error
	Delete(ctx context.Context, id string) error
	IncrementViewCount(ctx context.Context, id string) error
}

func NewAnnouncementRepository(store db.Store) AnnouncementRepository {
	return &sqlAnnouncementRepo{db: store.SQL()}
}
