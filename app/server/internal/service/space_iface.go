package service

import (
	"context"

	"feedchamber/internal/model"
)

type SpaceService interface {
	Tree(ctx context.Context) ([]*model.SpaceNode, error)
	Flat(ctx context.Context) ([]model.Space, error)
	GetByID(ctx context.Context, id string) (*model.Space, error)
	GetBySlug(ctx context.Context, slug string) (*model.Space, error)
	Create(ctx context.Context, name, description, icon, slug, parentID string, sortOrder int, visibility string, minLevel int) (*model.Space, error)
	Update(ctx context.Context, id, name, description, icon, slug, parentID string, sortOrder int, isActive bool, visibility string, minLevel int) (*model.Space, error)
	// VisibilityIndex 返回含祖先继承的有效可见性索引（内容出口共用）。
	VisibilityIndex(ctx context.Context) (VisibilityIndex, error)
	Delete(ctx context.Context, id string) error
	ToggleActive(ctx context.Context, id string) (*model.Space, error)
	GetChildren(ctx context.Context, parentID string) ([]model.Space, error)
	GetAncestors(ctx context.Context, id string) ([]model.Space, error)
	GetBreadcrumb(ctx context.Context, id string) ([]model.Space, error)
	GetDescendants(ctx context.Context, id string) ([]model.Space, error)
	GetDescendantIDs(ctx context.Context, id string) ([]string, error)
	GetSubTree(ctx context.Context, id string) (*model.SpaceNode, error)
	Move(ctx context.Context, id, newParentID string, sortOrder *int) (*model.Space, error)
	Reorder(ctx context.Context, parentID string, orderedIDs []string) error
	ListRoots(ctx context.Context) ([]model.Space, error)
	SetPermissionCacheUpdater(updater SpacePermissionCacheUpdater)
}
