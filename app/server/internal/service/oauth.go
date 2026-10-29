package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/repository"
	"strings"
	"time"
)

// oauthHTTPClient is a shared HTTP client with a timeout for OAuth provider calls.
// Using a shared client avoids creating a new client per request and ensures all
// external OAuth requests have a bounded duration.
var oauthHTTPClient = &http.Client{Timeout: 15 * time.Second}

// httpPostForm wraps http.PostForm using the shared oauthHTTPClient.
func httpPostForm(url string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(data.Encode()))
	return oauthHTTPClient.Do(req)
}

type oauthService struct {
	userRepo   repository.UserRepository
	oauthRepo  repository.OAuthRepository
	settingSvc SettingService
	kv         kvstore.Store
}

func NewOAuthService(userRepo repository.UserRepository, oauthRepo repository.OAuthRepository, settingSvc SettingService, kv kvstore.Store) OAuthService {
	return &oauthService{
		userRepo:   userRepo,
		oauthRepo:  oauthRepo,
		settingSvc: settingSvc,
		kv:         kv,
	}
}

// getProviderConfigFromDB fetches fresh OAuth config from database
func (s *oauthService) getProviderConfigFromDB(ctx context.Context, provider model.OAuthProvider) (model.OAuthConfig, error) {
	config := model.OAuthProviderConfigs[provider]

	p := string(provider)

	// Read settings from database (no env fallback — provider-specific secrets)
	config.Enabled = pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_enabled", "", "") == "true"
	config.ClientID = pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_client_id", "", "")
	config.ClientSecret = pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_client_secret", "", "")
	config.CallbackURL = pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_callback", "", "")

	// M-26: allow overriding endpoints/scopes for self-hosted/enterprise OAuth.
	// Empty DB value falls back to the hardcoded default already in `config`.
	// envKey is "" deliberately — no env fallback for provider-specific endpoints.
	config.AuthURL = pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_auth_url", "", config.AuthURL)
	config.TokenURL = pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_token_url", "", config.TokenURL)
	config.UserAPIURL = pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_user_api_url", "", config.UserAPIURL)
	config.Scopes = strings.Fields(pkg.GetSettingOrDefault(ctx, s.settingSvc, "oauth_"+p+"_scopes", "", joinScopes(config.Scopes)))

	return config, nil
}

// GetAuthURL generates the OAuth authorization URL
func (s *oauthService) GetAuthURL(ctx context.Context, provider model.OAuthProvider, state string) (string, error) {
	config, err := s.getProviderConfigFromDB(ctx, provider)
	if err != nil {
		return "", err
	}

	if !config.Enabled {
		return "", pkg.NewAppError(pkg.FORBIDDEN, "provider "+string(provider)+" is not enabled")
	}

	if config.ClientID == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "provider "+string(provider)+" is not configured")
	}

	// Build auth URL with parameters
	q := url.Values{}
	q.Set("client_id", config.ClientID)
	q.Set("redirect_uri", config.CallbackURL)
	q.Set("response_type", "code")
	if len(config.Scopes) > 0 {
		q.Set("scope", joinScopes(config.Scopes))
	}
	if state != "" {
		q.Set("state", state)
	}

	authURL := config.AuthURL + "?" + q.Encode()

	// Store state in KV for CSRF validation (5 min TTL)
	if state != "" {
		if err := s.kv.Set(ctx, "oauth:"+state, string(provider), 5*time.Minute); err != nil {
			return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to store oauth state: "+err.Error())
		}
	}

	return authURL, nil
}

