package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type GroupHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	ListMembers(ctx iris.Context)
	AddMember(ctx iris.Context)
	RemoveMember(ctx iris.Context)
	GetPermissions(ctx iris.Context)
	SyncPermissions(ctx iris.Context)
}

type groupHandler struct {
	svc service.GroupService
}

func newGroupHandler(svc service.GroupService) GroupHandler {
	return &groupHandler{svc: svc}
}

// List 用户组列表
// @Summary      用户组列表
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /groups/list [post]
func (h *groupHandler) List(ctx iris.Context) {
	groups, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, groups)
}

// Create 创建用户组
// @Summary      创建用户组
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/create [post]
func (h *groupHandler) Create(ctx iris.Context) {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Visibility  string `json:"visibility"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	g, err := h.svc.Create(ctx.Request().Context(), req.Name, req.DisplayName, req.Visibility)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, g)
}

// Update 更新用户组
// @Summary      更新用户组
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/{id}/update [post]
func (h *groupHandler) Update(ctx iris.Context) {
	var req struct {
		DisplayName string `json:"display_name"`
		Visibility  string `json:"visibility"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	g, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), req.DisplayName, req.Visibility)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, g)
}

// Delete 删除用户组
// @Summary      删除用户组
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/{id}/delete [post]
func (h *groupHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ListMembers 组成员列表
// @Summary      组成员列表
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/{id}/members/list [post]
func (h *groupHandler) ListMembers(ctx iris.Context) {
	var body struct {
		ListParams
	}
	_ = ctx.ReadJSON(&body)
	body.Normalize()
	members, total, err := h.svc.ListMembers(ctx.Request().Context(), ctx.Params().Get("id"), body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, members, total)
}

// AddMember 添加组成员
// @Summary      添加组成员
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/{id}/members/add [post]
func (h *groupHandler) AddMember(ctx iris.Context) {
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.AddMember(ctx.Request().Context(), ctx.Params().Get("id"), req.UserID, req.Role); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// RemoveMember 移除组成员
// @Summary      移除组成员
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/{id}/members/remove [post]
func (h *groupHandler) RemoveMember(ctx iris.Context) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.RemoveMember(ctx.Request().Context(), ctx.Params().Get("id"), req.UserID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// GetPermissions 获取组权限
// @Summary      获取组权限
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/{id}/permissions [post]
func (h *groupHandler) GetPermissions(ctx iris.Context) {
	codes, err := h.svc.GetGroupPermissions(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{
		"group_id":    ctx.Params().Get("id"),
		"permissions": codes,
	})
}

// SyncPermissions 同步组权限
// @Summary      同步组权限
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /groups/{id}/permissions/sync [post]
func (h *groupHandler) SyncPermissions(ctx iris.Context) {
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.SyncGroupPermissions(ctx.Request().Context(), ctx.Params().Get("id"), req.Permissions); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]any{
		"group_id":    ctx.Params().Get("id"),
		"permissions": req.Permissions,
	})
}
