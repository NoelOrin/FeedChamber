package gateway

import (
	"context"
	"net/http"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSettingReader implements SettingReader for testing.
type testSettingReader map[string]string

func (m testSettingReader) Get(_ context.Context, key string) (string, error) {
	return m[key], nil
}

// newTestOAuthManager creates an OAuthManager with test settings.
func newTestOAuthManager(t *testing.T, settings testSettingReader) *OAuthManager {
	t.Helper()
	deps := Deps{
		SettingService: settings,
	}
	om := NewOAuthManager(deps)
	require.NotNil(t, om)
	return om
}

func TestOAuthManager_InitProviders_Enabled(t *testing.T) {
	settings := testSettingReader{
		"oauth_github_enabled":        "true",
		"oauth_github_client_id":      "gh-client",
		"oauth_github_client_secret":  "gh-secret",
		"oauth_google_enabled":        "true",
		"oauth_google_client_id":      "goog-client",
		"oauth_google_client_secret":  "goog-secret",
		"oauth_discord_enabled":       "true",
		"oauth_discord_client_id":     "dc-client",
		"oauth_discord_client_secret": "dc-secret",
	}
	om := newTestOAuthManager(t, settings)
	assert.Len(t, om.providers, 3)
	assert.Contains(t, om.providers, "github")
	assert.Contains(t, om.providers, "google")
	assert.Contains(t, om.providers, "discord")
}

func TestOAuthManager_InitProviders_Disabled(t *testing.T) {
	om := newTestOAuthManager(t, testSettingReader{})
	assert.Empty(t, om.providers)
}

func TestOAuthManager_InitProviders_MissingSecret(t *testing.T) {
	settings := testSettingReader{
		"oauth_github_enabled":   "true",
		"oauth_github_client_id": "gh-client",
	}
	om := newTestOAuthManager(t, settings)
	assert.Len(t, om.providers, 0, "provider without secret should not be registered")
}

func TestOAuthManager_EndpointOverrides(t *testing.T) {
	settings := testSettingReader{
		"oauth_github_enabled":       "true",
		"oauth_github_client_id":     "gh-client",
		"oauth_github_client_secret": "gh-secret",
		"oauth_github_auth_url":      "https://ghe.example.com/login/oauth/authorize",
		"oauth_github_token_url":     "https://ghe.example.com/login/oauth/access_token",
	}
	om := newTestOAuthManager(t, settings)
	gh, ok := om.providers["github"]
	require.True(t, ok)
	assert.Equal(t, "https://ghe.example.com/login/oauth/authorize", gh.Config.Endpoint.AuthURL)
	assert.Equal(t, "https://ghe.example.com/login/oauth/access_token", gh.Config.Endpoint.TokenURL)
}

func TestOAuthManager_HandleLogin_Redirect(t *testing.T) {
	settings := testSettingReader{
		"oauth_github_enabled":       "true",
		"oauth_github_client_id":     "gh-client",
		"oauth_github_client_secret": "gh-secret",
		"oauth_github_callback":      "https://example.com/oauth/callback",
	}
	om := NewOAuthManager(Deps{SettingService: settings})
	require.NotNil(t, om.providers["github"])

	state := generateState()
	url := om.providers["github"].Config.AuthCodeURL(state, oauth2.AccessTypeOnline)
	assert.Contains(t, url, "https://github.com/login/oauth/authorize")
	assert.Contains(t, url, "client_id=gh-client")
	assert.Contains(t, url, "redirect_uri=https%3A%2F%2Fexample.com%2Foauth%2Fcallback")
	assert.Contains(t, url, "response_type=code")
	assert.Contains(t, url, "state=")
}

func TestOAuthManager_HandleLogin_UnsupportedProvider(t *testing.T) {
	om := newTestOAuthManager(t, testSettingReader{})
	app := iris.New()
	app.Get("/oauth/{provider}", func(ctx iris.Context) {
		om.HandleLogin(ctx)
	})
	e := httptest.New(t, app)
	e.GET("/oauth/gitlab").Expect().Status(http.StatusBadRequest)
}

func TestOAuthManager_HandleCallback_InvalidState(t *testing.T) {
	om := newTestOAuthManager(t, testSettingReader{})
	app := iris.New()
	app.Get("/oauth/{provider}/callback", func(ctx iris.Context) {
		om.HandleCallback(ctx)
	})

	e := httptest.New(t, app)
	e.GET("/oauth/github/callback").WithQuery("code", "test").WithQuery("state", "invalid").
		Expect().Status(http.StatusBadRequest)
}

func TestOAuthManager_HandleCallback_UnsupportedProvider(t *testing.T) {
	om := newTestOAuthManager(t, testSettingReader{})
	app := iris.New()
	app.Get("/oauth/{provider}/callback", func(ctx iris.Context) {
		om.HandleCallback(ctx)
	})
	e := httptest.New(t, app)
	e.GET("/oauth/gitlab/callback").WithQuery("code", "test").WithQuery("state", "test").
		Expect().Status(http.StatusBadRequest)
}

func TestGenerateState_UniqueAndLength(t *testing.T) {
	s1 := generateState()
	s2 := generateState()
	assert.Equal(t, 32, len(s1))
	assert.Equal(t, 32, len(s2))
	assert.NotEqual(t, s1, s2)
}

// validateState is the OAuth CSRF gate. It must fail closed when there is no KV
// to verify against, and a state may only be used once.
func TestValidateState_NilKVFailsClosed(t *testing.T) {
	require.False(t, validateState(context.Background(), nil, "abc", "github"),
		"a state cannot be validated without a KV; accepting it would defeat the CSRF check")
}

func TestValidateState_MatchConsumesState(t *testing.T) {
	ctx := context.Background()
	kv := newStubKV()
	require.NoError(t, kv.Set(ctx, "oauth_state:xyz", "github", time.Minute))

	require.True(t, validateState(ctx, kv, "xyz", "github"))
	require.False(t, validateState(ctx, kv, "xyz", "github"), "state must be consumed after one use")

	require.NoError(t, kv.Set(ctx, "oauth_state:abc", "github", time.Minute))
	require.False(t, validateState(ctx, kv, "abc", "gitlab"), "provider must match")
	require.False(t, validateState(ctx, kv, "", "github"), "empty state is never valid")
}
