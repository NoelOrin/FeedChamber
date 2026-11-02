package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"time"
)

// ── Content ──────────────────────────────────────────────────────────────────

type ContentRepository interface {
	ListByRoot(ctx context.Context, rootID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, error)
	CountByRoot(ctx context.Context, rootID string, startTime, endTime *time.Time) (int, error)
	ListByRootWithTotal(ctx context.Context, rootID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error)
	CountUnreadByRoot(ctx context.Context, rootID string, after time.Time) (int, error)
	ListByParent(ctx context.Context, parentID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, error)
	CountByParent(ctx context.Context, parentID string, startTime, endTime *time.Time) (int, error)
	ListByParentWithTotal(ctx context.Context, parentID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error)
	ListAll(ctx context.Context, offset, limit int) ([]model.ContentListItem, error)
	CountAll(ctx context.Context) (int, error)
	ListAllWithTotal(ctx context.Context, offset, limit int) ([]model.ContentListItem, int, error)
	ListTrash(ctx context.Context, offset, limit int) ([]model.ContentListItem, int, error)
	FindByID(ctx context.Context, id string) (*model.Content, error)
	Create(ctx context.Context, c *model.Content) error
	Update(ctx context.Context, c *model.Content) error
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) error
	IncrementLikeCount(ctx context.Context, id string) error
	DecrementLikeCount(ctx context.Context, id string) error
}

func NewContentRepository(store db.Store) ContentRepository {
	return &sqlContentRepo{db: store.SQL()}
}
