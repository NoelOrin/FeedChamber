package handler

import (
	"feedchamber/internal/authz"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/logger"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// CustomizeHandler 是站点自定义层的 HTTP 入口。
//
// 写方法一律用 writeActor 构造 actor，**不再假定调用方是人工**：
// API Key 与 OAuth 客户端也能通过网关进入后台路由，它们必须被识别为机器
// 凭证（高风险内容强制草稿、publish 只认 scope），否则「后台」就成了绕过
// 分级生效的后门。
type CustomizeHandler interface {
	// 片段
	ListSnippets(ctx iris.Context)
	CreateSnippet(ctx iris.Context)
	UpdateSnippet(ctx iris.Context)
	DeleteSnippet(ctx iris.Context)
	ToggleSnippet(ctx iris.Context)
	// 主题 token
	ListThemeTokens(ctx iris.Context)
	SetThemeTokens(ctx iris.Context)
	DeleteThemeToken(ctx iris.Context)
	ClearThemeTokens(ctx iris.Context)
	// 导航
	ListNavItems(ctx iris.Context)
	CreateNavItem(ctx iris.Context)
	UpdateNavItem(ctx iris.Context)
	DeleteNavItem(ctx iris.Context)
	// 首页模块
	ListHomeModules(ctx iris.Context)
	CreateHomeModule(ctx iris.Context)
	UpdateHomeModule(ctx iris.Context)
	DeleteHomeModule(ctx iris.Context)
	// 自定义页
	ListCustomPages(ctx iris.Context)
	CreateCustomPage(ctx iris.Context)
	UpdateCustomPage(ctx iris.Context)
	DeleteCustomPage(ctx iris.Context)
	PublishCustomPage(ctx iris.Context)
	// 前台
	GetPublicPage(ctx iris.Context)
	GetPublicCustomization(ctx iris.Context)
}

// 以下具名请求体只为 swagger schema 而存在：处理函数仍按需读取字段，
// 但 openapi.json 里必须有 body 形状，否则客户端与 AI 只能猜参数。

// ToggleSnippetRequest 是片段启停的请求体。
type ToggleSnippetRequest struct {
	Enabled bool `json:"enabled"`
}

// SetThemeTokensRequest 是批量覆盖主题 token 的请求体。
type SetThemeTokensRequest struct {
	Items []service.ThemeTokenInput `json:"items"`
}

// ListCustomPagesRequest 是自定义页分页查询的请求体。
type ListCustomPagesRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// PublishCustomPageRequest 是自定义页发布/取消发布的请求体。
type PublishCustomPageRequest struct {
	Publish bool `json:"publish"`
}

type customizeHandler struct {
	svc service.CustomizeService
	// trust 只用于给前台聚合接口提供访问者的真实等级（可见性过滤需要它）。
	trust service.TrustService
}

func newCustomizeHandler(svc service.CustomizeService, trust service.TrustService) CustomizeHandler {
	return &customizeHandler{svc: svc, trust: trust}
}

// writeActor 构造写入上下文，凭证来源由网关写入的 auth_method 决定。
//
// 关键点：api_key / oauth_client 与人工操作走的是**同一批后台路由**
// （网关的 Enforce 对两者一视同仁），所以来源不能由路由推断，只能读凭证。
//   - 会话 → WriteSourceUI：站长本人在后台点按钮，按其请求的启用状态写入；
//   - 机器 → WriteSourceMachine：高风险内容强制草稿；
//   - CanPublish 只看凭证自身的 scope（与 MCP 的判据一致），不看角色继承。
func writeActor(ctx iris.Context) service.WriteActor {
	return actorFor(
		ctx.Values().GetString(pkg.CtxAuthMethod),
		ctx.Values().GetString(pkg.CtxAPIKeyScopes),
		ctx.Values().GetString(pkg.CtxClientScopes),
		currentUserID(ctx),
	)
}

// actorFor 是 writeActor 的纯函数内核，便于测试。
func actorFor(method, apiKeyScopes, clientScopes, userID string) service.WriteActor {
	// 会话 = 站长本人在后台操作。发布能力由 route 级 customize:publish 门控，
	// WriteActor.CanPublish 只对机器凭证有意义（service 层只在 Source==machine
	// 时读它），这里填 true 表示「不会因为机器凭证的草稿规则被降级」。
	actor := service.WriteActor{UserID: userID, Source: service.WriteSourceUI, CanPublish: true}
	switch method {
	case pkg.AuthMethodAPIKey:
		actor.Source = service.WriteSourceMachine
		actor.CanPublish = authz.ScopeHas(apiKeyScopes, model.PermCustomizePublish)
	case pkg.AuthMethodOAuthClient:
		actor.Source = service.WriteSourceMachine
		actor.CanPublish = authz.ScopeHas(clientScopes, model.PermCustomizePublish)
	}
	return actor
}

// ── 片段 ──────────────────────────────────────────────────────

// 片段列表
// @Summary      片段列表
// @Description  列出全部自定义片段（含未启用的草稿）。
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/customization/snippets/list [post]
func (h *customizeHandler) ListSnippets(ctx iris.Context) {
	items, err := h.svc.ListSnippets(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 创建片段
// @Summary      创建片段
// @Description  创建一段自定义 CSS / JS / HTML。
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body service.SnippetInput true "片段内容"
// @Router       /admin/customization/snippets/create [post]
func (h *customizeHandler) CreateSnippet(ctx iris.Context) {
	var in service.SnippetInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	snippet, res, err := h.svc.CreateSnippet(ctx.Request().Context(), in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"snippet": snippet, "applied": res.Applied, "reason": res.Reason})
}

// 更新片段
// @Summary      更新片段
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "片段 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.SnippetInput true "片段内容"
// @Param        payload body service.SnippetInput true "片段内容"
// @Router       /admin/customization/snippets/{id}/update [post]
func (h *customizeHandler) UpdateSnippet(ctx iris.Context) {
	id := ctx.Params().Get("id")
	var in service.SnippetInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	snippet, res, err := h.svc.UpdateSnippet(ctx.Request().Context(), id, in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"snippet": snippet, "applied": res.Applied, "reason": res.Reason})
}

// 删除片段
// @Summary      删除片段
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "片段 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.SnippetInput true "片段内容"
// @Router       /admin/customization/snippets/{id}/delete [post]
func (h *customizeHandler) DeleteSnippet(ctx iris.Context) {
	if err := h.svc.DeleteSnippet(ctx.Request().Context(), ctx.Params().Get("id"), writeActor(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 启停片段
// @Summary      启停片段
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "片段 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body ToggleSnippetRequest true "启停状态"
// @Router       /admin/customization/snippets/{id}/toggle [post]
func (h *customizeHandler) ToggleSnippet(ctx iris.Context) {
	var in ToggleSnippetRequest
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.SetSnippetEnabled(ctx.Request().Context(), ctx.Params().Get("id"), in.Enabled, writeActor(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ── 主题 token ────────────────────────────────────────────────

// 主题覆盖列表
// @Summary      主题覆盖列表
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body ToggleSnippetRequest true "启停状态"
// @Router       /admin/customization/tokens/list [post]
func (h *customizeHandler) ListThemeTokens(ctx iris.Context) {
	items, err := h.svc.ListThemeTokens(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 设置主题覆盖
// @Summary      设置主题覆盖
// @Description  按 token 名覆盖视觉变量。CSS 类改动风险低，直接生效。
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body SetThemeTokensRequest true "token 覆盖列表"
// @Router       /admin/customization/tokens/set [post]
func (h *customizeHandler) SetThemeTokens(ctx iris.Context) {
	var in SetThemeTokensRequest
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	res, err := h.svc.SetThemeTokens(ctx.Request().Context(), in.Items, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"applied": res.Applied, "reason": res.Reason})
}

// 删除主题覆盖
// @Summary      删除主题覆盖
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body SetThemeTokensRequest true "token 覆盖列表"
// @Router       /admin/customization/tokens/{token}/delete [post]
func (h *customizeHandler) DeleteThemeToken(ctx iris.Context) {
	if err := h.svc.DeleteThemeToken(ctx.Request().Context(), ctx.Params().Get("token"), writeActor(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 清空主题覆盖
// @Summary      清空主题覆盖
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/customization/tokens/clear [post]
func (h *customizeHandler) ClearThemeTokens(ctx iris.Context) {
	if err := h.svc.ClearThemeTokens(ctx.Request().Context(), writeActor(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ── 导航 ──────────────────────────────────────────────────────

// 导航项列表
// @Summary      导航项列表
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/customization/nav/list [post]
func (h *customizeHandler) ListNavItems(ctx iris.Context) {
	items, err := h.svc.ListNavItems(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 创建导航项
// @Summary      创建导航项
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body service.NavItemInput true "导航项"
// @Router       /admin/customization/nav/create [post]
func (h *customizeHandler) CreateNavItem(ctx iris.Context) {
	var in service.NavItemInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	item, res, err := h.svc.CreateNavItem(ctx.Request().Context(), in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"item": item, "applied": res.Applied, "reason": res.Reason})
}

// 更新导航项
// @Summary      更新导航项
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "导航项 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.NavItemInput true "导航项"
// @Param        payload body service.NavItemInput true "导航项"
// @Router       /admin/customization/nav/{id}/update [post]
func (h *customizeHandler) UpdateNavItem(ctx iris.Context) {
	var in service.NavItemInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	item, res, err := h.svc.UpdateNavItem(ctx.Request().Context(), ctx.Params().Get("id"), in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"item": item, "applied": res.Applied, "reason": res.Reason})
}

// 删除导航项
// @Summary      删除导航项
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "导航项 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.NavItemInput true "导航项"
// @Router       /admin/customization/nav/{id}/delete [post]
func (h *customizeHandler) DeleteNavItem(ctx iris.Context) {
	if err := h.svc.DeleteNavItem(ctx.Request().Context(), ctx.Params().Get("id"), writeActor(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ── 首页模块 ──────────────────────────────────────────────────

// 首页模块列表
// @Summary      首页模块列表
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/customization/modules/list [post]
func (h *customizeHandler) ListHomeModules(ctx iris.Context) {
	items, err := h.svc.ListHomeModules(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 创建首页模块
// @Summary      创建首页模块
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body service.HomeModuleInput true "首页模块"
// @Router       /admin/customization/modules/create [post]
func (h *customizeHandler) CreateHomeModule(ctx iris.Context) {
	var in service.HomeModuleInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	m, res, err := h.svc.CreateHomeModule(ctx.Request().Context(), in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"module": m, "applied": res.Applied, "reason": res.Reason})
}

// 更新首页模块
// @Summary      更新首页模块
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "模块 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.HomeModuleInput true "首页模块"
// @Param        payload body service.HomeModuleInput true "首页模块"
// @Router       /admin/customization/modules/{id}/update [post]
func (h *customizeHandler) UpdateHomeModule(ctx iris.Context) {
	var in service.HomeModuleInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	m, res, err := h.svc.UpdateHomeModule(ctx.Request().Context(), ctx.Params().Get("id"), in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"module": m, "applied": res.Applied, "reason": res.Reason})
}

// 删除首页模块
// @Summary      删除首页模块
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "模块 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.HomeModuleInput true "首页模块"
// @Router       /admin/customization/modules/{id}/delete [post]
func (h *customizeHandler) DeleteHomeModule(ctx iris.Context) {
	if err := h.svc.DeleteHomeModule(ctx.Request().Context(), ctx.Params().Get("id"), writeActor(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ── 自定义页 ──────────────────────────────────────────────────

// 自定义页列表
// @Summary      自定义页列表
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body ListCustomPagesRequest false "分页参数；可传空对象"
// @Router       /admin/customization/pages/list [post]
func (h *customizeHandler) ListCustomPages(ctx iris.Context) {
	var in ListCustomPagesRequest
	_ = ctx.ReadJSON(&in)
	items, total, err := h.svc.ListCustomPages(ctx.Request().Context(), in.Page, in.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, int(total))
}

// 创建自定义页
// @Summary      创建自定义页
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Param        payload body ListCustomPagesRequest false "分页参数"
// @Param        payload body service.CustomPageInput true "自定义页"
// @Router       /admin/customization/pages/create [post]
func (h *customizeHandler) CreateCustomPage(ctx iris.Context) {
	var in service.CustomPageInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	p, res, err := h.svc.CreateCustomPage(ctx.Request().Context(), in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"page": p, "applied": res.Applied, "reason": res.Reason})
}

// 更新自定义页
// @Summary      更新自定义页
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "页面 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.CustomPageInput true "自定义页"
// @Param        payload body service.CustomPageInput true "自定义页"
// @Router       /admin/customization/pages/{id}/update [post]
func (h *customizeHandler) UpdateCustomPage(ctx iris.Context) {
	var in service.CustomPageInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	p, res, err := h.svc.UpdateCustomPage(ctx.Request().Context(), ctx.Params().Get("id"), in, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"page": p, "applied": res.Applied, "reason": res.Reason})
}

// 删除自定义页
// @Summary      删除自定义页
// @Tags         站点自定义
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "页面 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body service.CustomPageInput true "自定义页"
// @Router       /admin/customization/pages/{id}/delete [post]
func (h *customizeHandler) DeleteCustomPage(ctx iris.Context) {
	if err := h.svc.DeleteCustomPage(ctx.Request().Context(), ctx.Params().Get("id"), writeActor(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 发布/取消发布自定义页
// @Summary      发布自定义页
// @Tags         站点自定义
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "页面 ID"
// @Success      200  {object}  pkg.Response
// @Param        payload body PublishCustomPageRequest true "发布开关"
// @Router       /admin/customization/pages/{id}/publish [post]
func (h *customizeHandler) PublishCustomPage(ctx iris.Context) {
	var in PublishCustomPageRequest
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	p, res, err := h.svc.PublishCustomPage(ctx.Request().Context(), ctx.Params().Get("id"), in.Publish, writeActor(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{"page": p, "applied": res.Applied, "reason": res.Reason})
}

// ── 前台 ──────────────────────────────────────────────────────

// 读取自定义页（公开）
// @Summary      读取自定义页
// @Description  只返回已发布页面；草稿表现为不存在。
// @Tags         站点自定义
// @Produce      json
// @Param        slug path string true "页面 slug"
// @Success      200  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Param        payload body PublishCustomPageRequest true "发布开关"
// @Router       /pages/{slug} [get]
func (h *customizeHandler) GetPublicPage(ctx iris.Context) {
	p, err := h.svc.GetPublishedPage(ctx.Request().Context(), ctx.Params().Get("slug"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{
		"slug":       p.Slug,
		"title":      p.Title,
		"content":    p.Content,
		"updated_at": p.UpdatedAt,
	})
}

// logCustomizeIssue 记录前台聚合的分区级失败。
//
// 前台自定义是装饰层，读失败要降级而不是 500；但降级不等于无声——
// 只有日志里能区分「站长没配」与「最近一次读挂了」。
func logCustomizeIssue(ctx iris.Context, part string, err error) {
	if err == nil {
		return
	}
	logger.FromContext(ctx.Request().Context()).
		WithComponent("customize").
		Warn("customization unavailable",
			logger.F("part", part),
			logger.F("error", err.Error()))
}

// 前台自定义聚合
// @Summary      前台自定义聚合
// @Description  返回导航、首页模块与生效片段，供首屏注入使用。已按可见性过滤。
// @Tags         站点自定义
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /customization/public [post]
func (h *customizeHandler) GetPublicCustomization(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	viewerID := currentUserID(ctx)
	isLogin := viewerID != ""
	// 真实等级：`visibility=level` 的导航与首页模块按它过滤。
	// 固定传 0 会让该可见性选项对所有人都不生效（包括站长本人），
	// 而 `TrustService.LevelOf` 本身就是廉价读（游客返回 0）。
	level := 0
	if isLogin && h.trust != nil {
		level = h.trust.LevelOf(reqCtx, viewerID)
	}

	// 五个分区相互独立，任一分区失败都不该拖垮整站外观——降级为空，
	// 但要留日志：静默空数组会让「配置没生效」变成无法排查的现象。
	nav, err := h.svc.PublicNav(reqCtx, isLogin, level)
	logCustomizeIssue(ctx, "public.nav", err)
	modules, err := h.svc.PublicHomeModules(reqCtx, isLogin, level)
	logCustomizeIssue(ctx, "public.modules", err)
	headSnippets, err := h.svc.ActiveSnippets(reqCtx, model.SnippetPositionHead)
	logCustomizeIssue(ctx, "public.snippets.head", err)
	bodySnippets, err := h.svc.ActiveSnippets(reqCtx, model.SnippetPositionBodyEnd)
	logCustomizeIssue(ctx, "public.snippets.body", err)
	tokens, err := h.svc.ThemeOverrides(reqCtx)
	logCustomizeIssue(ctx, "public.tokens", err)

	pkg.Success(ctx, map[string]any{
		"nav":           nav,
		"home_modules":  modules,
		"head_snippets": headSnippets,
		"body_snippets": bodySnippets,
		"theme_tokens":  tokens,
	})
}
