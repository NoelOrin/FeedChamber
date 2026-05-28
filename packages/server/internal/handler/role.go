package handler

import (
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/google/uuid"
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

func (h *roleHandler) List(ctx iris.Context) {
	roles, err := h.repo.List(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, roles)
}

func (h *roleHandler) Create(ctx iris.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	role := &model.Role{
		ID:   uuid.New().String(),
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