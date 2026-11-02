package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"
)

// ToolsDeps 是工具集所需的服务。
//
// 全部是既有 service 的接口：MCP 只做适配，不新增业务规则。
// 唯一的例外是 CustomizeSvc——它被以 WriteSourceMachine 调用，
// 由 service 层决定高风险内容是否落草稿。
type ToolsDeps struct {
	Posts     service.PostService
	Contents  service.ContentService
	Likes     service.LikeService
	Favorites service.FavoriteService
	Spaces    service.SpaceService
	Tags      service.TagService
	Users     service.UserService
	Customize service.CustomizeService
	MaxBytes  int
}

// RegisterAll 把全部工具登记进注册表。写类工具会自动进入默认停用集。
func RegisterAll(r *Registry, d ToolsDeps) {
	registerReadTools(r, d)
	registerWriteTools(r, d)
	registerCustomizeTools(r, d)
}

// ── 读工具 ────────────────────────────────────────────────────

func registerReadTools(r *Registry, d ToolsDeps) {
	r.Register(Tool{
		Name:        "search_posts",
		Description: "搜索帖子。返回与站内搜索一致的结果集，遵循站点可见性门槛。",
		Perm:        model.PermPostRead,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"keyword":{"type":"string","description":"搜索关键词"},
				"page":{"type":"integer","description":"页码，从 1 开始","default":1},
				"pageSize":{"type":"integer","description":"每页条数，最大 50","default":20}
			},
			"required":["keyword"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				Keyword  string `json:"keyword"`
				Page     int    `json:"page"`
				PageSize int    `json:"pageSize"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			if strings.TrimSpace(args.Keyword) == "" {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "keyword 不能为空")
			}
			items, total, err := d.Posts.Search(ctx, args.Keyword, args.Page, clampPageSize(args.PageSize))
			if err != nil {
				return nil, err
			}
			return map[string]any{"items": items, "total": total}, nil
		},
	})

	r.Register(Tool{
		Name:        "get_post",
		Description: "按 ID 读取帖子详情（含正文）。正文超过结果上限时会被截断。",
		Perm:        model.PermPostRead,
		InputSchema: schema(`{
			"type":"object",
			"properties":{"id":{"type":"string","description":"帖子 ID"}},
			"required":["id"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				ID string `json:"id"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			if args.ID == "" {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "id 不能为空")
			}
			detail, err := d.Posts.GetDetail(ctx, args.ID)
			if err != nil {
				return nil, err
			}
			return detail, nil
		},
	})

	r.Register(Tool{
		Name:        "list_spaces",
		Description: "列出版面树。返回结构含版面 id / 名称 / slug / 可见性。",
		Perm:        model.PermPostRead,
		InputSchema: schema(`{"type":"object","properties":{}}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			tree, err := d.Spaces.Tree(ctx)
			if err != nil {
				return nil, err
			}
			return map[string]any{"spaces": tree}, nil
		},
	})

	r.Register(Tool{
		Name:        "list_tags",
		Description: "列出标签（分页）。",
		Perm:        model.PermPostRead,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"page":{"type":"integer","default":1},
				"pageSize":{"type":"integer","default":50}
			}
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				Page     int `json:"page"`
				PageSize int `json:"pageSize"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			items, total, err := d.Tags.List(ctx, args.Page, clampPageSize(args.PageSize))
			if err != nil {
				return nil, err
			}
			return map[string]any{"items": items, "total": total}, nil
		},
	})

	r.Register(Tool{
		Name:        "get_user_profile",
		Description: "按用户 ID 读取公开资料。",
		Perm:        model.PermUserRead,
		InputSchema: schema(`{
			"type":"object",
			"properties":{"id":{"type":"string","description":"用户 ID"}},
			"required":["id"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				ID string `json:"id"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			if args.ID == "" {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "id 不能为空")
			}
			user, err := d.Users.GetByID(ctx, args.ID)
			if err != nil {
				return nil, err
			}
			return user.ToPublic(), nil
		},
	})
}

// ── 内容写工具 ────────────────────────────────────────────────
//
// 这些工具走站点既有业务流程：初始状态由 moderation_mode 决定，
// MCP 不引入任何审核旁路（见 mcp-server 规范）。

func registerWriteTools(r *Registry, d ToolsDeps) {
	r.Register(Tool{
		Name:        "create_post",
		Description: "创建帖子。初始状态遵循站点审核策略（可能需要审核后才公开）。",
		Perm:        model.PermPostCreate,
		Write:       true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"title":{"type":"string","description":"标题"},
				"body":{"type":"string","description":"正文（Markdown）"},
				"spaceId":{"type":"string","description":"版面 ID，可为空"}
			},
			"required":["title","body"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				Title   string `json:"title"`
				Body    string `json:"body"`
				SpaceID string `json:"spaceId"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			if strings.TrimSpace(args.Title) == "" || strings.TrimSpace(args.Body) == "" {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "title 与 body 不能为空")
			}
			post, err := d.Posts.Create(ctx, in.UserID, args.Title, args.Body, args.SpaceID)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"id":     post.ID,
				"status": post.Status,
				"note":   postStatusNote(post.Status),
			}, nil
		},
	})

	r.Register(Tool{
		Name:        "create_comment",
		Description: "在帖子下发表评论。",
		Perm:        model.PermCommentCreate,
		Write:       true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"postId":{"type":"string","description":"帖子 ID"},
				"body":{"type":"string","description":"评论正文"},
				"parentId":{"type":"string","description":"父评论 ID（回复时填）"}
			},
			"required":["postId","body"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				PostID   string `json:"postId"`
				Body     string `json:"body"`
				ParentID string `json:"parentId"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			if args.PostID == "" || strings.TrimSpace(args.Body) == "" {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "postId 与 body 不能为空")
			}
			c, err := d.Contents.Create(ctx, args.PostID, in.UserID, args.Body, args.ParentID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": c.ID}, nil
		},
	})

	r.Register(Tool{
		Name:        "like_post",
		Description: "点赞或取消点赞帖子（toggle）。返回当前是否已点赞。",
		Perm:        model.PermContentLike,
		Write:       true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{"postId":{"type":"string","description":"帖子 ID"}},
			"required":["postId"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				PostID string `json:"postId"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			if args.PostID == "" {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "postId 不能为空")
			}
			liked, err := d.Likes.ToggleContentLike(ctx, args.PostID, in.UserID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"liked": liked}, nil
		},
	})

	r.Register(Tool{
		Name:        "favorite_post",
		Description: "收藏或取消收藏帖子（toggle）。返回当前是否已收藏。",
		Perm:        model.PermFavoriteSelf,
		Write:       true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{"postId":{"type":"string","description":"帖子 ID"}},
			"required":["postId"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				PostID string `json:"postId"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			if args.PostID == "" {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "postId 不能为空")
			}
			favorited, err := d.Favorites.Toggle(ctx, args.PostID, in.UserID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"favorited": favorited}, nil
		},
	})
}

func postStatusNote(status string) string {
	if status == model.PostStatusPending {
		return "帖子已保存，但站点审核策略要求人工审核后才能公开"
	}
	return "帖子已发布"
}

// ── 自定义层工具 ──────────────────────────────────────────────
//
// 全部以 WriteSourceMachine 调 service：高风险内容（JS / HTML / 导航 /
// 首页模块 / 自定义页）在凭证未持 customize:publish 时会被强制存为草稿。

func registerCustomizeTools(r *Registry, d ToolsDeps) {
	r.Register(Tool{
		Name:        "list_snippets",
		Description: "列出全部自定义片段（含未启用的草稿）。",
		Perm:        model.PermCustomizeRead,
		InputSchema: schema(`{"type":"object","properties":{}}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			items, err := d.Customize.ListSnippets(ctx)
			if err != nil {
				return nil, err
			}
			return map[string]any{"snippets": items}, nil
		},
	})

	r.Register(Tool{
		Name: "upsert_snippet",
		Description: "写入一段自定义片段（CSS / JS / HTML）。" +
			"CSS 直接生效；JS 与 HTML 默认保存为草稿，需站长在后台启用（除非凭证持 customize:publish）。",
		Perm:           model.PermCustomizeWrite,
		Write:          true,
		CustomizeWrite: true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","description":"已有片段 ID；留空则新建"},
				"kind":{"type":"string","enum":["css","js","html"]},
				"position":{"type":"string","enum":["head","body_end"]},
				"content":{"type":"string","description":"片段内容"},
				"enabled":{"type":"boolean","description":"是否请求生效；高风险类型可能被强制存为草稿"},
				"sort":{"type":"integer"}
			},
			"required":["kind","content"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				ID       string `json:"id"`
				Kind     string `json:"kind"`
				Position string `json:"position"`
				Content  string `json:"content"`
				Enabled  bool   `json:"enabled"`
				Sort     int    `json:"sort"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			machine := service.WriteActor{
				UserID:     in.UserID,
				Source:     service.WriteSourceMachine,
				CanPublish: in.CanPublish,
			}
			input := service.SnippetInput{
				Kind: args.Kind, Position: args.Position,
				Content: args.Content, Enabled: args.Enabled, Sort: args.Sort,
			}
			if args.ID == "" {
				// AI 新建片段默认请求生效；是否真的生效由 service 判定。
				if !hasEnabledKey(in.Args) {
					input.Enabled = true
				}
				snippet, res, err := d.Customize.CreateSnippet(ctx, input, machine)
				if err != nil {
					return nil, err
				}
				return writePayload("snippet", snippet, res), nil
			}
			snippet, res, err := d.Customize.UpdateSnippet(ctx, args.ID, input, machine)
			if err != nil {
				return nil, err
			}
			return writePayload("snippet", snippet, res), nil
		},
	})

	r.Register(Tool{
		Name:           "set_theme_tokens",
		Description:    "按 token 名覆盖主题视觉变量（如 --primary、--radius）。CSS 类改动直接生效。",
		Perm:           model.PermCustomizeWrite,
		Write:          true,
		CustomizeWrite: true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"items":{
					"type":"array",
					"items":{
						"type":"object",
						"properties":{
							"token":{"type":"string","description":"CSS 变量名，如 --primary"},
							"value":{"type":"string","description":"值，如 #7c3aed"},
							"mode":{"type":"string","enum":["all","light","dark"]}
						},
						"required":["token","value"]
					}
				}
			},
			"required":["items"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				Items []service.ThemeTokenInput `json:"items"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			res, err := d.Customize.SetThemeTokens(ctx, args.Items, service.WriteActor{
				UserID: in.UserID, Source: service.WriteSourceMachine, CanPublish: in.CanPublish,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"applied": res.Applied, "reason": res.Reason}, nil
		},
	})

	r.Register(Tool{
		Name:           "upsert_nav_item",
		Description:    "写入一条自定义导航项。默认保存为草稿，需站长在后台启用（除非凭证持 customize:publish）。",
		Perm:           model.PermCustomizeWrite,
		Write:          true,
		CustomizeWrite: true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","description":"已有导航项 ID；留空则新建"},
				"label":{"type":"string"},
				"href":{"type":"string"},
				"icon":{"type":"string"},
				"sort":{"type":"integer"},
				"visibility":{"type":"string","enum":["public","login","level"]},
				"minLevel":{"type":"integer"},
				"enabled":{"type":"boolean"}
			},
			"required":["label","href"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				ID         string `json:"id"`
				Label      string `json:"label"`
				Href       string `json:"href"`
				Icon       string `json:"icon"`
				Sort       int    `json:"sort"`
				Visibility string `json:"visibility"`
				MinLevel   int    `json:"minLevel"`
				Enabled    bool   `json:"enabled"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			machine := service.WriteActor{UserID: in.UserID, Source: service.WriteSourceMachine, CanPublish: in.CanPublish}
			input := service.NavItemInput{
				Label: args.Label, Href: args.Href, Icon: args.Icon,
				Sort: args.Sort, Visibility: args.Visibility,
				MinLevel: args.MinLevel, Enabled: args.Enabled,
			}
			if args.ID == "" {
				if !hasEnabledKey(in.Args) {
					input.Enabled = true
				}
				item, res, err := d.Customize.CreateNavItem(ctx, input, machine)
				if err != nil {
					return nil, err
				}
				return writePayload("item", item, res), nil
			}
			item, res, err := d.Customize.UpdateNavItem(ctx, args.ID, input, machine)
			if err != nil {
				return nil, err
			}
			return writePayload("item", item, res), nil
		},
	})

	r.Register(Tool{
		Name:           "upsert_home_module",
		Description:    "写入一个首页模块。html 类型默认保存为草稿，需站长启用（除非凭证持 customize:publish）。",
		Perm:           model.PermCustomizeWrite,
		Write:          true,
		CustomizeWrite: true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"kind":{"type":"string","enum":["latest","hot","spaces","html"]},
				"title":{"type":"string"},
				"sort":{"type":"integer"},
				"visibility":{"type":"string","enum":["public","login","level"]},
				"minLevel":{"type":"integer"},
				"config":{"type":"string","description":"模块专属 JSON 参数；html 模块在这里放内容"},
				"enabled":{"type":"boolean"}
			},
			"required":["kind"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				ID         string `json:"id"`
				Kind       string `json:"kind"`
				Title      string `json:"title"`
				Sort       int    `json:"sort"`
				Visibility string `json:"visibility"`
				MinLevel   int    `json:"minLevel"`
				Config     string `json:"config"`
				Enabled    bool   `json:"enabled"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			machine := service.WriteActor{UserID: in.UserID, Source: service.WriteSourceMachine, CanPublish: in.CanPublish}
			input := service.HomeModuleInput{
				Kind: args.Kind, Title: args.Title, Sort: args.Sort,
				Visibility: args.Visibility, MinLevel: args.MinLevel,
				Config: args.Config, Enabled: args.Enabled,
			}
			if args.ID == "" {
				if !hasEnabledKey(in.Args) {
					input.Enabled = true
				}
				m, res, err := d.Customize.CreateHomeModule(ctx, input, machine)
				if err != nil {
					return nil, err
				}
				return writePayload("module", m, res), nil
			}
			m, res, err := d.Customize.UpdateHomeModule(ctx, args.ID, input, machine)
			if err != nil {
				return nil, err
			}
			return writePayload("module", m, res), nil
		},
	})

	r.Register(Tool{
		Name:           "upsert_custom_page",
		Description:    "写入一个自定义页（/p/{slug}，Markdown）。默认存为草稿，需站长发布（除非凭证持 customize:publish）。",
		Perm:           model.PermCustomizeWrite,
		Write:          true,
		CustomizeWrite: true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"slug":{"type":"string","description":"URL 路径段，不得与既有路由冲突"},
				"title":{"type":"string"},
				"content":{"type":"string","description":"Markdown 正文"},
				"status":{"type":"string","enum":["draft","published"]}
			},
			"required":["slug","title"]
		}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			var args struct {
				ID      string `json:"id"`
				Slug    string `json:"slug"`
				Title   string `json:"title"`
				Content string `json:"content"`
				Status  string `json:"status"`
			}
			if err := in.DecodeArgs(&args); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "参数解析失败")
			}
			machine := service.WriteActor{UserID: in.UserID, Source: service.WriteSourceMachine, CanPublish: in.CanPublish}
			input := service.CustomPageInput{
				Slug: args.Slug, Title: args.Title, Content: args.Content, Status: args.Status,
			}
			if args.ID == "" {
				if input.Status == "" {
					input.Status = model.CustomPagePublished
				}
				p, res, err := d.Customize.CreateCustomPage(ctx, input, machine)
				if err != nil {
					return nil, err
				}
				return writePayload("page", p, res), nil
			}
			p, res, err := d.Customize.UpdateCustomPage(ctx, args.ID, input, machine)
			if err != nil {
				return nil, err
			}
			return writePayload("page", p, res), nil
		},
	})
}

// writePayload 是自定义层写工具的统一返回。
//
// applied 必须如实回给调用方：AI 需要知道「改了」与「改了但没生效」的区别，
// 否则它会告诉用户「已改好」，而站点上什么都没变。
func writePayload(key string, entity any, res service.WriteResult) map[string]any {
	out := map[string]any{
		key:       entity,
		"applied": res.Applied,
	}
	if res.Reason != "" {
		out["reason"] = res.Reason
	}
	if !res.Applied {
		out["hint"] = "本次写入未生效，需站长在后台「站点自定义」中启用"
	}
	return out
}

// hasEnabledKey 判断客户端是否显式传了 enabled 字段。
//
// 这决定 AI 未指定时的默认值：显式 false 表示「我就想存草稿」，
// 没传才用「请求生效」的默认。
func hasEnabledKey(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	_, ok := probe["enabled"]
	return ok
}

func clampPageSize(n int) int {
	if n <= 0 {
		return 20
	}
	if n > 50 {
		return 50
	}
	return n
}

// schema 把 JSON 字面量转成 RawMessage，写起来比手拼 struct 直观。
func schema(s string) json.RawMessage {
	return json.RawMessage(strings.TrimSpace(s))
}

var _ = fmt.Sprintf
