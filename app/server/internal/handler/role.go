package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type RoleHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Delete(ctx iris.Context)
	Update(ctx iris.Context)
}

type roleHandler struct {
	svc service.RoleService
}

func newRoleHandler(svc service.RoleService) RoleHandler {
	return &roleHandler{svc: svc}
}

// List 角色列表
// @Summary      角色列表
// @Description  获取所有角色列表
// @Tags         角色
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.Role}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /role/list [post]
func (h *roleHandler) List(ctx iris.Context) {
	roles, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, roles)
}

// Create 创建角色
// @Summary      创建角色
// @Description  创建新角色
// @Tags         角色
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,display_name=string,description=string} true "角色信息"
// @Success      201  {object}  pkg.Response{data=model.Role}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      409  {object}  pkg.Response
// @Router       /role/create [post]
func (h *roleHandler) Create(ctx iris.Context) {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	role, err := h.svc.Create(ctx.Request().Context(), req.Name, req.DisplayName)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, role)
}

// Delete 删除角色
// @Summary      删除角色
// @Description  删除指定角色
// @Tags         角色
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "角色 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /role/{id}/delete [post]
func (h *roleHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Update 更新角色
// @Summary      更新角色
// @Description  更新角色名称、显示名称和描述
// @Tags         角色
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,display_name=string,description=string} true "角色信息"
// @Success      200  {object}  pkg.Response{data=model.Role}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /role/{id}/update [post]
func (h *roleHandler) Update(ctx iris.Context) {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	role, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), req.Name, req.DisplayName, req.Description)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, role)
}
