package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type UserHandler interface {
	GetProfile(ctx iris.Context)
	GetByID(ctx iris.Context)
	List(ctx iris.Context)
	UpdateRole(ctx iris.Context)
	Delete(ctx iris.Context)
}

type userHandler struct {
	svc service.UserService
}

func newUserHandler(svc service.UserService) UserHandler {
	return &userHandler{svc: svc}
}

func (h *userHandler) GetProfile(ctx iris.Context) {
	userUUID := ctx.Values().GetString("user_uuid")
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	user, err := h.svc.GetByUUID(ctx.Request().Context(), userUUID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, user)
}

func (h *userHandler) GetByID(ctx iris.Context) {
	user, err := h.svc.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, user)
}

func (h *userHandler) List(ctx iris.Context) {
	page := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	users, err := h.svc.List(ctx.Request().Context(), page, pageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"list": users})
}

func (h *userHandler) UpdateRole(ctx iris.Context) {
	var req struct {
		Role string `json:"role"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.UpdateRole(ctx.Request().Context(), ctx.Params().Get("id"), req.Role); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

func (h *userHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}