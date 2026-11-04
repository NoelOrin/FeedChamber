package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// SlotEntryRepository 负责壳层插槽条目的持久化。
type SlotEntryRepository interface {
	Create(ctx context.Context, e *model.SlotEntry) error
	Update(ctx context.Context, e *model.SlotEntry) error
	Get(ctx context.Context, id string) (*model.SlotEntry, error)
	// List 返回全部条目（含停用），按 slot + sort 升序，供后台管理页。
	List(ctx context.Context, slot string) ([]model.SlotEntry, error)
	// ListEnabled 返回全部已启用条目，按 slot + sort 升序。
	// 可见性过滤在 service 层：仓库不做访问者判定。
	ListEnabled(ctx context.Context) ([]model.SlotEntry, error)
	Delete(ctx context.Context, id string) error
	// DeleteBySlotAndOwner 删除某插件在某插槽写入的条目。
	// 插件重写同一插槽时用它保证「覆盖」而不是「追加」。
	DeleteBySlotAndOwner(ctx context.Context, slot, pluginID string) error
}

func NewSlotEntryRepository(store db.Store) SlotEntryRepository {
	return &sqlSlotEntryRepo{db: store.SQL()}
}
