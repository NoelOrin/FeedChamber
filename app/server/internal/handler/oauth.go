package handler

import (
	"log/slog"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type OAuthHandler struct {
	oauthService service.OAuthService
	bannedSvc    service.BannedService
	sessions     service.SessionStore
}

func NewOAuthHandler(oauthService service.OAuthService, bannedSvc service.BannedService, sessions service.SessionStore) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
		bannedSvc:    bannedSvc,
		sessions:     sessions,
	}
}

// GetAuthURL generates the OAuth authorization URL
// @Summary      获取OAuth授权URL
// @Description  生成OAuth授权URL，用于跳转到第三方登录页
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Param        provider  path      string  true  "OAuth提供商 (github/google/qq/wechat/weibo)"
// @Success      200  {object}  pkg.Response{data=object{auth_url=string,state=string}}
// @Failure      400  {object}  pkg.Response
// @Router       /oauth/{provider}/auth [get]
func (h *OAuthHandler) GetAuthURL(ctx iris.Context) {
	provider := model.OAuthProvider(ctx.Params().Get("provider"))
	if provider == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "provider is required")
		return
	}

	// Generate state for CSRF protection
	state := pkg.NewID()

	authURL, err := h.oauthService.GetAuthURL(ctx.Request().Context(), provider, state)
	if err != nil {
		slog.Error("oauth get auth url failed", "provider", provider, "error", err)
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "获取授权链接失败")
		return
	}

	pkg.Success(ctx, iris.Map{
		"auth_url": authURL,
		"state":    state,
	})
}

// HandleCallback handles the OAuth callback
// @Summary      OAuth回调处理
// @Description  处理OAuth提供商的回调，交换code获取用户信息
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Param        provider  path      string  true  "OAuth提供商"
// @Param        code      query     string  true  "授权码"
// @Param        state     query     string  false "state参数"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Router       /oauth/{provider}/callback [get]
func (h *OAuthHandler) HandleCallback(ctx iris.Context) {
	provider := model.OAuthProvider(ctx.Params().Get("provider"))
	if provider == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "provider is required")
		return
	}

	code := ctx.URLParamDefault("code", "")
	if code == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "code is required")
		return
	}

	state := ctx.URLParamDefault("state", "")

	oauthUser, err := h.oauthService.HandleCallback(ctx.Request().Context(), provider, code, state)
	if err != nil {
		slog.Error("oauth callback failed", "provider", provider, "error", err)
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "OAuth 登录失败")
		return
	}

	account, err := h.oauthService.FindByProviderAndProviderID(ctx.Request().Context(), provider, oauthUser.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	if account != nil {
		user, err := h.oauthService.FindUserByUUID(ctx.Request().Context(), account.UserID)
		if err != nil {
			pkg.HandleError(ctx, err)
			return
		}

		// Ban check before issuing the session credential
		if model.HasBanRole(user.Role) {
			pkg.Fail(ctx, pkg.FORBIDDEN, "Account is banned")
			return
		}
		if h.bannedSvc != nil {
			if banned := h.bannedSvc.IsBanned(ctx.Request().Context(), user.ID); banned {
				pkg.Fail(ctx, pkg.FORBIDDEN, "Account is banned")
				return
			}
		}

		// 社交登录绑定完成 = 登录成功：签发服务端会话，凭证与 /auth/login 同构。
		if h.sessions == nil {
			pkg.Fail(ctx, pkg.INTERNAL_ERROR, "session store unavailable")
			return
		}
		sess, err := h.sessions.Issue(ctx.Request().Context(), user, ctx.RemoteAddr(), ctx.Request().UserAgent())
		if err != nil {
			pkg.HandleError(ctx, err)
			return
		}
		pkg.SetSessionCookie(ctx, sess.SessionID, model.SessionTTL)
		pkg.Success(ctx, iris.Map{
			"access_token": sess.SessionID,
			"user":         user.ToResponse(),
		})
	} else {
		pkg.Success(ctx, iris.Map{
			"needs_register": true,
			"oauth_user":     oauthUser,
		})
	}
}

