package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/kataras/iris/v12"
)

// OAuthProvider implements the OAuth 2.0 Authorization Framework (RFC 6749)
// as a provider, supporting authorization_code and client_credentials grant types.
type OAuthProvider struct {
	deps Deps
	db   *gorm.DB
}

// NewOAuthProvider creates a new OAuthProvider with the given dependencies.
func NewOAuthProvider(deps Deps) *OAuthProvider {
	return &OAuthProvider{
		deps: deps,
		db:   deps.DB,
	}
}

// ── Client Registration (admin) ──────────────────────────────────────────────

// 注册 OAuth 客户端
// @Summary      注册 OAuth 客户端
// @Description  把本服务作为 OAuth 2.0 Provider 时，登记一个客户端。
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,redirect_uris=[]string,scopes=[]string} true "客户端配置"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/clients [post]
// RegisterClient handles POST /api/v1/oauth/clients.
// Requires admin-level permission (enforced by the route middleware).
func (p *OAuthProvider) RegisterClient(ctx iris.Context) {
	var req struct {
		Name         string `json:"name"`
		RedirectURIs string `json:"redirect_uris"`
		Scopes       string `json:"scopes"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	if req.Name == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "缺少 name")
		return
	}
	var redirectURIs []string
	if err := json.Unmarshal([]byte(req.RedirectURIs), &redirectURIs); err != nil || len(redirectURIs) == 0 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "redirect_uris 必须是非空 JSON 数组")
		return
	}

	clientID := randomString(20)
	// bcrypt rejects inputs longer than 72 bytes; 32 random bytes hex-encode to
	// exactly 64 chars, leaving headroom for the (fixed-length) secret.
	clientSecret := randomString(32)

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "客户端密钥哈希失败")
		return
	}

	createdBy := ctx.Values().GetString(pkg.CtxUsername)

	client := &model.OAuthClient{
		ClientID:     clientID,
		ClientSecret: string(hashedSecret),
		Name:         req.Name,
		RedirectURIs: req.RedirectURIs,
		Scopes:       req.Scopes,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now().Unix(),
	}

	if err := p.db.Create(client).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "客户端创建失败")
		return
	}

	pkg.Success(ctx, map[string]any{
		"client_id":     client.ClientID,
		"client_secret": clientSecret,
		"name":          client.Name,
	})
}

// OAuth 客户端列表
// @Summary      OAuth 客户端列表
// @Description  列出已登记的 OAuth 客户端。
// @Tags         OAuth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/clients/list [post]
// ListClients handles POST /api/v1/oauth/clients/list (admin).
func (p *OAuthProvider) ListClients(ctx iris.Context) {
	var clients []model.OAuthClient
	if err := p.db.Order("created_at DESC").Find(&clients).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "客户端列表读取失败")
		return
	}
	pkg.Success(ctx, clients)
}

// 更新 OAuth 客户端
// @Summary      更新 OAuth 客户端
// @Description  修改客户端名称、回调地址或权限范围。
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        body body object{name=string,redirect_uris=[]string,scopes=[]string} false "更新字段"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/clients/{id}/update [post]
// UpdateClient handles POST /api/v1/oauth/clients/{id}/update (admin).
func (p *OAuthProvider) UpdateClient(ctx iris.Context) {
	var id uint
	if _, err := fmt.Sscanf(ctx.Params().Get("id"), "%d", &id); err != nil || id == 0 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "client_id 无效")
		return
	}
	var client model.OAuthClient
	if err := p.db.First(&client, id).Error; err != nil {
		pkg.Fail(ctx, pkg.NOT_FOUND, "客户端不存在")
		return
	}
	var req struct {
		Name         string `json:"name"`
		RedirectURIs string `json:"redirect_uris"`
		Scopes       string `json:"scopes"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
		client.Name = req.Name
	}
	if req.RedirectURIs != "" {
		var redirectURIs []string
		if err := json.Unmarshal([]byte(req.RedirectURIs), &redirectURIs); err != nil || len(redirectURIs) == 0 {
			pkg.Fail(ctx, pkg.INVALID_PARAMS, "redirect_uris 必须是非空 JSON 数组")
			return
		}
		updates["redirect_uris"] = req.RedirectURIs
		client.RedirectURIs = req.RedirectURIs
	}
	if req.Scopes != "" {
		updates["scopes"] = req.Scopes
		client.Scopes = req.Scopes
	}
	if err := p.db.Model(&client).Updates(updates).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "客户端更新失败")
		return
	}
	pkg.Success(ctx, client)
}

