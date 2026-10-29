package handler

import (
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// CustomizeHandler 是站点自定义层的 HTTP 入口（后台）。
//
// 注意：这里的每个写方法都构造 WriteSourceUI 的 actor——后台是人工操作，
// 高风险内容按请求的启用状态写入。MCP 走的是同一组 service 方法，但传
// WriteSourceMachine，由 service 层的 decideHighRisk 决定是否强制草稿。
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

type customizeHandler struct{ svc service.CustomizeService }

func newCustomizeHandler(svc service.CustomizeService) CustomizeHandler {
	return &customizeHandler{svc: svc}
}

// uiActor 构造后台人工操作的写入上下文。
func uiActor(ctx iris.Context) service.WriteActor {
	return service.WriteActor{
		UserID: currentUserID(ctx),
		Source: service.WriteSourceUI,
		// 后台人工操作不受「机器凭证」限制，CanPublish 不再参与判定。
		CanPublish: true,
	}
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
// @Router       /admin/customization/snippets/create [post]
func (h *customizeHandler) CreateSnippet(ctx iris.Context) {
	var in service.SnippetInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	snippet, res, err := h.svc.CreateSnippet(ctx.Request().Context(), in, uiActor(ctx))
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
// @Router       /admin/customization/snippets/{id}/update [post]
func (h *customizeHandler) UpdateSnippet(ctx iris.Context) {
	id := ctx.Params().Get("id")
	var in service.SnippetInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	snippet, res, err := h.svc.UpdateSnippet(ctx.Request().Context(), id, in, uiActor(ctx))
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
// @Router       /admin/customization/snippets/{id}/delete [post]
func (h *customizeHandler) DeleteSnippet(ctx iris.Context) {
	if err := h.svc.DeleteSnippet(ctx.Request().Context(), ctx.Params().Get("id"), uiActor(ctx)); err != nil {
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
// @Router       /admin/customization/snippets/{id}/toggle [post]
func (h *customizeHandler) ToggleSnippet(ctx iris.Context) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.SetSnippetEnabled(ctx.Request().Context(), ctx.Params().Get("id"), in.Enabled, uiActor(ctx)); err != nil {
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
// @Router       /admin/customization/tokens/set [post]
func (h *customizeHandler) SetThemeTokens(ctx iris.Context) {
	var in struct {
		Items []service.ThemeTokenInput `json:"items"`
	}
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	res, err := h.svc.SetThemeTokens(ctx.Request().Context(), in.Items, uiActor(ctx))
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
// @Router       /admin/customization/tokens/{token}/delete [post]
func (h *customizeHandler) DeleteThemeToken(ctx iris.Context) {
	if err := h.svc.DeleteThemeToken(ctx.Request().Context(), ctx.Params().Get("token"), uiActor(ctx)); err != nil {
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
	if err := h.svc.ClearThemeTokens(ctx.Request().Context(), uiActor(ctx)); err != nil {
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
// @Router       /admin/customization/nav/create [post]
func (h *customizeHandler) CreateNavItem(ctx iris.Context) {
	var in service.NavItemInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	item, res, err := h.svc.CreateNavItem(ctx.Request().Context(), in, uiActor(ctx))
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
// @Router       /admin/customization/nav/{id}/update [post]
func (h *customizeHandler) UpdateNavItem(ctx iris.Context) {
	var in service.NavItemInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	item, res, err := h.svc.UpdateNavItem(ctx.Request().Context(), ctx.Params().Get("id"), in, uiActor(ctx))
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
// @Router       /admin/customization/nav/{id}/delete [post]
func (h *customizeHandler) DeleteNavItem(ctx iris.Context) {
	if err := h.svc.DeleteNavItem(ctx.Request().Context(), ctx.Params().Get("id"), uiActor(ctx)); err != nil {
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
// @Router       /admin/customization/modules/create [post]
func (h *customizeHandler) CreateHomeModule(ctx iris.Context) {
	var in service.HomeModuleInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	m, res, err := h.svc.CreateHomeModule(ctx.Request().Context(), in, uiActor(ctx))
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
// @Router       /admin/customization/modules/{id}/update [post]
func (h *customizeHandler) UpdateHomeModule(ctx iris.Context) {
	var in service.HomeModuleInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	m, res, err := h.svc.UpdateHomeModule(ctx.Request().Context(), ctx.Params().Get("id"), in, uiActor(ctx))
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
// @Router       /admin/customization/modules/{id}/delete [post]
func (h *customizeHandler) DeleteHomeModule(ctx iris.Context) {
	if err := h.svc.DeleteHomeModule(ctx.Request().Context(), ctx.Params().Get("id"), uiActor(ctx)); err != nil {
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
// @Router       /admin/customization/pages/list [post]
func (h *customizeHandler) ListCustomPages(ctx iris.Context) {
	var in struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
	}
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
// @Router       /admin/customization/pages/create [post]
func (h *customizeHandler) CreateCustomPage(ctx iris.Context) {
	var in service.CustomPageInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	p, res, err := h.svc.CreateCustomPage(ctx.Request().Context(), in, uiActor(ctx))
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
// @Router       /admin/customization/pages/{id}/update [post]
func (h *customizeHandler) UpdateCustomPage(ctx iris.Context) {
	var in service.CustomPageInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	p, res, err := h.svc.UpdateCustomPage(ctx.Request().Context(), ctx.Params().Get("id"), in, uiActor(ctx))
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
// @Router       /admin/customization/pages/{id}/delete [post]
func (h *customizeHandler) DeleteCustomPage(ctx iris.Context) {
	if err := h.svc.DeleteCustomPage(ctx.Request().Context(), ctx.Params().Get("id"), uiActor(ctx)); err != nil {
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
// @Router       /admin/customization/pages/{id}/publish [post]
func (h *customizeHandler) PublishCustomPage(ctx iris.Context) {
	var in struct {
		Publish bool `json:"publish"`
	}
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	p, res, err := h.svc.PublishCustomPage(ctx.Request().Context(), ctx.Params().Get("id"), in.Publish, uiActor(ctx))
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

// 前台自定义聚合
// @Summary      前台自定义聚合
// @Description  返回导航、首页模块与生效片段，供首屏注入使用。已按可见性过滤。
// @Tags         站点自定义
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /customization/public [post]
func (h *customizeHandler) GetPublicCustomization(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	isLogin := currentUserID(ctx) != ""
	// 游客按 TL0；登录用户的等级在网关阶段未加载，这里用 0 兜底，
	// 前台导航的等级过滤因此对登录用户按最低等级处理——宁可少显示一项，
	// 也不要因为等级判定失败而多显示。
	level := 0

	nav, _ := h.svc.PublicNav(reqCtx, isLogin, level)
	modules, _ := h.svc.PublicHomeModules(reqCtx, isLogin, level)
	headSnippets, _ := h.svc.ActiveSnippets(reqCtx, model.SnippetPositionHead)
	bodySnippets, _ := h.svc.ActiveSnippets(reqCtx, model.SnippetPositionBodyEnd)
	tokens, _ := h.svc.ThemeOverrides(reqCtx)

	pkg.Success(ctx, map[string]any{
		"nav":           nav,
		"home_modules":  modules,
		"head_snippets": headSnippets,
		"body_snippets": bodySnippets,
		"theme_tokens":  tokens,
	})
}
