package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// SnippetRepository 负责自定义片段的持久化。
//
// 没有分页：片段是站长手工维护的少量条目（通常个位数），
// 分页只会让「当前生效的 JS 有哪些」这类查询变复杂。
type SnippetRepository interface {
	Create(ctx context.Context, s *model.Snippet) error
	Update(ctx context.Context, s *model.Snippet) error
	Get(ctx context.Context, id string) (*model.Snippet, error)
	List(ctx context.Context) ([]model.Snippet, error)
	// ListEnabled 返回指定注入位置下所有已启用片段，按 sort 升序。
	// 前台注入只走这条路径，避免把停用片段带出去。
	ListEnabled(ctx context.Context, position string) ([]model.Snippet, error)
	Delete(ctx context.Context, id string) error
	SetEnabled(ctx context.Context, id string, enabled bool, updatedBy string) error
}

func NewSnippetRepository(store db.Store) SnippetRepository {
	return &sqlSnippetRepo{db: store.SQL()}
}