// HandleCallback exchanges the authorization code for an access token.
// state 为空时跳过 CSRF 校验（用于已认证的 LinkAccount 场景，A-02 修复）。
func (s *oauthService) HandleCallback(ctx context.Context, provider model.OAuthProvider, code string, state string) (*model.OAuthUser, error) {
	// Validate CSRF state (skip when state is empty — already authenticated via session)
	if state != "" {
		exists, err := s.kv.Exists(ctx, "oauth:"+state)
		if err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to validate oauth state: "+err.Error())
		}
		if !exists {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid or expired state")
		}
		// One-time use: delete the state immediately
		if err := s.kv.Delete(ctx, "oauth:"+state); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to consume oauth state: "+err.Error())
		}
	}

	config, err := s.getProviderConfigFromDB(ctx, provider)
	if err != nil {
		return nil, err
	}

	if !config.Enabled {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "provider "+string(provider)+" is not enabled")
	}

	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider "+string(provider)+" is not configured")
	}

	// Exchange code for token
	tokenResp, err := s.exchangeCodeForToken(provider, code, config)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "failed to exchange token: "+err.Error())
	}

	// Get user info from provider
	user, err := s.GetUserUserProfile(provider, tokenResp.AccessToken, config.UserAPIURL)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "failed to get user profile: "+err.Error())
	}

	user.AccessToken = tokenResp.AccessToken
	user.RefreshToken = tokenResp.RefreshToken
	if tokenResp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		user.ExpiresAt = &t
	}

	return user, nil
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// exchangeCodeForToken exchanges authorization code for access token
func (s *oauthService) exchangeCodeForToken(provider model.OAuthProvider, code string, config model.OAuthConfig) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("redirect_uri", config.CallbackURL)

	switch provider {
	case model.ProviderGitHub:
		form.Set("client_id", config.ClientID)
		form.Set("client_secret", config.ClientSecret)
	case model.ProviderGoogle:
		form.Set("client_id", config.ClientID)
		form.Set("client_secret", config.ClientSecret)
		form.Set("grant_type", "authorization_code")
	case model.ProviderQQ:
		form.Set("client_id", config.ClientID)
		form.Set("client_secret", config.ClientSecret)
		form.Set("grant_type", "authorization_code")
	case model.ProviderWeChat:
		form.Set("appid", config.ClientID)
		form.Set("secret", config.ClientSecret)
		form.Set("grant_type", "authorization_code")
	case model.ProviderWeibo:
		form.Set("client_id", config.ClientID)
		form.Set("client_secret", config.ClientSecret)
		form.Set("grant_type", "authorization_code")
	case model.ProviderDiscord:
		form.Set("client_id", config.ClientID)
		form.Set("client_secret", config.ClientSecret)
		form.Set("grant_type", "authorization_code")
	default:
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "unsupported provider: "+string(provider))
	}

	resp, err := httpPostForm(config.TokenURL, form)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "token exchange request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to read token response: "+err.Error())
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to parse token response: "+err.Error())
	}

	if tokenResp.AccessToken == "" {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "empty access token in response: "+string(body))
	}

	return &tokenResp, nil
}

// GetUserUserProfile fetches user info from the OAuth provider
func (s *oauthService) GetUserUserProfile(provider model.OAuthProvider, accessToken string, userAPIURL string) (*model.OAuthUser, error) {
	if userAPIURL == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "user API URL is required for provider: "+string(provider))
	}

	req, err := http.NewRequest("GET", userAPIURL, nil)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to create user profile request: "+err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to fetch user profile: "+err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to read user profile response: "+err.Error())
	}

	var user model.OAuthUser
	switch provider {
	case model.ProviderGitHub:
		var u struct {
			ID     int64  `json:"id"`
			Login  string `json:"login"`
			Name   string `json:"name"`
			Email  string `json:"email"`
			Avatar string `json:"avatar_url"`
		}
		if err := json.Unmarshal(body, &u); err != nil {
			return nil, err
		}
		user = model.OAuthUser{
			ID:       fmt.Sprintf("%d", u.ID),
			Name:     u.Name,
			Email:    u.Email,
			Avatar:   u.Avatar,
			Provider: string(model.ProviderGitHub),
		}
	case model.ProviderGoogle:
		var u struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Email   string `json:"email"`
			Picture string `json:"picture"`
		}
		if err := json.Unmarshal(body, &u); err != nil {
			return nil, err
		}
		user = model.OAuthUser{
			ID:       u.ID,
			Name:     u.Name,
			Email:    u.Email,
			Avatar:   u.Picture,
			Provider: string(model.ProviderGoogle),
		}
	case model.ProviderQQ:
		var u struct {
			OpenID   string `json:"openid"`
			Nickname string `json:"nickname"`
			Avatar   string `json:"figureurl_qq_2"`
		}
		if err := json.Unmarshal(body, &u); err != nil {
			return nil, err
		}
		user = model.OAuthUser{
			ID:       u.OpenID,
			Name:     u.Nickname,
			Avatar:   u.Avatar,
			Provider: string(model.ProviderQQ),
		}
	case model.ProviderWeChat:
		var u struct {
			OpenID     string `json:"openid"`
			Nickname   string `json:"nickname"`
			HeadImgURL string `json:"headimgurl"`
		}
		if err := json.Unmarshal(body, &u); err != nil {
			return nil, err
		}
		user = model.OAuthUser{
			ID:       u.OpenID,
			Name:     u.Nickname,
			Avatar:   u.HeadImgURL,
			Provider: string(model.ProviderWeChat),
		}
	case model.ProviderWeibo:
		var u struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			Avatar string `json:"avatar_hd"`
		}
		if err := json.Unmarshal(body, &u); err != nil {
			return nil, err
		}
		user = model.OAuthUser{
			ID:       fmt.Sprintf("%d", u.ID),
			Name:     u.Name,
			Avatar:   u.Avatar,
			Provider: string(model.ProviderWeibo),
		}
	case model.ProviderDiscord:
		var u struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Email    string `json:"email"`
			Avatar   string `json:"avatar"`
		}
		if err := json.Unmarshal(body, &u); err != nil {
			return nil, err
		}
		// Discord avatar is a hash; construct the CDN URL.
		avatarURL := ""
		if u.Avatar != "" {
			avatarURL = "https://cdn.discordapp.com/avatars/" + u.ID + "/" + u.Avatar + ".png"
		}
		user = model.OAuthUser{
			ID:       u.ID,
			Name:     u.Username,
			Email:    u.Email,
			Avatar:   avatarURL,
			Provider: string(model.ProviderDiscord),
		}
	default:
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "unsupported provider: "+string(provider))
	}

	return &user, nil
}

