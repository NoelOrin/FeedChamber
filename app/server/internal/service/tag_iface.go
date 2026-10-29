package service

import (
	"context"

	"feedchamber/internal/model"
)

type TagService interface {
	List(ctx context.Context, page, pageSize int) ([]model.Tag, int, error)
	Create(ctx context.Context, name string) (*model.Tag, error)
	Update(ctx context.Context, id, name string) (*model.Tag, error)
	Delete(ctx context.Context, id string) error
	GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error)
	SetPostTags(ctx context.Context, postID string, tagNames []string) error
	GetPostsByTagName(ctx context.Context, tagName string, page, pageSize int) ([]string, error)
}