// GetProviderConfig returns the configuration for a specific provider
// @Summary      获取单个OAuth提供商配置
// @Description  获取指定OAuth提供商的配置信息（不包含敏感字段）
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Param        provider  path      string  true  "OAuth提供商"
// @Success      200  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /oauth/{provider}/config [get]
func (h *OAuthHandler) GetProviderConfig(ctx iris.Context) {
	provider := model.OAuthProvider(ctx.Params().Get("provider"))
	if provider == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "provider is required")
		return
	}

	config, ok := h.oauthService.GetProviderConfig(ctx.Request().Context(), provider)
	if !ok {
		pkg.Fail(ctx, pkg.NOT_FOUND, "provider not found")
		return
	}

	// Filter out sensitive information
	pkg.Success(ctx, iris.Map{
		"enabled":      config.Enabled,
		"scopes":       config.Scopes,
		"auth_url":     config.AuthURL,
		"token_url":    config.TokenURL,
		"user_api_url": config.UserAPIURL,
		"callback_url": config.CallbackURL,
	})
}

// GetAllProviderConfigs returns all provider configurations
// @Summary      获取所有OAuth配置
// @Description  获取所有支持的OAuth提供商及其配置信息
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /oauth/providers [get]
func (h *OAuthHandler) GetAllProviderConfigs(ctx iris.Context) {
	configs := h.oauthService.GetAllProviderConfigs(ctx.Request().Context())

	result := make(map[string]interface{})
	for provider, config := range configs {
		result[string(provider)] = iris.Map{
			"enabled":      config.Enabled,
			"scopes":       config.Scopes,
			"auth_url":     config.AuthURL,
			"token_url":    config.TokenURL,
			"user_api_url": config.UserAPIURL,
			"callback_url": config.CallbackURL,
		}
	}

	pkg.Success(ctx, result)
}

// LinkAccount links an OAuth account to the current user
// @Summary      关联OAuth账号
// @Description  将OAuth账号关联到当前登录用户
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        provider  path      string  true  "OAuth提供商"
// @Param        body      body      object{code=string}  true  "授权码"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/{provider}/link [post]
func (h *OAuthHandler) LinkAccount(ctx iris.Context) {
	provider := model.OAuthProvider(ctx.Params().Get("provider"))
	if provider == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "provider is required")
		return
	}

	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Code == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "code is required")
		return
	}
	// state is optional for LinkAccount: the route requires authentication, so the user
	// identity is already verified. When state is provided, HandleCallback validates
	// it against KV for full CSRF protection; when empty, CSRF check is skipped.

	// Get current user from context (set by the gateway auth middleware)
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	// Exchange code for user info
	user, err := h.oauthService.HandleCallback(ctx.Request().Context(), provider, req.Code, req.State)
	if err != nil {
		slog.Error("oauth link callback failed", "provider", provider, "error", err)
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "OAuth 登录失败")
		return
	}

	// Link account
	err = h.oauthService.LinkAccount(ctx.Request().Context(), u.ID, provider, user.ID, "", user.ID, user.Avatar, user.Name, user.Email, user.AccessToken, user.RefreshToken, user.ExpiresAt)
	if err != nil {
		slog.Error("oauth link account failed", "provider", provider, "error", err)
		pkg.Fail(ctx, pkg.ALREADY_EXISTS, "关联账号失败")
		return
	}

	pkg.Success(ctx, iris.Map{
		"message": "OAuth account linked successfully",
		"user":    user,
	})
}

// GetLinkedAccounts returns all OAuth accounts linked to the current user
// @Summary      获取已关联的OAuth账号
// @Description  获取当前登录用户已关联的所有OAuth账号
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/accounts [get]
func (h *OAuthHandler) GetLinkedAccounts(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	accounts, err := h.oauthService.GetLinkedAccounts(ctx.Request().Context(), u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	pkg.Success(ctx, accounts)
}

// UnlinkAccount removes an OAuth account from the current user
// @Summary      取消关联OAuth账号
// @Description  取消当前登录用户与指定OAuth提供商的账号关联
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        provider  path      string  true  "OAuth提供商"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/{provider}/unlink [post]
func (h *OAuthHandler) UnlinkAccount(ctx iris.Context) {
	provider := model.OAuthProvider(ctx.Params().Get("provider"))
	if provider == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "provider is required")
		return
	}

	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	err := h.oauthService.UnlinkAccount(ctx.Request().Context(), u.ID, provider)
	if err != nil {
		pkg.Fail(ctx, pkg.NOT_FOUND, err.Error())
		return
	}

	pkg.Success(ctx, iris.Map{
		"message": "OAuth account unlinked successfully",
	})
}
