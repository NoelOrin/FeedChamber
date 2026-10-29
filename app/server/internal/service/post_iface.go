package service

import (
	"context"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/repository"
)

type PostService interface {
	List(ctx context.Context, page, pageSize int, tagName, authorID, status string, startTime, endTime *time.Time, spaceName, spaceSlug, spaceID string, includeSub bool, sortBy string) ([]model.PostListItem, int, error)
	// ListByIDs 批量获取帖子详情（含作者名等）.
	ListByIDs(ctx context.Context, ids []string) ([]model.PostListItem, error)
	GetByID(ctx context.Context, id string) (*model.PostListItem, error)
	GetDetail(ctx context.Context, id string) (*model.PostDetail, error)
	// CheckViewablePost 复用它给从属内容出口（评论/回复）做可见性判定：
	// 帖子不可见时返回与详情一致的 404/403，从属内容不得绕过门槛。
	CheckViewablePost(ctx context.Context, post *model.Post) error
	Create(ctx context.Context, authorID, title, body, spaceID string) (*model.Post, error)
	Update(ctx context.Context, id, title, body, actorUUID, actorRole string) (*model.Post, error)
	Delete(ctx context.Context, id, actorUUID, actorRole string) error
	Search(ctx context.Context, keyword string, page, pageSize int) ([]model.PostListItem, int, error)
	ListTrash(ctx context.Context, page, pageSize int) ([]model.PostListItem, int, error)
	RestoreTrash(ctx context.Context, id string) error
	Suggestions(ctx context.Context, postID string, limit int) ([]model.PostListItem, error)
	TogglePin(ctx context.Context, id, actorRole string) (*model.Post, error)
	ToggleLock(ctx context.Context, id, actorRole string) (*model.Post, error)
	SetClosed(ctx context.Context, id string, closed bool, actorID, actorRole string) (*model.Post, error)
	// SetStatus changes a post status directly (approved / pending / rejected /
	// closed) without requiring a moderation record; the route is gated by
	// report:manage in the gateway.
	SetStatus(ctx context.Context, id, status string) error
	// SetFeatured 设置/取消精华（posts.featured_at），并重算作者 XP。
	SetFeatured(ctx context.Context, id string, featured bool) (*model.Post, error)
	SetPin(ctx context.Context, id, actorRole string, input SetPinRequest) (*model.Post, error)
	Unpin(ctx context.Context, id, actorRole string) (*model.Post, error)
	ListPinned(ctx context.Context, spaceID string, page, pageSize int) ([]model.PostListItem, int, error)
	FlushViewCounts(ctx context.Context) error
	StartViewCountFlusher(ctx context.Context) context.CancelFunc
	StartPinExpiryChecker(ctx context.Context) context.CancelFunc
	SetTrustService(svc TrustService)
	// MarkRead records that the user has read the post (upsert last_read_at).
	MarkRead(ctx context.Context, userID, postID string) error
	// MarkReadToContent records a comment-level read cursor for the post.
	MarkReadToContent(ctx context.Context, userID, postID, lastContentID string) error
	// EnrichWithReadState fills has_new / last_read_at for the given user.
	// No-op when userID is empty. Phase-2 rule: no record => has_new=false.
	EnrichWithReadState(ctx context.Context, userID string, items []model.PostListItem)
	// SitemapEntries returns one page of publicly visible posts for the sitemap.
	// It deliberately bypasses List(): no viewer filtering, no pagination cap, and
	// only the three fields a <url> element needs.
	SitemapEntries(ctx context.Context, page, pageSize int) ([]repository.SitemapEntry, error)
	// CountSitemapEntries reports how many posts the sitemap would contain.
	CountSitemapEntries(ctx context.Context) (int, error)
}

// SetPinRequest holds the input for a programmatic pin update.
type SetPinRequest struct {
	PinOrder       int    `json:"pin_order"`
	PinnedExpireAt *int64 `json:"pinned_expire_at,omitempty"` // unix timestamp
}
