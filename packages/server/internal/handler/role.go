package handler

import (
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/kataras/iris/v12"
)

type RoleHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Delete(ctx iris.Context)
}

type roleHandler struct {
	repo repository.RoleRepository
}

func newRoleHandler(repo repository.RoleRepository) RoleHandler {
	return &roleHandler{repo: repo}
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
// @Router       /role/list [get]
func (h *roleHandler) List(ctx iris.Context) {
	roles, err := h.repo.List(ctx.Request().Context())
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
// @Param        body body object{name=string} true "角色信息"
// @Success      201  {object}  pkg.Response{data=model.Role}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      409  {object}  pkg.Response
// @Router       /role/create [post]
func (h *roleHandler) Create(ctx iris.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	role := &model.Role{
		ID:   pkg.NewID(),
		Name: req.Name,
	}
	if err := h.repo.Create(ctx.Request().Context(), role); err != nil {
		pkg.Fail(ctx, pkg.ALREADY_EXISTS, "role already exists")
		return
	}
	// Reload role cache
	if roles, err := h.repo.List(ctx.Request().Context()); err == nil {
		model.LoadRoleCache(roles)
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
// @Router       /role/{id} [delete]
func (h *roleHandler) Delete(ctx iris.Context) {
	if err := h.repo.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	// Reload role cache
	if roles, err := h.repo.List(ctx.Request().Context()); err == nil {
		model.LoadRoleCache(roles)
	}
	pkg.Success(ctx, nil)
}