// GetUserProfile is an alias for GetUserUserProfile (public API)
func (s *oauthService) GetUserProfile(provider model.OAuthProvider, accessToken string) (*model.OAuthUser, error) {
	cfg, ok := model.OAuthProviderConfigs[provider]
	if !ok {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "unsupported provider: "+string(provider))
	}
	// Uses the hardcoded UserAPIURL, not the DB override — OAuthService interface
	// has no ctx, so a DB lookup here is not possible without an interface change.
	return s.GetUserUserProfile(provider, accessToken, cfg.UserAPIURL)
}

// LinkAccount links an OAuth account to a user
func (s *oauthService) LinkAccount(ctx context.Context, userID string, provider model.OAuthProvider, providerID, openid, unionid string, avatar, name, email, token, refreshToken string, expiresAt *time.Time) error {
	// Check if account already linked
	existing, err := s.oauthRepo.FindByProviderAndProviderID(ctx, provider, providerID)
	if err != nil {
		return err
	}
	if existing != nil {
		return pkg.NewAppError(pkg.ALREADY_EXISTS, "this OAuth account is already linked to another user")
	}

	account := &model.OAuthAccount{
		UserID:       userID,
		Provider:     provider,
		ProviderID:   providerID,
		OpenID:       openid,
		UnionID:      unionid,
		Avatar:       avatar,
		Name:         name,
		Email:        email,
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}

	return s.oauthRepo.Create(ctx, account)
}

// UnlinkAccount removes an OAuth account from a user
func (s *oauthService) UnlinkAccount(ctx context.Context, userID string, provider model.OAuthProvider) error {
	account, err := s.oauthRepo.FindByUserIDAndProvider(ctx, userID, provider)
	if err != nil {
		return err
	}
	if account == nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "account not found")
	}

	return s.oauthRepo.Delete(ctx, account.ID)
}

// GetLinkedAccounts returns all OAuth accounts linked to a user
func (s *oauthService) GetLinkedAccounts(ctx context.Context, userID string) ([]*model.OAuthAccount, error) {
	return s.oauthRepo.FindByUserID(ctx, userID)
}

func (s *oauthService) FindByProviderAndProviderID(ctx context.Context, provider model.OAuthProvider, providerID string) (*model.OAuthAccount, error) {
	return s.oauthRepo.FindByProviderAndProviderID(ctx, provider, providerID)
}

func (s *oauthService) FindUserByUUID(ctx context.Context, uuid string) (*model.User, error) {
	return s.userRepo.FindByID(ctx, uuid)
}

// GetProviderConfig returns the configuration for a specific provider (from DB)
func (s *oauthService) GetProviderConfig(ctx context.Context, provider model.OAuthProvider) (model.OAuthConfig, bool) {
	config, err := s.getProviderConfigFromDB(ctx, provider)
	if err != nil {
		return model.OAuthConfig{}, false
	}
	return config, true
}

// GetAllProviderConfigs returns all provider configurations (from DB)
func (s *oauthService) GetAllProviderConfigs(ctx context.Context) map[model.OAuthProvider]model.OAuthConfig {
	providers := []model.OAuthProvider{
		model.ProviderGitHub,
		model.ProviderGoogle,
		model.ProviderQQ,
		model.ProviderWeChat,
		model.ProviderWeibo,
		model.ProviderDiscord,
	}

	configs := make(map[model.OAuthProvider]model.OAuthConfig)
	for _, p := range providers {
		config, err := s.getProviderConfigFromDB(ctx, p)
		if err == nil {
			configs[p] = config
		}
	}
	return configs
}

// joinScopes joins scopes with spaces (OAuth standard)
func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	result := ""
	for i, scope := range scopes {
		if i > 0 {
			result += " "
		}
		result += scope
	}
	return result
}