// 重置客户端密钥
// @Summary      重置客户端密钥
// @Description  生成新的 client_secret，旧密钥立即失效。
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/clients/{id}/reset-secret [post]
// ResetClientSecret handles POST /api/v1/oauth/clients/{id}/reset-secret (admin).
func (p *OAuthProvider) ResetClientSecret(ctx iris.Context) {
	var id uint
	if _, err := fmt.Sscanf(ctx.Params().Get("id"), "%d", &id); err != nil || id == 0 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "client_id 无效")
		return
	}
	var client model.OAuthClient
	if err := p.db.First(&client, id).Error; err != nil {
		pkg.Fail(ctx, pkg.NOT_FOUND, "客户端不存在")
		return
	}
	secret := randomString(32) // see RegisterClient: bcrypt's 72-byte limit
	hashed, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "客户端密钥哈希失败")
		return
	}
	if err := p.db.Model(&client).Update("client_secret", string(hashed)).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "客户端密钥重置失败")
		return
	}
	pkg.Success(ctx, map[string]any{"client_id": client.ClientID, "client_secret": secret})
}

// 删除 OAuth 客户端
// @Summary      删除 OAuth 客户端
// @Description  删除客户端并级联撤销其授权。
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /oauth/clients/{id}/delete [post]
// DeleteClient handles POST /api/v1/oauth/clients/{id}/delete (admin).
func (p *OAuthProvider) DeleteClient(ctx iris.Context) {
	var id uint
	if _, err := fmt.Sscanf(ctx.Params().Get("id"), "%d", &id); err != nil || id == 0 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "client_id 无效")
		return
	}
	if err := p.db.Delete(&model.OAuthClient{}, id).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "客户端删除失败")
		return
	}
	pkg.Success(ctx, map[string]any{"message": "Client deleted"})
}

// ── Authorization Endpoint ───────────────────────────────────────────────────

// OAuth 授权端点
// @Summary      OAuth 授权端点
// @Description  标准的 OAuth 2.0 授权端点；GET 展示授权页/跳转，POST 提交同意结果。
// @Tags         OAuth
// @Success      200  {object}  pkg.Response
// @Router       /oauth/authorize [get]
// @Router       /oauth/authorize [post]
// Authorize handles GET|POST /api/v1/oauth/authorize.
// Requires the resource owner (user) to be authenticated (session or API key).
// Returns an authorization code for the authorization_code grant type.
func (p *OAuthProvider) Authorize(ctx iris.Context) {
	responseType := ctx.FormValue("response_type")
	if responseType == "" {
		responseType = "code" // default
	}
	if responseType != "code" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("unsupported_response_type", "Only response_type=code is supported"))
		return
	}

	clientID := ctx.FormValue("client_id")
	if clientID == "" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_request", "client_id is required"))
		return
	}

	// Validate client exists
	var client model.OAuthClient
	if err := p.db.Where("client_id = ?", clientID).First(&client).Error; err != nil {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_client", "Unknown client"))
		return
	}
	redirectURI := ctx.FormValue("redirect_uri")
	if redirectURI != "" && !clientAllowsRedirect(client.RedirectURIs, redirectURI) {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_request", "redirect_uri is not allowed"))
		return
	}

	// Get the authenticated user from request context (set by gateway middleware)
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("access_denied", "Authentication required"))
		return
	}

	requestedScope := ctx.FormValue("scope")
	if requestedScope != "" {
		allowedList := splitScopes(client.Scopes)
		if len(allowedList) > 0 {
			allowed := map[string]bool{}
			for _, s := range allowedList {
				allowed[s] = true
			}
			for _, s := range splitScopes(requestedScope) {
				if !allowed[s] {
					ctx.StopWithJSON(400, oauthError("invalid_scope", "Requested scope is not allowed"))
					return
				}
			}
		}
	}

	// Generate and store the authorization code
	code := randomString(32)
	authCode := &model.OAuthAuthorize{
		Code:        code,
		ClientID:    clientID,
		UserID:      userUUID,
		Scope:       requestedScope,
		RedirectURI: redirectURI,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
		CreatedAt:   time.Now(),
	}
	if err := p.db.Create(authCode).Error; err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to store authorization code"))
		return
	}

	resp := map[string]any{
		"code":  code,
		"state": ctx.FormValue("state"),
	}
	pkg.Success(ctx, resp)
}

