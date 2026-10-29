package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"

	"github.com/kataras/iris/v12"
)

// UserFetchFunc fetches user info from the OAuth provider using the token.
type UserFetchFunc func(ctx context.Context, token *oauth2.Token) (*model.OAuthUser, error)

// ProviderConfig holds the OAuth2 config and user fetch function for a provider.
type ProviderConfig struct {
	Config    *oauth2.Config
	FetchUser UserFetchFunc
}

// OAuthManager manages OAuth2 authentication flows for supported providers.
type OAuthManager struct {
	deps      Deps
	providers map[string]*ProviderConfig
}

// NewOAuthManager creates a new OAuthManager and initializes provider configs
// from the settings table.
func NewOAuthManager(deps Deps) *OAuthManager {
	om := &OAuthManager{
		deps:      deps,
		providers: make(map[string]*ProviderConfig),
	}
	om.initProviders()
	return om
}

// GetProviderConfig returns the oauth2.Config for a provider.
func (om *OAuthManager) GetProviderConfig(provider string) (*ProviderConfig, bool) {
	pc, ok := om.providers[provider]
	return pc, ok
}

// initProviders reads settings and creates oauth2.Config for each provider.
func (om *OAuthManager) initProviders() {
	ctx := context.Background()

	// GitHub
	om.initProvider(ctx, "github", GitHubEndpoint, []string{"user:email"}, fetchGitHubUser)

	// Google
	om.initProvider(ctx, "google", GoogleEndpoint, []string{"email", "profile"}, fetchGoogleUser)

	// Discord
	om.initProvider(ctx, "discord", DiscordEndpoint, []string{"identify", "email"}, fetchDiscordUser)
}

func (om *OAuthManager) initProvider(ctx context.Context, name string, endpoint oauth2.Endpoint, defaultScopes []string, fetchFn UserFetchFunc) {
	if om.deps.SettingService == nil {
		return
	}

	enabled, _ := om.deps.SettingService.Get(ctx, "oauth_"+name+"_enabled")
	if enabled != "true" {
		return
	}

	clientID, _ := om.deps.SettingService.Get(ctx, "oauth_"+name+"_client_id")
	clientSecret, _ := om.deps.SettingService.Get(ctx, "oauth_"+name+"_client_secret")
	callbackURL, _ := om.deps.SettingService.Get(ctx, "oauth_"+name+"_callback")

	if clientID == "" || clientSecret == "" {
		return
	}

	// Allow endpoint overrides via settings (for self-hosted/enterprise OAuth).
	authURL := readSetting(ctx, om.deps.SettingService, "oauth_"+name+"_auth_url", endpoint.AuthURL)
	tokenURL := readSetting(ctx, om.deps.SettingService, "oauth_"+name+"_token_url", endpoint.TokenURL)
	if authURL != endpoint.AuthURL || tokenURL != endpoint.TokenURL {
		endpoint = oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL}
	}

	om.providers[name] = &ProviderConfig{
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  callbackURL,
			Scopes:       defaultScopes,
			Endpoint:     endpoint,
		},
		FetchUser: fetchFn,
	}
}

// readSetting reads a setting or returns the default value.
func readSetting(ctx context.Context, sg pkg.SettingReader, key, defaultVal string) string {
	if sg == nil {
		return defaultVal
	}
	v, err := sg.Get(ctx, key)
	if err != nil || v == "" {
		return defaultVal
	}
	return v
}

// HandleLogin redirects to the OAuth provider's authorization page.
// It generates a state parameter, stores it in the KV store for CSRF validation,
// then redirects the user to the provider's auth URL.
func (om *OAuthManager) HandleLogin(ctx iris.Context) {
	provider := ctx.Params().Get("provider")
	pc, ok := om.providers[provider]
	if !ok {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "不支持的登录提供方: "+provider)
		return
	}

	state := generateState()

	// Store state in KV for CSRF validation (10 min TTL).
	if om.deps.KV != nil {
		_ = om.deps.KV.Set(ctx.Request().Context(), "oauth_state:"+state, provider, 10*time.Minute)
	}

	authURL := pc.Config.AuthCodeURL(state, oauth2.AccessTypeOnline)
	ctx.Redirect(authURL, http.StatusTemporaryRedirect)
}

// HandleCallback processes the OAuth callback from the provider.
// It validates the state, exchanges the authorization code for a token,
// fetches the user's profile from the provider, and returns the user info as JSON.
func (om *OAuthManager) HandleCallback(ctx iris.Context) {
	provider := ctx.Params().Get("provider")
	pc, ok := om.providers[provider]
	if !ok {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "不支持的登录提供方: "+provider)
		return
	}

	code := ctx.URLParam("code")
	state := ctx.URLParam("state")

	// Validate state for CSRF protection.
	if !validateState(ctx.Request().Context(), om.deps.KV, state, provider) {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "state 无效或已过期")
		return
	}

	// Exchange code for token.
	token, err := pc.Config.Exchange(ctx.Request().Context(), code)
	if err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "令牌交换失败: "+err.Error())
		return
	}

	// Fetch user info from the provider.
	user, err := pc.FetchUser(ctx.Request().Context(), token)
	if err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "拉取用户信息失败: "+err.Error())
		return
	}

	pkg.Success(ctx, map[string]any{"user": user})
}

// generateState produces a random hex string for OAuth CSRF protection.
func generateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// validateState checks that the state parameter matches what was stored in KV.
func validateState(ctx context.Context, kv kvstore.Store, state, expectedProvider string) bool {
	if state == "" {
		return false
	}
	if kv == nil {
		return false
	}
	val, err := kv.Get(ctx, "oauth_state:"+state)
	if err != nil || val != expectedProvider {
		return false
	}
	_ = kv.Delete(ctx, "oauth_state:"+state)
	return true
}
