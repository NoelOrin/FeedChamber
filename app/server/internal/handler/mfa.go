package handler

import (
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type MFAHandler interface {
	SetupTOTP(ctx iris.Context)
	EnableTOTP(ctx iris.Context)
	DisableTOTP(ctx iris.Context)
	RegenerateBackup(ctx iris.Context)
	VerifyLogin(ctx iris.Context)
}

type mfaHandler struct {
	svc      service.MFAService
	authSvc  service.AuthService
	userSvc  service.UserService
	sessions service.SessionStore
}

func newMFAHandler(svc service.MFAService, authSvc service.AuthService, userSvc service.UserService, sessions service.SessionStore) MFAHandler {
	return &mfaHandler{svc: svc, authSvc: authSvc, userSvc: userSvc, sessions: sessions}
}

// 初始化 TOTP 绑定
// @Summary      初始化 TOTP 绑定
// @Description  生成 TOTP 密钥与绑定信息（尚未启用）。
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/mfa/totp/setup [post]
func (h *mfaHandler) SetupTOTP(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	res, err := h.svc.SetupTOTP(ctx.Request().Context(), u.ID, u.Username)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, res)
}

// 启用 TOTP
// @Summary      启用 TOTP
// @Description  校验动态码后启用 TOTP 双因素认证。
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{code=string} true "6 位动态码"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/mfa/totp/enable [post]
func (h *mfaHandler) EnableTOTP(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := ctx.ReadJSON(&body); err != nil || body.Code == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	codes, err := h.svc.EnableTOTP(ctx.Request().Context(), u.ID, body.Code)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"backup_codes": codes})
}

// 禁用 TOTP
// @Summary      禁用 TOTP
// @Description  关闭 TOTP 双因素认证。
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{code=string} true "6 位动态码"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/mfa/totp/disable [post]
func (h *mfaHandler) DisableTOTP(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := ctx.ReadJSON(&body); err != nil || body.Code == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	if err := h.svc.DisableTOTP(ctx.Request().Context(), u.ID, body.Code); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 重新生成备份码
// @Summary      重新生成备份码
// @Description  作废旧备份码并生成一组新的备用登录码。
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/mfa/backup/regenerate [post]
func (h *mfaHandler) RegenerateBackup(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := ctx.ReadJSON(&body); err != nil || body.Code == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	codes, err := h.svc.RegenerateBackupCodes(ctx.Request().Context(), u.ID, body.Code)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"backup_codes": codes})
}

// MFA 登录校验
// @Summary      MFA 登录校验
// @Description  登录第二步：提交 TOTP 动态码或备份码换取会话。该端点公开且限流 20/分钟。
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Param        body body object{mfa_token=string,code=string} true "MFA 令牌与动态码"
// @Success      200  {object}  pkg.Response
// @Router       /auth/mfa/verify [post]
// VerifyLogin completes MFA challenge and issues tokens.
func (h *mfaHandler) VerifyLogin(ctx iris.Context) {
	var body struct {
		MFAToken string `json:"mfa_token"`
		Code     string `json:"code"`
	}
	if err := ctx.ReadJSON(&body); err != nil || body.MFAToken == "" || body.Code == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	userID, err := h.svc.VerifyLoginChallenge(ctx.Request().Context(), body.MFAToken, body.Code)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	u, err := h.userSvc.GetByID(ctx.Request().Context(), userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	// VerifyLogin 完成即视为登录成功：签发服务端会话（与 /auth/login 同构）。
	if h.sessions == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "session store unavailable")
		return
	}
	sess, err := h.sessions.Issue(ctx.Request().Context(), u, ctx.RemoteAddr(), ctx.Request().UserAgent())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SetSessionCookie(ctx, sess.SessionID, model.SessionTTL)
	pkg.Success(ctx, iris.Map{
		"access_token": sess.SessionID,
		"user":         u.ToResponse(),
	})
}
