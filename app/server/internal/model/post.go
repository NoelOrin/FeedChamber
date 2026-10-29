package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	PostStatusApproved = "approved"
	PostStatusPending  = "pending"
	PostStatusRejected = "rejected"
	PostStatusClosed   = "closed" // phase 2
)

type Post struct {
	ID            string `gorm:"primaryKey;index:idx_posts_sitemap,priority:3" json:"id"`
	Title         string `gorm:"not null" json:"title"`
	Slug          string `gorm:"size:100;index" json:"slug"`
	ContentID     string `gorm:"uniqueIndex;not null" json:"content_id"`
	ContentDigest string `gorm:"size:200" json:"content_digest"`
	AuthorID      string `gorm:"not null;index" json:"author_id"`
	SpaceID       string `gorm:"index" json:"space_id"`
	Status        string `gorm:"size:16;default:approved;index" json:"status"`
	// MinLevel 是单帖可见性覆盖：0 = 继承版面策略；>0 = 需等级 ≥ MinLevel。
	MinLevel       int        `gorm:"default:0" json:"min_level"`
	Pinned         bool       `gorm:"default:false" json:"pinned"`
	PinOrder       int        `gorm:"default:0" json:"pin_order"` // lower = higher priority
	PinnedAt       *time.Time `json:"pinned_at,omitempty"`        // when pinned
	PinnedExpireAt *time.Time `json:"pinned_expire_at,omitempty"` // auto-unpin deadline
	// FeaturedAt 非空即精华。精华是 XP 指标 featured_post_count 的唯一来源。
	FeaturedAt    *time.Time     `json:"featured_at,omitempty"`
	Locked        bool           `gorm:"default:false" json:"locked"`
	Exiled        bool           `gorm:"default:false;index" json:"exiled"` // 暗区软隔离标记：主推荐流过滤，UI 折叠
	Folded        bool           `gorm:"default:false;index" json:"folded"` // 动议 fold 结果：内容降权（不删）
	WatchedUntil  *time.Time     `json:"watched_until,omitempty"`           // 动议 watch 结果：限时观察标记
	LastActiveAt  time.Time      `gorm:"index" json:"last_active_at"`
	ViewCount     int64          `gorm:"default:0" json:"view_count"`
	CommentCount  int64          `gorm:"default:0" json:"comment_count"`
	LikeCount     int64          `gorm:"default:0" json:"like_count"`
	FavoriteCount int64          `gorm:"default:0" json:"favorite_count"`
	DeletedAt     gorm.DeletedAt `gorm:"index;index:idx_posts_sitemap,priority:1" json:"deleted_at,omitempty" swaggertype:"string"`
	CreatedAt     time.Time      `json:"created_at"`
	// 复合索引专供 sitemap 的 `WHERE status IN (...) AND deleted_at IS NULL
	// ORDER BY updated_at DESC, id DESC`。
	//
	// 实测结论（跨库差异，勿按直觉假设「加了索引就一定更快」）：
	//   - SQLite：消除 `USE TEMP B-TREE FOR ORDER BY`，5 万帖下 offset=5000 从
	//     78ms 降到 4ms（18×）。列序必须是 (deleted_at, updated_at, id)：
	//     把 status 放前导列会因 `IN (...)` 是范围条件而阻断后续列的排序用途。
	//   - PostgreSQL：优化器**不会**选这个索引（deleted_at 选择性太差，只有极少数行
	//     非 NULL），实测强制启用与默认顺序扫描同为 ~8ms，无收益；本索引在 PG 上
	//     只增加写入成本。保留它是因为默认部署是 SQLite，而 PG 侧代价可忽略。
	//   - MySQL：未能验证——AutoMigrate 在本仓库既有缺陷处（如 Space.Name 这类
	//     无长度的 string + uniqueIndex）会直接失败，与索引无关。
	UpdatedAt time.Time `gorm:"index:idx_posts_sitemap,priority:2" json:"updated_at"`
}

// PostDetail is the full detail DTO, including the post body.
type PostDetail struct {
	PostListItem
	Body string `json:"body"`
}

// PostListItem is the DTO for post list queries (no JOIN needed).
type PostListItem struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Slug               string     `json:"slug"`
	ContentDigest      string     `json:"content_digest"`
	ContentID          string     `json:"content_id"`
	AuthorID           string     `json:"author_id"`
	AuthorName         string     `json:"author_name"`
	AuthorAvatar       string     `json:"author_avatar"`
	SpaceID            string     `json:"space_id"`
	SpaceName          string     `json:"space_name"`
	Status             string     `json:"status"`
	MinLevel           int        `json:"min_level"`
	Pinned             bool       `json:"pinned"`
	PinOrder           int        `json:"pin_order"`
	PinnedAt           *int64     `json:"pinned_at,omitempty"` // unix timestamp, nil if never pinned
	FeaturedAt         *time.Time `json:"featured_at,omitempty"`
	Locked             bool       `json:"locked"`
	LastActiveAt       time.Time  `json:"last_active_at"`
	ViewCount          int64      `json:"view_count"`
	CommentCount       int64      `json:"comment_count"`
	LikeCount          int64      `json:"like_count"`
	FavoriteCount      int64      `json:"favorite_count"`
	HasNew             bool       `json:"has_new"`
	LastReadAt         *time.Time `json:"last_read_at,omitempty"`
	UnreadContentCount int        `json:"unread_content_count,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
