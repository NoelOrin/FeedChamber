package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg/testnats"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestOAuthService builds a concrete *oauthService for white-box tests.
// userRepo/oauthRepo are nil because the config + auth-url paths under test
// never touch them. kv is a real NATS store (GetAuthURL writes state).
func newTestOAuthService(t *testing.T, settings *mock.SettingService) *oauthService {
	t.Helper()
	if settings == nil {
		settings = mock.NewSettingService()
	}
	kv := testnats.NewKVStore(t, "oauth_test")
	return &oauthService{settingSvc: settings, kv: kv}
}

func TestGetProviderConfigFromDB_OverridesEndpointsAndScopes(t *testing.T) {
	settings := mock.NewSettingService().
		WithSetting("oauth_github_auth_url", "https://ghe.example.com/login/oauth/authorize").
		WithSetting("oauth_github_token_url", "https://ghe.example.com/login/oauth/access_token").
		WithSetting("oauth_github_user_api_url", "https://ghe.example.com/api/v3/user").
		WithSetting("oauth_github_scopes", "read:user repo")
	svc := newTestOAuthService(t, settings)

	cfg, err := svc.getProviderConfigFromDB(context.Background(), model.ProviderGitHub)
	require.NoError(t, err)

	assert.Equal(t, "https://ghe.example.com/login/oauth/authorize", cfg.AuthURL)
	assert.Equal(t, "https://ghe.example.com/login/oauth/access_token", cfg.TokenURL)
	assert.Equal(t, "https://ghe.example.com/api/v3/user", cfg.UserAPIURL)
	assert.Equal(t, []string{"read:user", "repo"}, cfg.Scopes)
}

func TestGetProviderConfigFromDB_FallsBackToDefaults(t *testing.T) {
	// No overrides set — every endpoint/scope must equal the hardcoded baseline.
	svc := newTestOAuthService(t, mock.NewSettingService())

	cfg, err := svc.getProviderConfigFromDB(context.Background(), model.ProviderGitHub)
	require.NoError(t, err)

	want := model.OAuthProviderConfigs[model.ProviderGitHub]
	assert.Equal(t, want.AuthURL, cfg.AuthURL)
	assert.Equal(t, want.TokenURL, cfg.TokenURL)
	assert.Equal(t, want.UserAPIURL, cfg.UserAPIURL)
	assert.Equal(t, want.Scopes, cfg.Scopes)
}

func TestGetProviderConfigFromDB_EmptyScopesFallback(t *testing.T) {
	// Weibo's default scope list is empty; with no override it must stay empty.
	svc := newTestOAuthService(t, mock.NewSettingService())

	cfg, err := svc.getProviderConfigFromDB(context.Background(), model.ProviderWeibo)
	require.NoError(t, err)

	assert.Empty(t, cfg.Scopes)
}

func TestGetAuthURL_UsesOverriddenAuthURLAndScopes(t *testing.T) {
	settings := mock.NewSettingService().
		WithSetting("oauth_github_enabled", "true").
		WithSetting("oauth_github_client_id", "client-123").
		WithSetting("oauth_github_callback", "https://forum.example.com/callback").
		WithSetting("oauth_github_auth_url", "https://ghe.example.com/login/oauth/authorize").
		WithSetting("oauth_github_scopes", "read:user repo")
	svc := newTestOAuthService(t, settings)

	authURL, err := svc.GetAuthURL(context.Background(), model.ProviderGitHub, "state-xyz")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(authURL, "https://ghe.example.com/login/oauth/authorize?"),
		"auth URL should start with the overridden endpoint, got: %s", authURL)
	// Scopes are space-joined then URL-encoded ("read:user repo" -> "read%3Auser+repo").
	assert.Contains(t, authURL, "scope=read%3Auser+repo")
	assert.Contains(t, authURL, "client_id=client-123")
}

func TestGetUserUserProfile_UsesProvidedURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok-abc", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"login":"octocat","name":"The Octocat","email":"octo@example.com","avatar_url":"https://avatars/x"}`))
	}))
	defer srv.Close()

	svc := newTestOAuthService(t, nil)

	user, err := svc.GetUserUserProfile(model.ProviderGitHub, "tok-abc", srv.URL)
	require.NoError(t, err)

	assert.Equal(t, "42", user.ID)
	assert.Equal(t, "The Octocat", user.Name)
	assert.Equal(t, "octo@example.com", user.Email)
	assert.Equal(t, string(model.ProviderGitHub), user.Provider)
}
