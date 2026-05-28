package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type AuthHandler interface {
	Register(ctx iris.Context)
	Login(ctx iris.Context)
	Logout(ctx iris.Context)
	RefreshToken(ctx iris.Context)
	GetRefreshToken(ctx iris.Context)
	ChangePassword(ctx iris.Context)
	FirstChangePassword(ctx iris.Context)
	ResetPassword(ctx iris.Context)
}

type authHandler struct {
	svc service.AuthService
}

func newAuthHandler(svc service.AuthService) AuthHandler {
	return &authHandler{svc: svc}
}

func (h *authHandler) Register(ctx iris.Context) {
	var req service.RegisterRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.svc.Register(ctx.Request().Context(), &req)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, resp)
}

func (h *authHandler) Login(ctx iris.Context) {
	var req service.LoginRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.svc.Login(ctx.Request().Context(), &req)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, resp)
}

func (h *authHandler) Logout(ctx iris.Context) {
	// Best-effort: mark token as expired by not blacklisting (no Redis for now)
	pkg.Success(ctx, nil)
}

func (h *authHandler) RefreshToken(ctx iris.Context) {
	username := ctx.Values().GetString("username")
	userUUID := ctx.Values().GetString("user_uuid")
	role := ctx.Values().GetString("user_role")
	if username == "" || userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	token, err := h.svc.RefreshToken(ctx.Request().Context(), username, userUUID, role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"access_token": token})
}

func (h *authHandler) GetRefreshToken(ctx iris.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	claims, err := pkg.ParseToken(req.RefreshToken)
	if err != nil {
		pkg.Fail(ctx, pkg.TOKEN_WRONG, "invalid refresh token")
		return
	}
	newToken, err := h.svc.RefreshToken(ctx.Request().Context(), claims.Username, claims.UserUUID, claims.Role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"access_token": newToken})
}

func (h *authHandler) ChangePassword(ctx iris.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	username := ctx.Values().GetString("username")
	if username == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	if err := h.svc.ChangePassword(ctx.Request().Context(), username, req.OldPassword, req.NewPassword); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

func (h *authHandler) FirstChangePassword(ctx iris.Context) {
	var req struct {
		NewPassword string  `json:"new_password"`
		Name        *string `json:"name"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	username := ctx.Values().GetString("username")
	if username == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	if err := h.svc.FirstChangePassword(ctx.Request().Context(), username, req.NewPassword, req.Name); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

func (h *authHandler) ResetPassword(ctx iris.Context) {
	var req struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.ResetPassword(ctx.Request().Context(), req.Username, req.NewPassword); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}