// ── Token Endpoint ───────────────────────────────────────────────────────────

// OAuth 令牌端点
// @Summary      OAuth 令牌端点
// @Description  以授权码或客户端凭证换取 opaque access token（RFC 6749 形状，非统一信封）。
// @Tags         OAuth
// @Accept       application/x-www-form-urlencoded
// @Produce      json
// @Success      200  {object}  object
// @Router       /oauth/token [post]
// Token handles POST /api/v1/oauth/token.
// Authenticates the client and issues access tokens.
func (p *OAuthProvider) Token(ctx iris.Context) {
	grantType := ctx.FormValue("grant_type")
	if grantType == "" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_request", "grant_type is required"))
		return
	}

	switch grantType {
	case "authorization_code":
		p.handleAuthorizationCodeGrant(ctx)
	case "client_credentials":
		p.handleClientCredentialsGrant(ctx)
	default:
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("unsupported_grant_type", "Supported grant types: authorization_code, client_credentials"))
		return
	}
}

func (p *OAuthProvider) handleAuthorizationCodeGrant(ctx iris.Context) {
	client, err := p.authenticateClient(ctx)
	if err != nil {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("invalid_client", "Client authentication failed"))
		return
	}

	code := ctx.FormValue("code")
	if code == "" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_request", "code is required"))
		return
	}

	// Verify the authorization code
	var authCode model.OAuthAuthorize
	if err := p.db.Where("code = ? AND client_id = ? AND expires_at > ?", code, client.ClientID, time.Now()).First(&authCode).Error; err != nil {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_grant", "Invalid or expired authorization code"))
		return
	}
	if redirectURI := ctx.FormValue("redirect_uri"); authCode.RedirectURI != "" && redirectURI != authCode.RedirectURI {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_grant", "redirect_uri mismatch"))
		return
	}

	// Delete the used authorization code (one-time use)
	p.db.Delete(&authCode)

	// The resource owner must still exist; the token row references their id.
	var user model.User
	if err := p.db.WithContext(ctx.Request().Context()).
		Where("id = ?", authCode.UserID).First(&user).Error; err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to look up user"))
		return
	}

	// Opaque access token; its SHA-256 digest is what gets stored.
	accessTokenStr, err := p.issueOAuthAccessToken(ctx.Request().Context(), client.ClientID, user.ID, authCode.Scope)
	if err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to generate access token"))
		return
	}

	ctx.JSON(map[string]any{
		"access_token": accessTokenStr,
		"token_type":   "Bearer",
		"expires_in":   int64(pkg.OAuthAccessTokenTTL.Seconds()),
		"scope":        authCode.Scope,
	})
}

