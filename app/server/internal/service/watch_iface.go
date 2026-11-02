package service

import (
	"context"
	"feedchamber/internal/model"
)

type WatchService interface {
	Set(ctx context.Context, userID, targetType, targetID, level string) (*model.Watch, error)
	Delete(ctx context.Context, userID, targetType, targetID string) error
	Get(ctx context.Context, userID, targetType, targetID string) (*model.Watch, error)
	List(ctx context.Context, userID string, page, pageSize int) ([]model.Watch, int, error)
	// NotifyWatchers fans out a watch notification for a post update/reply.
	// notified carries recipients already notified for this same event; entries
	// are skipped and new ones added so one comment yields one notification.
	// Excludes authorID and muted watchers. Dedupes across post/tag watches
	// （版面关注已于 2026-10-28 移除，见 model.Watch 的注释）。
	NotifyWatchers(ctx context.Context, postID, authorID, senderName, postTitle string, notified map[string]struct{})
}
