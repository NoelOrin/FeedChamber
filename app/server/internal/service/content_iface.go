package service

import (
	"context"
	"time"

	"feedchamber/internal/model"
)

// PostViewer 是「按帖子维度」的可见性判定入口，由 PostService 实现。
// 评论/回复等从属内容出口必须复用它，避免绕过帖子本身的等级门槛。
type PostViewer interface {
	CheckViewablePost(ctx context.Context, post *model.Post) error
}

type ContentService interface {
	ListByPost(ctx context.Context, postID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error)
	ListReplies(ctx context.Context, parentID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error)
	ListAll(ctx context.Context, page, pageSize int) ([]model.ContentListItem, int, error)
	ListTrash(ctx context.Context, page, pageSize int) ([]model.ContentListItem, int, error)
	GetByID(ctx context.Context, id string) (*model.ContentListItem, error)
	Create(ctx context.Context, postID, authorID, body, parentID string) (*model.Content, error)
	Update(ctx context.Context, postID, id, body, actorUUID, actorRole string) (*model.Content, error)
	Delete(ctx context.Context, postID, id, actorUUID, actorRole string) error
	AdminDelete(ctx context.Context, id, actorRole string) error
	RestoreTrash(ctx context.Context, id string) error
	// SetStatusForPlugin 供插件宿主 API 的 comments.setStatus 使用。它只改
	// 状态列且自带取值域校验，不经过所有权 / 权限判定——插件调用已经过
	// capability 校验，管理员操作则走 AdminDelete 等带权限的入口。
	SetStatusForPlugin(ctx context.Context, id, status string) error
}
