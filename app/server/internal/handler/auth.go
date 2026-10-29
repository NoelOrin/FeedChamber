package handler

import (
	"net/mail"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type AuthHandler interface {
	Register(ctx iris.Context)
	ApproveRegistration(ctx iris.Context)
	Login(ctx iris.Context)
	RefreshToken(ctx iris.Context)
	ChangePassword(ctx iris.Context)
	FirstChangePassword(ctx iris.Context)
	ResetPassword(ctx iris.Context)
	ForgotPassword(ctx iris.Context)
	ResetPasswordByCode(ctx iris.Context)
	SendVerifyEmail(ctx iris.Context)
	VerifyEmail(ctx iris.Context)
}

type authHandler struct {
	svc      service.AuthService
	sessions service.SessionStore
	emailSvc service.EmailService
	audit    service.AuditLogService
}

func newAuthHandler(svc service.AuthService, sessions service.SessionStore, emailSvc service.EmailService, audit service.AuditLogService) AuthHandler {
	return &authHandler{svc: svc, sessions: sessions, emailSvc: emailSvc, audit: audit}
}

// Register 用户注册
// @Summary      用户注册
// @Description  注册新用户，签发服务端会话；返回 access_token（会话 id）和用户信息
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
	req.IP = ctx.RemoteAddr()
	req.UserAgent = ctx.Request().UserAgent()
	resp, err := h.svc.Register(ctx.Request().Context(), &req)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	// access_token 现在是不透明的会话 id；同时下发 HttpOnly master_session cookie。
	pkg.SetSessionCookie(ctx, resp.Token, model.SessionTTL)
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, resp)
}

// Login 用户登录
// @Summary      用户登录
// @Description  使用用户名或邮箱登录，签发服务端会话；返回 access_token（会话 id）和用户信息
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
	req.IP = ctx.RemoteAddr()
	req.UserAgent = ctx.Request().UserAgent()
	resp, err := h.svc.Login(ctx.Request().Context(), &req)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SetSessionCookie(ctx, resp.Token, model.SessionTTL)
	pkg.Success(ctx, resp)
}

// RefreshToken 轮换会话 id
// @Summary      刷新会话
// @Description  签发新会话 id、吊销旧的（rotation）；响应体 access_token 为新 id
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{access_token=string}}
// @Failure      401  {object}  pkg.Response
// @Router       /auth/refresh [post]
func (h *authHandler) RefreshToken(ctx iris.Context) {
	sess := currentSession(ctx)
	if sess == nil || h.sessions == nil {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	fresh, err := h.sessions.Rotate(ctx.Request().Context(), sess)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SetSessionCookie(ctx, fresh.SessionID, model.SessionTTL)
	pkg.Success(ctx, iris.Map{"access_token": fresh.SessionID})
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
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.ChangePassword(ctx.Request().Context(), u.Username, req.OldPassword, req.NewPassword); err != nil {
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
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.FirstChangePassword(ctx.Request().Context(), u.Username, req.NewPassword, req.Name); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ResetPassword 重置密码
// @Summary      重置密码（管理员）
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

// ForgotPassword 发送密码重置验证码
// @Summary      发送重置密码验证码
// @Description  向指定邮箱发送 6 位数字验证码，无论邮箱是否存在均返回成功（防枚举）
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body object{email=string} true "邮箱"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      429  {object}  pkg.Response
// @Router       /auth/forgot_password [post]
func (h *authHandler) ForgotPassword(ctx iris.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Email == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "邮箱不能为空")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "邮箱格式不正确")
		return
	}
	// Always return success regardless of whether the email exists (防枚举).
	_ = h.emailSvc.SendCode(ctx.Request().Context(), service.PurposeResetPassword, req.Email)
	pkg.Success(ctx, nil)
}

// ResetPasswordByCode 通过邮箱验证码重置密码
// @Summary      通过验证码重置密码
// @Summary      通过验证码重置密码
// @Description  校验邮箱验证码后重置用户密码
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body object{email=string,code=string,new_password=string} true "重置信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Router       /auth/reset_password_by_code [post]
func (h *authHandler) ResetPasswordByCode(ctx iris.Context) {
	var req struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Email == "" || req.Code == "" || req.NewPassword == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "邮箱、验证码和新密码均不能为空")
		return
	}
	if err := h.emailSvc.VerifyCode(ctx.Request().Context(), service.PurposeResetPassword, req.Email, req.Code); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if err := h.svc.ResetPasswordByEmail(ctx.Request().Context(), req.Email, req.NewPassword); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// SendVerifyEmail 发送邮箱验证码
// @Router /auth/send_verify_email [post]
func (h *authHandler) SendVerifyEmail(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.SendVerifyEmail(ctx.Request().Context(), u.ID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// VerifyEmail 验证邮箱
// @Router /auth/verify_email [post]
func (h *authHandler) VerifyEmail(ctx iris.Context) {
	var body struct {
		Code string `json:"code"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.VerifyEmail(ctx.Request().Context(), u.ID, body.Code); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ApproveRegistration 审批待审核用户
// @Summary      审批注册
// @Description  将 pending_user 角色审批为 user
// @Tags         管理
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "用户 ID"
// @Success      200  {object}  pkg.Response
// @Router       /admin/users/{id}/approve_registration [post]
func (h *authHandler) ApproveRegistration(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "缺少用户 ID")
		return
	}
	if err := h.svc.ApproveRegistration(ctx.Request().Context(), id); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "admin.approve_registration", id, "")
	pkg.Success(ctx, nil)
}
