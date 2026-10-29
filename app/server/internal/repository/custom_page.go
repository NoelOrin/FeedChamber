package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// CustomPageRepository 负责自定义静态页的持久化。
type CustomPageRepository interface {
	Create(ctx context.Context, p *model.CustomPage) error
	Update(ctx context.Context, p *model.CustomPage) error
	Get(ctx context.Context, id string) (*model.CustomPage, error)
	// GetBySlug 供前台 /p/{slug} 查询；调用方负责判断是否已发布。
	GetBySlug(ctx context.Context, slug string) (*model.CustomPage, error)
	List(ctx context.Context, offset, limit int) ([]model.CustomPage, error)
	Count(ctx context.Context) (int64, error)
	// ListPublished 供 sitemap 与前台列表使用。
	ListPublished(ctx context.Context) ([]model.CustomPage, error)
	Delete(ctx context.Context, id string) error
	SetStatus(ctx context.Context, id, status, updatedBy string) error
}

func NewCustomPageRepository(store db.Store) CustomPageRepository {
	return &sqlCustomPageRepo{db: store.SQL()}
}
