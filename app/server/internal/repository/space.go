package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type SpaceRepository interface {
	List(ctx context.Context) ([]model.Space, error)
	FindByID(ctx context.Context, id string) (*model.Space, error)
	FindBySlug(ctx context.Context, slug string) (*model.Space, error)
	Create(ctx context.Context, s *model.Space) error
	Update(ctx context.Context, s *model.Space) error
	Delete(ctx context.Context, id string) error
	ToggleActive(ctx context.Context, id string) (*model.Space, error)
	// SetSystemFlags 显式写 system/recommendable 两列（GORM 会省略零值，
	// 想要把 recommendable 置 false 必须走 map 更新）。
	SetSystemFlags(ctx context.Context, id string, system, recommendable bool) error
	// BackfillRecommendable 一次性把非系统版面的 recommendable 置 true（存量行补齐）。
	BackfillRecommendable(ctx context.Context) error
	FindChildren(ctx context.Context, parentID string) ([]model.Space, error)
	FindAncestors(ctx context.Context, id string) ([]model.Space, error)
	FindDescendants(ctx context.Context, id string) ([]model.Space, error)
	FindDescendantIDs(ctx context.Context, id string) ([]string, error)
	HasChildren(ctx context.Context, id string) (bool, error)
	CountChildren(ctx context.Context, parentID string) (int, error)
	ListRoots(ctx context.Context) ([]model.Space, error)
	Exists(ctx context.Context, id string) (bool, error)
	BatchUpdateSortOrder(ctx context.Context, orderedIDs []string) error
	BatchUpdateDepthPath(ctx context.Context, updates map[string]struct {
		Depth int
		Path  string
	}) error
}

func NewSpaceRepository(store db.Store) SpaceRepository {
	return &sqlSpaceRepo{db: store.SQL()}
}
