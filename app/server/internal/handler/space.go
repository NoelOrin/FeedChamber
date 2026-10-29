package handler

import (
	"fmt"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type SpaceHandler interface {
	Tree(ctx iris.Context)
	Flat(ctx iris.Context)
	Get(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	ToggleActive(ctx iris.Context)
	Children(ctx iris.Context)
	Ancestors(ctx iris.Context)
	Breadcrumb(ctx iris.Context)
	Descendants(ctx iris.Context)
	SubTree(ctx iris.Context)
	Move(ctx iris.Context)
	Reorder(ctx iris.Context)
	Roots(ctx iris.Context)
}

type spaceHandler struct {
	svc   service.SpaceService
	trust service.TrustService
}

func newSpaceHandler(svc service.SpaceService, trust service.TrustService) SpaceHandler {
	return &spaceHandler{svc: svc, trust: trust}
}

// Tree 获取版面树
// @Summary      获取版面树
// @Description  获取所有版面的树形结构列表（NGA 层级）
// @Tags         版面
// @Accept       json
// @Produce      json
// @Success      200  {object}  pkg.Response{data=[]model.SpaceNode}
// @Router       /spaces/tree [post]
func (h *spaceHandler) Tree(ctx iris.Context) {
	nodes, err := h.svc.Tree(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if nodes == nil {
		nodes = []*model.SpaceNode{}
	}
	pkg.Success(ctx, nodes)
}

// Flat 获取版面平铺列表
// @Summary      获取版面平铺列表
// @Tags         版面
// @Accept       json
// @Produce      json
// @Success      200  {object}  pkg.Response{data=[]model.Space}
// @Router       /spaces/flat [post]
func (h *spaceHandler) Flat(ctx iris.Context) {
	spaces, err := h.svc.Flat(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if spaces == nil {
		spaces = []model.Space{}
	}
	pkg.Success(ctx, spaces)
}

// Roots 获取根版面列表
// @Summary      获取根版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Success      200  {object}  pkg.Response{data=[]model.Space}
// @Router       /spaces/roots [post]
func (h *spaceHandler) Roots(ctx iris.Context) {
	spaces, err := h.svc.ListRoots(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if spaces == nil {
		spaces = []model.Space{}
	}
	pkg.Success(ctx, spaces)
}

// Get 获取单个版面
// @Summary      获取版面详情
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response{data=model.Space}
// @Router       /spaces/{space_id} [post]
func (h *spaceHandler) Get(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	space, err := h.svc.GetByID(reqCtx, ctx.Params().Get("space_id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	// 版面可见性统一下沉到可见性谓词：public 放行、login 要求登录、
	// level 要求等级 ≥ min_level（含祖先继承）。
	idx, err := h.svc.VisibilityIndex(reqCtx)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	effective := idx.Resolve(space.ID)
	viewerID := currentUserID(ctx)
	level := 0
	if viewerID != "" && h.trust != nil {
		level = h.trust.LevelOf(reqCtx, viewerID)
	}
	if !effective.Allows(level, viewerID != "") {
		if effective.Visibility == model.SpaceVisibilityLogin {
			pkg.FailStatus(ctx, 403, pkg.FORBIDDEN, "该版面需要登录后访问")
		} else {
			pkg.FailStatus(ctx, 403, pkg.FORBIDDEN, fmt.Sprintf("需要等级 %d，当前等级 %d", effective.RequiredLevel(), level))
		}
		return
	}
	pkg.Success(ctx, space)
}

// Create 创建版面
// @Summary      创建版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        body body object{name=string,slug=string,description=string,icon=string,parent_id=string,sort_order=int} true "版面信息"
// @Success      201  {object}  pkg.Response{data=model.Space}
// @Router       /spaces/create [post]
func (h *spaceHandler) Create(ctx iris.Context) {
	var body struct {
		Name        string `json:"name"`
		Visibility  string `json:"visibility"`
		MinLevel    int    `json:"min_level"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Slug        string `json:"slug"`
		ParentID    string `json:"parent_id"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	space, err := h.svc.Create(ctx.Request().Context(), body.Name, body.Description, body.Icon, body.Slug, body.ParentID, body.SortOrder, body.Visibility, body.MinLevel)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, space)
}

// Update 更新版面（支持部分更新，parent_id 缺省表示不移动）
// @Summary      更新版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Param        body body object{name=string,slug=string,description=string,icon=string,parent_id=string,sort_order=int,is_active=boolean} true "更新信息（仅传需变更字段）"
// @Success      200  {object}  pkg.Response{data=model.Space}
// @Router       /spaces/{space_id}/update [post]
func (h *spaceHandler) Update(ctx iris.Context) {
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Icon        *string `json:"icon"`
		Slug        *string `json:"slug"`
		ParentID    *string `json:"parent_id"`
		SortOrder   *int    `json:"sort_order"`
		IsActive    *bool   `json:"is_active"`
		Visibility  *string `json:"visibility"`
		MinLevel    *int    `json:"min_level"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	id := ctx.Params().Get("space_id")
	existing, err := h.svc.GetByID(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	name := existing.Name
	if body.Name != nil {
		name = *body.Name
	}
	desc := existing.Description
	if body.Description != nil {
		desc = *body.Description
	}
	icon := existing.Icon
	if body.Icon != nil {
		icon = *body.Icon
	}
	slug := existing.Slug
	if body.Slug != nil {
		slug = *body.Slug
	}
	parentID := existing.ParentID
	if body.ParentID != nil {
		parentID = *body.ParentID
	}
	sortOrder := existing.SortOrder
	if body.SortOrder != nil {
		sortOrder = *body.SortOrder
	}
	isActive := existing.IsActive
	if body.IsActive != nil {
		isActive = *body.IsActive
	}
	visibility := existing.Visibility
	if visibility == "" {
		visibility = model.SpaceVisibilityPublic
	}
	if body.Visibility != nil {
		visibility = *body.Visibility
	}
	minLevel := existing.MinLevel
	if body.MinLevel != nil {
		minLevel = *body.MinLevel
	}
	space, err := h.svc.Update(ctx.Request().Context(), id, name, desc, icon, slug, parentID, sortOrder, isActive, visibility, minLevel)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, space)
}

// Delete 删除版面
// @Summary      删除版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response
// @Router       /spaces/{space_id}/delete [post]
func (h *spaceHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("space_id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ToggleActive 切换版面启用状态
// @Summary      切换版面状态
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Param        body body object{is_active=boolean} false "启用状态（不传则切换）"
// @Success      200  {object}  pkg.Response{data=model.Space}
// @Router       /spaces/{space_id}/status [post]
func (h *spaceHandler) ToggleActive(ctx iris.Context) {
	var body struct {
		IsActive *bool `json:"is_active"`
	}
	raw := ctx.Params().Get("space_id")
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if body.IsActive == nil {
		space, err := h.svc.ToggleActive(ctx.Request().Context(), raw)
		if err != nil {
			pkg.HandleError(ctx, err)
			return
		}
		pkg.Success(ctx, space)
		return
	}
	space, err := h.svc.GetByID(ctx.Request().Context(), raw)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	updated, err := h.svc.Update(ctx.Request().Context(), space.ID, space.Name, space.Description, space.Icon, space.Slug, space.ParentID, space.SortOrder, *body.IsActive, space.Visibility, space.MinLevel)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, updated)
}

// Children 获取子版面
// @Summary      获取直接子版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "父版面 ID"
// @Success      200  {object}  pkg.Response{data=[]model.Space}
// @Router       /spaces/{space_id}/children [post]
func (h *spaceHandler) Children(ctx iris.Context) {
	id := ctx.Params().Get("space_id")
	children, err := h.svc.GetChildren(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if children == nil {
		children = []model.Space{}
	}
	pkg.Success(ctx, children)
}

// Ancestors 获取祖先链
// @Summary      获取祖先版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response{data=[]model.Space}
// @Router       /spaces/{space_id}/ancestors [post]
func (h *spaceHandler) Ancestors(ctx iris.Context) {
	id := ctx.Params().Get("space_id")
	ancestors, err := h.svc.GetAncestors(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if ancestors == nil {
		ancestors = []model.Space{}
	}
	pkg.Success(ctx, ancestors)
}

// Breadcrumb 获取面包屑
// @Summary      获取版面面包屑
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response{data=[]model.Space}
// @Router       /spaces/{space_id}/breadcrumb [post]
func (h *spaceHandler) Breadcrumb(ctx iris.Context) {
	id := ctx.Params().Get("space_id")
	breadcrumb, err := h.svc.GetBreadcrumb(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if breadcrumb == nil {
		breadcrumb = []model.Space{}
	}
	pkg.Success(ctx, breadcrumb)
}

// Descendants 获取所有后代
// @Summary      获取后代版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response{data=[]model.Space}
// @Router       /spaces/{space_id}/descendants [post]
func (h *spaceHandler) Descendants(ctx iris.Context) {
	id := ctx.Params().Get("space_id")
	desc, err := h.svc.GetDescendants(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if desc == nil {
		desc = []model.Space{}
	}
	pkg.Success(ctx, desc)
}

// SubTree 获取子树
// @Summary      获取版面子树
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response{data=model.SpaceNode}
// @Router       /spaces/{space_id}/subtree [post]
func (h *spaceHandler) SubTree(ctx iris.Context) {
	id := ctx.Params().Get("space_id")
	node, err := h.svc.GetSubTree(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, node)
}

// Move 移动版面
// @Summary      移动版面
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        space_id  path      string  true  "版面 ID"
// @Param        body body object{parent_id=string,sort_order=int} true "目标父级"
// @Success      200  {object}  pkg.Response{data=model.Space}
// @Router       /spaces/{space_id}/move [post]
func (h *spaceHandler) Move(ctx iris.Context) {
	var body struct {
		ParentID  *string `json:"parent_id"`
		SortOrder *int    `json:"sort_order"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if body.ParentID == nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "parent_id 不能为空（传空字符串表示移到根）")
		return
	}
	space, err := h.svc.Move(ctx.Request().Context(), ctx.Params().Get("space_id"), *body.ParentID, body.SortOrder)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, space)
}

// Reorder 同级排序
// @Summary      版面同级重排
// @Tags         版面
// @Accept       json
// @Produce      json
// @Param        body body object{parent_id=string,ordered_ids=[]string} true "排序"
// @Success      200  {object}  pkg.Response
// @Router       /spaces/reorder [post]
func (h *spaceHandler) Reorder(ctx iris.Context) {
	var body struct {
		ParentID   string   `json:"parent_id"`
		OrderedIDs []string `json:"ordered_ids"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.Reorder(ctx.Request().Context(), body.ParentID, body.OrderedIDs); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
