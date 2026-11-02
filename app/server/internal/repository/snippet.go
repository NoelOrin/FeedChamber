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
	// ListEnabled 返回全部已启用片段，按 sort 升序。
	//
	// 不按 position 列过滤：模型里 `EffectivePosition()` 才是权威位置
	// （html 片段一律落 body_end，存量数据可能仍写着 head），位置匹配
	// 统一在 service 层做，避免 SQL 与模型两套判定打架。
	ListEnabled(ctx context.Context) ([]model.Snippet, error)
	Delete(ctx context.Context, id string) error
	SetEnabled(ctx context.Context, id string, enabled bool, updatedBy string) error
}

func NewSnippetRepository(store db.Store) SnippetRepository {
	return &sqlSnippetRepo{db: store.SQL()}
}
