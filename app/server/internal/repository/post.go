package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

// SetPinInput holds the fields for a programmatic pin update.
type SetPinInput struct {
	Pinned         bool
	PinOrder       int
	PinnedAt       *time.Time
	PinnedExpireAt *time.Time
}

// SitemapEntry is the projection used by the sitemap handler: just enough to
// build a <url><loc>…</loc><lastmod>…</lastmod></url> element.
type SitemapEntry struct {
	ID        string    `gorm:"column:id"`
	Slug      string    `gorm:"column:slug"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// PostRepository 是帖子数据访问的聚合接口。
//
// 它此前是 38 个方法的上帝接口：读、写、计数器、置顶、搜索、回收站、sitemap
// 全塞在一起，任何消费者要测都得伪造整个接口。现按职责拆成 5 个子接口，本接口
// 仅做嵌入以保持既有调用点不变（写操作、查询、计数器、置顶、内容面）。
//
// 新代码请依赖真正需要的最小子接口，而不是这个聚合体。
type PostRepository interface {
	PostReadRepository
	PostWriteRepository
	PostCounterRepository
	PostPinRepository
	PostContentRepository
}

// PostReadRepository 覆盖读取与查询：单条、批量、列表、搜索、回收站、sitemap。
type PostReadRepository interface {
	List(ctx context.Context, offset, limit int) ([]model.Post, error)
	FindByID(ctx context.Context, id string) (*model.Post, error)
	FindByIDs(ctx context.Context, ids []string) ([]*model.Post, error)
	FindByIDsDetails(ctx context.Context, ids []string) ([]model.PostListItem, error)
	FindByContentID(ctx context.Context, contentID string) (*model.Post, error)
	FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error)
	// FindDetailByID returns the full post detail (PostListItem + Body) in a single query.
	FindDetailByID(ctx context.Context, id string) (*model.PostDetail, error)
	// ListWithTotal returns posts matching the filter with total count in a single query.
	ListWithTotal(ctx context.Context, filter PostListFilter, offset, limit int) ([]model.PostListItem, int, error)
	Search(ctx context.Context, keyword string, offset, limit int) ([]model.PostListItem, error)
	CountSearch(ctx context.Context, keyword string) (int, error)
	// SearchWithTotal returns posts matching the keyword with total count in a single query.
	SearchWithTotal(ctx context.Context, keyword string, offset, limit int) ([]model.PostListItem, int, error)
	// SearchWithTotalFilter returns matching posts with total count, applying viewer/block filters.
	SearchWithTotalFilter(ctx context.Context, keyword string, filter PostListFilter, offset, limit int) ([]model.PostListItem, int, error)
	// ListTrash returns soft-deleted posts for the admin trash list.
	ListTrash(ctx context.Context, offset, limit int) ([]model.PostListItem, int, error)
	// CountBySpaceID returns the number of posts in a space (excluding soft-deleted).
	CountBySpaceID(ctx context.Context, spaceID string) (int, error)
}

// PostWriteRepository 覆盖创建、更新、删除与回收站恢复。
type PostWriteRepository interface {
	Create(ctx context.Context, post *model.Post) error
	CreateWithContent(ctx context.Context, post *model.Post, content *model.Content) error
	Update(ctx context.Context, post *model.Post) error
	Delete(ctx context.Context, id string) error
	// Restore removes the soft-delete marker from a post.
	Restore(ctx context.Context, id string) error
}

// PostCounterRepository 覆盖各类计数器与浏览/活跃时间的自增自减。
type PostCounterRepository interface {
	// TouchActive 更新帖子最后活跃时间（评论等互动后调用）。
	TouchActive(ctx context.Context, id string, at time.Time) error
	IncrementViewCount(ctx context.Context, id string) error
	IncrementViewCountBy(ctx context.Context, id string, delta int64) error
	IncrementCommentCount(ctx context.Context, id string) error
	DecrementCommentCount(ctx context.Context, id string) error
	IncrementLikeCount(ctx context.Context, id string) error
	DecrementLikeCount(ctx context.Context, id string) error
	IncrementFavoriteCount(ctx context.Context, id string) error
	DecrementFavoriteCount(ctx context.Context, id string) error
}

// PostPinRepository 覆盖置顶/锁定状态与过期清理。
type PostPinRepository interface {
	// TogglePinned 原子翻转 pinned 状态，返回更新后的帖子。
	TogglePinned(ctx context.Context, id string) (*model.Post, error)
	// ToggleLocked 原子翻转 locked 状态，返回更新后的帖子。
	ToggleLocked(ctx context.Context, id string) (*model.Post, error)
	// SetPin atomically updates all pin fields. Use for programmatic pin control.
	SetPin(ctx context.Context, id string, input SetPinInput) (*model.Post, error)
	// Unpin clears pin state (shorthand for SetPin with all-zero values).
	Unpin(ctx context.Context, id string) (*model.Post, error)
	// ExpirePins clears pins where pinned_expire_at <= now. Returns count.
	ExpirePins(ctx context.Context) (int64, error)
	// ListPinnedBySpace returns pinned posts for a space (or all spaces if spaceID is empty).
	ListPinnedBySpace(ctx context.Context, spaceID string, offset, limit int, vis PostListFilter) ([]model.PostListItem, int64, error)
}

// PostLookupRepository 是只读 + 计数器的组合，供只需要"查帖子、动计数器"
// 的服务使用（favorite / like）。它们从不创建、删除或置顶帖子。
type PostLookupRepository interface {
	FindByID(ctx context.Context, id string) (*model.Post, error)
	FindByContentID(ctx context.Context, contentID string) (*model.Post, error)
	IncrementLikeCount(ctx context.Context, id string) error
	DecrementLikeCount(ctx context.Context, id string) error
	IncrementFavoriteCount(ctx context.Context, id string) error
	DecrementFavoriteCount(ctx context.Context, id string) error
}

// PostDigestRepository 是周报所需的最小投影：按时间取帖子 + 取详情。
type PostDigestRepository interface {
	FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error)
	ListWithTotal(ctx context.Context, filter PostListFilter, offset, limit int) ([]model.PostListItem, int, error)
}

// PostContentRepository 覆盖 sitemap 所需的轻量投影查询。
//
// 单独拆出是因为它的读者只有 sitemap handler：它从不需要 join 用户/版面，
// 也从不 select body，让枚举整个语料保持廉价。
type PostContentRepository interface {
	// ListSitemapEntries returns the minimal fields needed to emit sitemap <url>
	// entries, newest first. Unlike ListWithTotal it never joins users/spaces and
	// never selects the body, so enumerating the whole corpus stays cheap.
	// ListSitemapEntries 只输出匿名可见的公开帖；visibleSpaceIDs 由调用方按版面
	// 可见性（含祖先继承、游客=TL0）解析后传入。
	ListSitemapEntries(ctx context.Context, offset, limit int, visibleSpaceIDs []string) ([]SitemapEntry, error)
	// CountSitemapEntries returns how many publicly visible posts exist, so the
	// sitemap index can declare exactly as many shards as there are pages.
	CountSitemapEntries(ctx context.Context, visibleSpaceIDs []string) (int, error)
}

func NewPostRepository(store db.Store) PostRepository {
	return &sqlPostRepo{db: store.SQL()}
}
