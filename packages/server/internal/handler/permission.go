package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type PermissionHandler interface {
	ListPermissions(ctx iris.Context)
	GetRolePermissions(ctx iris.Context)
	SyncRolePermissions(ctx iris.Context)
}

type permissionHandler struct {
	permSvc service.PermissionService
}

func newPermissionHandler(permSvc service.PermissionService) PermissionHandler {
	return &permissionHandler{permSvc: permSvc}
}

// ListPermissions 权限列表
// @Summary      权限列表
// @Description  获取所有权限定义列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.Permission}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /permission/list [get]
func (h *permissionHandler) ListPermissions(ctx iris.Context) {
	perms, err := h.permSvc.ListPermissions(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, perms)
}

// GetRolePermissions 获取角色权限
// @Summary      获取角色权限
// @Description  获取指定角色的权限码列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        role query     string  true  "角色名称"
// @Success      200  {object}  pkg.Response{data=object{role=string,permissions=[]string}}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /permission/role [get]
func (h *permissionHandler) GetRolePermissions(ctx iris.Context) {
	var req struct {
		Role string `json:"role"`
	}
	_ = ctx.ReadJSON(&req)
	if req.Role == "" {
		req.Role = ctx.URLParam("role")
	}
	if req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "角色不能为空")
		return
	}
	codes := h.permSvc.GetRolePermissions(req.Role)
	pkg.Success(ctx, iris.Map{
		"role":        req.Role,
		"permissions": codes,
	})
}

// SyncRolePermissions 同步角色权限
// @Summary      同步角色权限
// @Description  为指定角色同步设置权限列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{role=string,permissions=[]string} true "角色权限信息"
// @Success      200  {object}  pkg.Response{data=object{role=string,permissions=[]string}}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /permission/sync [post]
func (h *permissionHandler) SyncRolePermissions(ctx iris.Context) {
	var req struct {
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "角色不能为空")
		return
	}

	if err := h.permSvc.SyncRolePermissions(ctx.Request().Context(), req.Role, req.Permissions); err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	pkg.Success(ctx, iris.Map{
		"role":        req.Role,
		"permissions": req.Permissions,
	})
}
