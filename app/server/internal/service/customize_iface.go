package service

import (
	"context"

	"feedchamber/internal/model"
)

// WriteSource 标识一次写入来自哪里。分级生效策略据此决定高风险内容是否直落草稿。
//
// ui       — 站长在后台人工操作，直接按请求的启用状态写入；
// machine  — 机器凭证（MCP / API Key）调用，高风险内容强制草稿（除非持发布权限）。
type WriteSource string

const (
	WriteSourceUI      WriteSource = "ui"
	WriteSourceMachine WriteSource = "machine"
)

// WriteActor 是一次写入的操作者上下文。
type WriteActor struct {
	UserID string
	Source WriteSource
	// CanPublish 表示该操作者是否持 customize:publish，允许机器写入直接生效。
	CanPublish bool
}

// WriteResult 是自定义层写操作的统一返回。
//
// Applied 必须如实反映「这次写入是否已对外生效」——规范要求调用方
// 能够区分「已生效」与「已存为草稿」，不能让 AI 以为改了就生效了。
type WriteResult struct {
	Applied bool   `json:"applied"`
	Reason  string `json:"reason,omitempty"`
}

// CustomizeService 是站点自定义层的统一业务入口。
//
// 后台 UI 与 MCP 工具 MUST 都走这一层：机器入口不得绕过所有权、门控、
// 可见性等既有校验，也不得为机器入口单独开审核旁路（见 layered-architecture 规范）。
type CustomizeService interface {
	// ── 片段 ──────────────────────────────────────────────
	ListSnippets(ctx context.Context) ([]model.Snippet, error)
	CreateSnippet(ctx context.Context, in SnippetInput, actor WriteActor) (*model.Snippet, WriteResult, error)
	UpdateSnippet(ctx context.Context, id string, in SnippetInput, actor WriteActor) (*model.Snippet, WriteResult, error)
	DeleteSnippet(ctx context.Context, id string, actor WriteActor) error
	SetSnippetEnabled(ctx context.Context, id string, enabled bool, actor WriteActor) error

	// ── 主题 token ────────────────────────────────────────
	ListThemeTokens(ctx context.Context) ([]model.ThemeToken, error)
	SetThemeTokens(ctx context.Context, items []ThemeTokenInput, actor WriteActor) (WriteResult, error)
	DeleteThemeToken(ctx context.Context, token string, actor WriteActor) error
	ClearThemeTokens(ctx context.Context, actor WriteActor) error

	// ── 导航 ──────────────────────────────────────────────
	ListNavItems(ctx context.Context) ([]model.NavItem, error)
	CreateNavItem(ctx context.Context, in NavItemInput, actor WriteActor) (*model.NavItem, WriteResult, error)
	UpdateNavItem(ctx context.Context, id string, in NavItemInput, actor WriteActor) (*model.NavItem, WriteResult, error)
	DeleteNavItem(ctx context.Context, id string, actor WriteActor) error

	// ── 首页模块 ──────────────────────────────────────────
	ListHomeModules(ctx context.Context) ([]model.HomeModule, error)
	CreateHomeModule(ctx context.Context, in HomeModuleInput, actor WriteActor) (*model.HomeModule, WriteResult, error)
	UpdateHomeModule(ctx context.Context, id string, in HomeModuleInput, actor WriteActor) (*model.HomeModule, WriteResult, error)
	DeleteHomeModule(ctx context.Context, id string, actor WriteActor) error

	// ── 自定义页 ──────────────────────────────────────────
	ListCustomPages(ctx context.Context, page, pageSize int) ([]model.CustomPage, int64, error)
	CreateCustomPage(ctx context.Context, in CustomPageInput, actor WriteActor) (*model.CustomPage, WriteResult, error)
	UpdateCustomPage(ctx context.Context, id string, in CustomPageInput, actor WriteActor) (*model.CustomPage, WriteResult, error)
	DeleteCustomPage(ctx context.Context, id string, actor WriteActor) error
	PublishCustomPage(ctx context.Context, id string, publish bool, actor WriteActor) (*model.CustomPage, WriteResult, error)

	// ── 前台读取（已按可见性过滤） ────────────────────────
	// ActiveSnippets 返回指定位置的生效片段（含 html 的 body_end 归一化）。
	ActiveSnippets(ctx context.Context, position string) ([]model.Snippet, error)
	ThemeOverrides(ctx context.Context) ([]model.ThemeToken, error)
	PublicNav(ctx context.Context, isLogin bool, level int) ([]model.NavItem, error)
	PublicHomeModules(ctx context.Context, isLogin bool, level int) ([]model.HomeModule, error)
	// GetPublishedPage 只返回已发布页；草稿返回 ErrNotFound 语义。
	GetPublishedPage(ctx context.Context, slug string) (*model.CustomPage, error)
}

// SnippetInput 是片段的写入参数。
type SnippetInput struct {
	Kind     string `json:"kind"`
	Position string `json:"position"`
	Content  string `json:"content"`
	Enabled  bool   `json:"enabled"`
	Sort     int    `json:"sort"`
}

// ThemeTokenInput 是单条 token 覆盖。
type ThemeTokenInput struct {
	Token string `json:"token"`
	Value string `json:"value"`
	Mode  string `json:"mode"`
}

// NavItemInput 是导航项的写入参数。
type NavItemInput struct {
	Label      string `json:"label"`
	Href       string `json:"href"`
	Icon       string `json:"icon"`
	Sort       int    `json:"sort"`
	Visibility string `json:"visibility"`
	MinLevel   int    `json:"min_level"`
	Enabled    bool   `json:"enabled"`
}

// HomeModuleInput 是首页模块的写入参数。
type HomeModuleInput struct {
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Sort       int    `json:"sort"`
	Visibility string `json:"visibility"`
	MinLevel   int    `json:"min_level"`
	Config     string `json:"config"`
	Enabled    bool   `json:"enabled"`
}

// CustomPageInput 是自定义页的写入参数。
type CustomPageInput struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Status  string `json:"status"`
}