func (p *OAuthProvider) handleClientCredentialsGrant(ctx iris.Context) {
	client, err := p.authenticateClient(ctx)
	if err != nil {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("invalid_client", "Client authentication failed"))
		return
	}

	scope := ctx.FormValue("scope")
	allowed := map[string]bool{}
	for _, s := range strings.FieldsFunc(client.Scopes, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		allowed[strings.TrimSpace(s)] = true
	}
	if len(allowed) == 0 {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_scope", "Client has no allowed scopes"))
		return
	}
	for _, s := range strings.Fields(scope) {
		if !allowed[s] {
			ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_scope", "Requested scope is not allowed"))
			return
		}
	}

	// Opaque access token bound to the client, not to a user.
	accessTokenStr, err := p.issueOAuthAccessToken(ctx.Request().Context(), client.ClientID, "", scope)
	if err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to generate access token"))
		return
	}

	ctx.JSON(map[string]any{
		"access_token": accessTokenStr,
		"token_type":   "Bearer",
		"expires_in":   int64(pkg.OAuthAccessTokenTTL.Seconds()),
		"scope":        scope,
	})
}

// ── Token Revocation ────────────────────────────────────────────────────────

// OAuth 令牌撤销
// @Summary      OAuth 令牌撤销
// @Description  撤销一枚 access token（RFC 7009）。
// @Tags         OAuth
// @Accept       application/x-www-form-urlencoded
// @Produce      json
// @Success      200  {object}  object
// @Router       /oauth/revoke [post]
// Revoke handles POST /api/v1/oauth/revoke (RFC 7009).
func (p *OAuthProvider) Revoke(ctx iris.Context) {
	client, err := p.authenticateClient(ctx)
	if err != nil {
		ctx.StopWithJSON(http.StatusUnauthorized, oauthError("invalid_client", "Client authentication failed"))
		return
	}

	tokenStr := ctx.FormValue("token")
	if tokenStr == "" {
		ctx.StopWithJSON(http.StatusBadRequest, oauthError("invalid_request", "token is required"))
		return
	}

	// Opaque tokens carry no structure, so revoke by digest. Unknown tokens
	// still return 200 per RFC 7009 §2.2: the endpoint must not be usable to
	// probe whether a token exists.
	err = p.db.WithContext(ctx.Request().Context()).Model(&model.OAuthToken{}).
		Where("access_token = ? AND client_id = ?", hashOAuthToken(tokenStr), client.ClientID).
		Update("revoked", true).Error
	if err != nil {
		ctx.StopWithJSON(http.StatusInternalServerError, oauthError("server_error", "Failed to revoke token"))
		return
	}

	ctx.StatusCode(http.StatusOK)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// authenticateClient extracts and validates client credentials from the request.
// Supports HTTP Basic auth and form-encoded body params.
func (p *OAuthProvider) authenticateClient(ctx iris.Context) (*model.OAuthClient, error) {
	clientID, clientSecret, ok := ctx.Request().BasicAuth()
	if !ok {
		clientID = ctx.FormValue("client_id")
		clientSecret = ctx.FormValue("client_secret")
	}
	if clientID == "" || clientSecret == "" {
		return nil, ErrInvalidClient
	}

	var client model.OAuthClient
	if err := p.db.Where("client_id = ?", clientID).First(&client).Error; err != nil {
		return nil, ErrInvalidClient
	}
	if err := bcrypt.CompareHashAndPassword([]byte(client.ClientSecret), []byte(clientSecret)); err != nil {
		return nil, ErrInvalidClient
	}
	return &client, nil
}

func splitScopes(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
}

func clientAllowsRedirect(raw, redirectURI string) bool {
	var uris []string
	if err := json.Unmarshal([]byte(raw), &uris); err != nil {
		return false
	}
	for _, uri := range uris {
		if strings.TrimSpace(uri) == redirectURI {
			return true
		}
	}
	return false
}

// randomString generates a hex-encoded random string of n bytes (result length = n*2).
func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// oauthError builds a standard OAuth 2.0 error response body.
func oauthError(errCode, description string) map[string]any {
	return map[string]any{
		"error":             errCode,
		"error_description": description,
	}
}

// ErrInvalidClient is returned when client authentication fails.
var ErrInvalidClient = &clientAuthError{}

type clientAuthError struct{}

func (e *clientAuthError) Error() string { return "invalid client credentials" }
