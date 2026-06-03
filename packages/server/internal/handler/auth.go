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

// Register 用户注册
// @Summary      用户注册
// @Description  注册新用户，返回 access_token、refresh_token 和用户信息
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body service.RegisterRequest true "注册信息"
// @Success      201  {object}  pkg.Response{data=service.AuthResponse}
// @Failure      400  {object}  pkg.Response
// @Failure      409  {object}  pkg.Response
// @Router       /auth/register [post]
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

// Login 用户登录
// @Summary      用户登录
// @Description  使用用户名或邮箱登录，返回 access_token、refresh_token 和用户信息
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body service.LoginRequest true "登录信息"
// @Success      200  {object}  pkg.Response{data=service.AuthResponse}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/login [post]
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

// Logout 用户登出
// @Summary      用户登出
// @Description  登出当前用户（使当前 token 失效）
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/logout [post]
func (h *authHandler) Logout(ctx iris.Context) {
	// Best-effort: mark token as expired by not blacklisting (no Redis for now)
	pkg.Success(ctx, nil)
}

// RefreshToken 刷新 access token
// @Summary      刷新 access token
// @Description  使用当前 JWT 中的用户信息刷新 access token
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{access_token=string}}
// @Failure      401  {object}  pkg.Response
// @Router       /auth/refresh [post]
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

// GetRefreshToken 使用 refresh token 换取新 access token
// @Summary      刷新令牌
// @Description  使用 refresh_token 换取新的 access_token
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body object{refresh_token=string} true "refresh token"
// @Success      200  {object}  pkg.Response{data=object{access_token=string}}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/refresh_token [post]
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
		pkg.Fail(ctx, pkg.TOKEN_WRONG, "无效的刷新令牌")
		return
	}
	newToken, err := h.svc.RefreshToken(ctx.Request().Context(), claims.Username, claims.UserUUID, claims.Role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"access_token": newToken})
}

// ChangePassword 修改密码
// @Summary      修改密码
// @Description  使用旧密码修改当前用户的密码
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{old_password=string,new_password=string} true "密码信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/change_password [post]
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

// FirstChangePassword 首次修改密码
// @Summary      首次修改密码
// @Description  管理员首次登录后修改密码，可选修改用户名
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{new_password=string,name=string} true "新密码信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/first_change_password [post]
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

// ResetPassword 重置密码
// @Summary      重置密码
// @Description  管理员重置指定用户的密码
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body object{username=string,new_password=string} true "重置信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /auth/reset_password [post]
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