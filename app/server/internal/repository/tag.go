package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type TagRepository interface {
	List(ctx context.Context, offset, limit int) ([]model.Tag, error)
	Count(ctx context.Context) (int, error)
	ListWithTotal(ctx context.Context, offset, limit int) ([]model.Tag, int, error)
	FindByID(ctx context.Context, id string) (*model.Tag, error)
	GetByName(ctx context.Context, name string) (*model.Tag, error)
	Create(ctx context.Context, tag *model.Tag) error
	CreateIfNotExists(ctx context.Context, tag *model.Tag) error
	Update(ctx context.Context, tag *model.Tag) error
	Delete(ctx context.Context, id string) error
	AddPostTag(ctx context.Context, postID, tagID string) error
	RemovePostTags(ctx context.Context, postID string) error
	GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error)
	GetPostsByTagID(ctx context.Context, tagID string, offset, limit int) ([]string, error)
}

func NewTagRepository(store db.Store) TagRepository {
	return &sqlTagRepo{db: store.SQL()}
}
