package model

import "time"

// 自定义页状态。
const (
	CustomPageDraft     = "draft"
	CustomPagePublished = "published"
)

// CustomPage 是一个以 /p/{slug} 访问的自定义静态页，正文为 Markdown。
//
// slug 必须唯一且不得与既有顶层路由冲突（保留字校验在 service 层）。
type CustomPage struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Slug      string    `gorm:"uniqueIndex;size:128;not null" json:"slug"`
	Title     string    `gorm:"size:256;not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Status    string    `gorm:"size:16;not null;default:'draft';index" json:"status"`
	UpdatedBy string    `gorm:"size:36" json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (CustomPage) TableName() string { return "custom_pages" }

// IsPublished 是「能否对站点访客可见」的唯一判据。
func (p CustomPage) IsPublished() bool { return p.Status == CustomPagePublished }

// ValidCustomPageStatus 校验状态取值。
func ValidCustomPageStatus(s string) bool {
	return s == CustomPageDraft || s == CustomPagePublished
}

// customPageReservedSlugs 是与站点既有顶层路由冲突的 slug。
//
// 这些前缀下有真实页面或协议端点，占用了会导致 /p/{slug} 永远不会被命中，
// 或者更糟——遮蔽既有功能。校验在 service 层，写入前拒绝。
var customPageReservedSlugs = map[string]struct{}{
	"":               {},
	"admin":          {},
	"api":            {},
	"login":          {},
	"logout":         {},
	"timeline":       {},
	"spaces":         {},
	"t":              {},
	"user":           {},
	"users":          {},
	"tag":            {},
	"tags":           {},
	"p":              {},
	"mcp":            {},
	"motions":        {},
	"settings":       {},
	"oauth":          {},
	"sso":            {},
	"upload":         {},
	"uploads":        {},
	"realtime":       {},
	"notification":   {},
	"pm":             {},
	"drafts":         {},
	"search":         {},
	"health":         {},
	"metrics":        {},
	"sitemap":        {},
	"sitemap-posts":  {},
	"sitemap-spaces": {},
	"sitemap-tags":   {},
	"robots":         {},
	"latest":         {},
	"swagger":        {},
	"well-known":     {},
	".well-known":    {},
}

// IsReservedSlug 判定 slug 是否与既有顶层路由冲突。
func IsReservedSlug(slug string) bool {
	_, reserved := customPageReservedSlugs[slug]
	return reserved
}
