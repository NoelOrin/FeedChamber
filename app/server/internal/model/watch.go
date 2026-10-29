package model

import "time"

// Watch levels
const (
	WatchLevelWatching = "watching"
	WatchLevelTracking = "tracking"
	WatchLevelMuted    = "muted"
)

// Watch target types
const (
	WatchTargetPost = "post"
	WatchTargetTag  = "tag"
)

// Watch tracks a user watching a post or tag.
//
// 版面关注（target_type="space"）已于 2026-10-28 移除：前台从无第二个入口、后端不再接受，
// 存量行由 db/migrations/cleanup_space_watches.sql 清理。别再把它加回来——「关注版面」
// 与「关注话题」语义重叠，产品上只保留后者。
type Watch struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"uniqueIndex:idx_watch;not null" json:"user_id"`
	TargetType string    `gorm:"uniqueIndex:idx_watch;size:16;not null" json:"target_type"` // post|tag
	TargetID   string    `gorm:"uniqueIndex:idx_watch;not null" json:"target_id"`
	Level      string    `gorm:"size:16;default:watching" json:"level"` // watching|tracking|muted
	CreatedAt  time.Time `json:"created_at"`
}

func (Watch) TableName() string { return "watches" }
