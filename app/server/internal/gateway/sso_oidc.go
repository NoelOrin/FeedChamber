package gateway

import (
	"net/http"
	"strings"
	"time"

	"github.com/kataras/iris/v12"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
)

// OIDC Discovery endpoint: GET /.well-known/openid-configuration
func (p *SSOProvider) Discovery(ctx iris.Context) {
	p.loadSettings(ctx.Request().Context())
	if !p.config.Enabled {
		pkg.Fail(ctx, pkg.NOT_FOUND, "SSO 未启用")
		return
	}
	issuer := p.config.Issuer
	if issuer == "" {
		issuer = ctx.Scheme() + "://" + ctx.Host()
	}
	ctx.JSON(map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/api/v1/sso/authorize",
		"token_endpoint":                        issuer + "/api/v1/sso/token",
		"userinfo_endpoint":                     issuer + "/api/v1/sso/userinfo",
		"jwks_uri":                              issuer + "/.well-known/jwks.json",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"claims_supported": []string{
			"sub", "iss", "aud", "exp", "iat", "auth_time",
			"name", "preferred_username", "email",
		},
		"scopes_supported":      []string{"openid", "profile", "email"},
		"grant_types_supported": []string{"authorization_code"},
	})
}

// OIDC 授权端点
// @Summary      OIDC 授权端点
// @Description  OIDC 授权端点，校验登录态后签发授权码。
// @Tags         SSO
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /sso/authorize [get]
// Authorize handles GET /api/v1/sso/authorize.
// Requires the resource owner to be authenticated (session or API key).
// Returns an authorization code for the authorization_code grant type.
func (p *SSOProvider) Authorize(ctx iris.Context) {
	p.loadSettings(ctx.Request().Context())
	if !p.config.Enabled {
		ctx.StopWithJSON(http.StatusNotFound, oauthError("access_denied", "SSO not enabled"))
		return
	}

	responseType := ctx.FormValue("response_type")
	if responseType != "code" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("unsupported_response_type", "Only response_type=code is supported"))
		return
	}

	clientID := ctx.FormValue("client_id")
	if clientID == "" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_request", "client_id is required"))
		return
	}
	if clientID != p.config.ClientID {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_client", "Unknown client"))
		return
	}

	redirectURI := ctx.FormValue("redirect_uri")

	// Get the authenticated user from request context (set by gateway middleware)
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("access_denied", "Authentication required"))
		return
	}

	// Generate and store authorization code (10 min TTL)
	code := randomString(32)
	store := p.deps.KV
	if store != nil {
		scope := ctx.FormValue("scope")
		nonce := ctx.FormValue("nonce")
		codeData := &ssoCodeData{
			Code:        code,
			ClientID:    clientID,
			UserUUID:    userUUID,
			RedirectURI: redirectURI,
			Scope:       scope,
			Nonce:       nonce,
		}
		_ = store.Set(ctx.Request().Context(), ssoCodeKey(code), codeData.String(), 10*time.Minute)
	}

	state := ctx.FormValue("state")
	resp := map[string]any{
		"code":  code,
		"state": state,
	}
	pkg.Success(ctx, resp)
}

