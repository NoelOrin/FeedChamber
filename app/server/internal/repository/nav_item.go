package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// NavItemRepository 负责自定义导航项的持久化。
type NavItemRepository interface {
	Create(ctx context.Context, n *model.NavItem) error
	Update(ctx context.Context, n *model.NavItem) error
	Get(ctx context.Context, id string) (*model.NavItem, error)
	List(ctx context.Context) ([]model.NavItem, error)
	// ListEnabled 返回全部已启用项，按 sort 升序；可见性过滤在 service 层。
	ListEnabled(ctx context.Context) ([]model.NavItem, error)
	Delete(ctx context.Context, id string) error
}

func NewNavItemRepository(store db.Store) NavItemRepository {
	return &sqlNavItemRepo{db: store.SQL()}
}