// OIDC 令牌端点
// @Summary      OIDC 令牌端点
// @Description  以授权码换取 access token 与 id_token（id_token 是唯一的 JWT）。
// @Tags         SSO
// @Accept       application/x-www-form-urlencoded
// @Produce      json
// @Success      200  {object}  object
// @Router       /sso/token [post]
// Token handles POST /api/v1/sso/token.
// Authenticates the client and issues access_token + id_token.
func (p *SSOProvider) Token(ctx iris.Context) {
	p.loadSettings(ctx.Request().Context())
	if !p.config.Enabled {
		ctx.StopWithJSON(http.StatusNotFound, oauthError("access_denied", "SSO not enabled"))
		return
	}

	grantType := ctx.FormValue("grant_type")
	if grantType != "authorization_code" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("unsupported_grant_type", "Only authorization_code is supported"))
		return
	}

	// Authenticate client
	clientID, clientSecret, ok := ctx.Request().BasicAuth()
	if !ok {
		clientID = ctx.FormValue("client_id")
		clientSecret = ctx.FormValue("client_secret")
	}
	if clientID != p.config.ClientID || clientSecret != p.config.ClientSecret {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("invalid_client", "Client authentication failed"))
		return
	}

	code := ctx.FormValue("code")
	if code == "" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_request", "code is required"))
		return
	}

	// Verify authorization code from KV store
	store := p.deps.KV
	if store == nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "KV store unavailable"))
		return
	}
	codeStr, err := store.Get(ctx.Request().Context(), ssoCodeKey(code))
	if err != nil || codeStr == "" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_grant", "Invalid or expired authorization code"))
		return
	}
	codeData := parseSSOCodeData(codeStr)
	if codeData == nil {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_grant", "Invalid authorization code data"))
		return
	}

	// Delete used authorization code (one-time use)
	_ = store.Delete(ctx.Request().Context(), ssoCodeKey(code))

	// Look up user
	var user model.User
	if err := p.deps.DB.Where("id = ?", codeData.UserUUID).First(&user).Error; err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to look up user"))
		return
	}

	// Ban check before issuing token
	if model.HasBanRole(user.Role) {
		ctx.StopWithJSON(http.StatusForbidden, oauthError("access_denied", "Account is banned"))
		return
	}
	if p.deps.Ban != nil && p.deps.Ban.IsBanned(ctx.Request().Context(), user.ID) {
		ctx.StopWithJSON(http.StatusForbidden, oauthError("access_denied", "Account is banned"))
		return
	}

	issuer := p.config.Issuer
	if issuer == "" {
		issuer = ctx.Scheme() + "://" + ctx.Host()
	}

	// Access token is opaque (oauth_tokens row); only the id_token is a JWT.
	accessToken, err := storeOAuthAccessToken(ctx.Request().Context(), p.deps.DB, p.config.ClientID, user.ID, codeData.Scope)
	if err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to generate access token"))
		return
	}

	// Generate ID token (OIDC-specific JWT)
	idToken, err := p.generateIDToken(ctx.Request().Context(), issuer, user.ID, p.config.ClientID, user.DisplayName, user.Username, codeData.Nonce)
	if err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to generate ID token"))
		return
	}

	ctx.JSON(map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int64(pkg.OAuthAccessTokenTTL.Seconds()),
		"id_token":     idToken,
		"scope":        codeData.Scope,
	})
}

// OIDC UserInfo
// @Summary      OIDC UserInfo
// @Description  以 access token 读取用户声明。
// @Tags         SSO
// @Produce      json
// @Success      200  {object}  object
// @Router       /sso/userinfo [get]
// UserInfo handles GET /api/v1/sso/userinfo.
// Requires a valid opaque Bearer access token obtained from the token endpoint.
func (p *SSOProvider) UserInfo(ctx iris.Context) {
	p.loadSettings(ctx.Request().Context())
	if !p.config.Enabled {
		ctx.StopWithJSON(http.StatusNotFound, oauthError("access_denied", "SSO not enabled"))
		return
	}

	authHeader := ctx.GetHeader("Authorization")
	tokenStr := parseBearerToken(authHeader)
	if tokenStr == "" {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("invalid_token", "Missing or invalid Authorization header"))
		return
	}

	row, err := lookupOAuthAccessToken(ctx.Request().Context(), p.deps.DB, tokenStr)
	if err != nil || row.UserID == "" {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("invalid_token", "Invalid or expired token"))
		return
	}

	var user model.User
	if err := p.deps.DB.WithContext(ctx.Request().Context()).Where("id = ?", row.UserID).First(&user).Error; err != nil {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("invalid_token", "Invalid or expired token"))
		return
	}

	issuer := p.config.Issuer
	if issuer == "" {
		issuer = ctx.Scheme() + "://" + ctx.Host()
	}

	ctx.JSON(map[string]any{
		"sub":                user.ID,
		"name":               user.Username,
		"preferred_username": user.Username,
		"iss":                issuer,
	})
}

// JWKS handles GET /.well-known/jwks.json.
// Returns the JSON Web Key Set for the signing key.
func (p *SSOProvider) JWKS(ctx iris.Context) {
	p.loadSettings(ctx.Request().Context())
	if !p.config.Enabled {
		pkg.Fail(ctx, pkg.NOT_FOUND, "SSO 未启用")
		return
	}
	ctx.JSON(p.jwks(ctx.Request().Context()))
}

// ── SSO authorization code helpers ────────────────────────────────────────────

type ssoCodeData struct {
	Code        string
	ClientID    string
	UserUUID    string
	RedirectURI string
	Scope       string
	Nonce       string
}

func (d *ssoCodeData) String() string {
	return strings.Join([]string{d.Code, d.ClientID, d.UserUUID, d.RedirectURI, d.Scope, d.Nonce}, "|")
}

func parseSSOCodeData(s string) *ssoCodeData {
	parts := strings.SplitN(s, "|", 6)
	if len(parts) < 5 {
		return nil
	}
	nonce := ""
	if len(parts) == 6 {
		nonce = parts[5]
	}
	return &ssoCodeData{
		Code:        parts[0],
		ClientID:    parts[1],
		UserUUID:    parts[2],
		RedirectURI: parts[3],
		Scope:       parts[4],
		Nonce:       nonce,
	}
}

func ssoCodeKey(code string) string {
	return "sso_code:" + code
}
