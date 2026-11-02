package gateway

import (
	"context"
	"strings"
	"testing"

	golangjwt "github.com/golang-jwt/jwt/v5"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/require"

	"feedchamber/internal/model"
)

// ID tokens are the only JWTs left in the system (OIDC Core requires them).
// These tests pin the signing-key lifecycle and the RS256 verification path.

func newIDTokenTestProvider(settings map[string]string) *SSOProvider {
	deps := Deps{}
	if settings != nil {
		deps.SettingService = testSettingReader(settings)
	}
	p := NewSSOProvider(deps)
	p.loadSettings(context.Background())
	return p
}

// The signing key must be stable for the lifetime of the provider: a key that
// regenerated per call would rotate the JWKS under relying parties.
func TestSSOProvider_JWKS_StableAcrossCalls(t *testing.T) {
	p := newIDTokenTestProvider(nil)

	first := p.jwks(context.Background())
	second := p.jwks(context.Background())
	require.Equal(t, first, second)

	keys, ok := first["keys"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, keys, 1)
	require.Equal(t, "RS256", keys[0]["alg"])
	require.NotEmpty(t, keys[0]["kid"])
	require.NotEmpty(t, keys[0]["n"])
	require.NotEmpty(t, keys[0]["e"])
}

func TestSSOProvider_GenerateIDToken_VerifiesRS256(t *testing.T) {
	p := newIDTokenTestProvider(nil)

	token, err := p.generateIDToken(context.Background(), "https://issuer.example", "u1", "client", "Alice", "alice", "nonce")
	require.NoError(t, err)

	key, kid := p.signingKey(context.Background())
	require.NotNil(t, key)

	parsed, err := golangjwt.Parse(token, func(tok *golangjwt.Token) (interface{}, error) {
		require.Equal(t, "RS256", tok.Method.Alg())
		require.Equal(t, kid, tok.Header["kid"])
		return &key.PublicKey, nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	claims, ok := parsed.Claims.(golangjwt.MapClaims)
	require.True(t, ok)
	require.Equal(t, "https://issuer.example", claims["iss"])
	require.Equal(t, "u1", claims["sub"])
	require.Equal(t, "client", claims["aud"])
	require.Equal(t, "nonce", claims["nonce"])
}

// A fresh provider instance must reuse the persisted key instead of minting a
// new one, otherwise every restart invalidates previously issued id_tokens.
func TestSSOProvider_SigningKeyPersistedAcrossInstances(t *testing.T) {
	store := &memorySettingWriter{values: map[string]string{}}
	first := NewSSOProvider(Deps{SettingService: store})
	firstKey, firstKid := first.signingKey(context.Background())
	require.NotNil(t, firstKey)
	require.NotEmpty(t, store.values[ssoSigningKeySetting], "generated key must be persisted")

	second := NewSSOProvider(Deps{SettingService: store})
	secondKey, secondKid := second.signingKey(context.Background())
	require.NotNil(t, secondKey)
	require.Equal(t, firstKey.PublicKey.N, secondKey.PublicKey.N)
	require.Equal(t, firstKid, secondKid)
}

// memorySettingWriter is a read/write settings fake proving the persistence
// path; testSettingReader is read-only, which exercises the in-process fallback.
type memorySettingWriter struct {
	values map[string]string
}

func (m *memorySettingWriter) Get(_ context.Context, key string) (string, error) {
	return m.values[key], nil
}

func (m *memorySettingWriter) Set(_ context.Context, key, value string) error {
	m.values[key] = value
	return nil
}

func TestSSOProvider_New_NilService(t *testing.T) {
	p := NewSSOProvider(Deps{SettingService: nil})
	require.NotNil(t, p)
}

func TestSSOProvider_Token_NoAuth(t *testing.T) {
	p := NewSSOProvider(Deps{SettingService: testSettingReader{
		"sso_oidc_enabled": "true",
		"sso_oidc_issuer":  "https://forum.example.com",
	}})
	app := iris.New()
	app.Post("/sso/token", func(ctx iris.Context) { p.Token(ctx) })
	e := httptest.New(t, app)
	e.POST("/sso/token").Expect().Status(400)
}

// The OIDC token response carries an opaque access token plus a signed JWT
// id_token; userinfo accepts the former and must reject the latter.
func TestSSOProvider_TokenIssuesOpaqueAccessAndJWTIDToken(t *testing.T) {
	db := setupTestDB(t)
	user := &model.User{ID: "u-sso", Username: "sso-user", DisplayName: "SSO User", Role: "user"}
	require.NoError(t, db.Create(user).Error)

	kv := newStubKV()
	const code = "sso-code-1"
	kv.data[ssoCodeKey(code)] = (&ssoCodeData{
		Code:     code,
		ClientID: "sso-client",
		UserUUID: user.ID,
		Scope:    "openid profile",
		Nonce:    "n-1",
	}).String()

	p := NewSSOProvider(Deps{
		SettingService: testSettingReader{
			"sso_oidc_enabled":       "true",
			"sso_oidc_issuer":        "https://forum.example.com",
			"sso_oidc_client_id":     "sso-client",
			"sso_oidc_client_secret": "sso-secret",
		},
		KV: kv,
		DB: db,
	})
	app := iris.New()
	app.Post("/sso/token", func(ctx iris.Context) { p.Token(ctx) })
	app.Get("/sso/userinfo", func(ctx iris.Context) { p.UserInfo(ctx) })

	e := httptest.New(t, app)
	resp := e.POST("/sso/token").
		WithFormField("grant_type", "authorization_code").
		WithFormField("code", code).
		WithFormField("client_id", "sso-client").
		WithFormField("client_secret", "sso-secret").
		Expect().Status(200).JSON().Object()

	accessToken := resp.Value("access_token").String().Raw()
	require.True(t, strings.HasPrefix(accessToken, oauthAccessTokenPrefix), "access token must be opaque, got %q", accessToken)

	idToken := resp.Value("id_token").String().Raw()
	key, kid := p.signingKey(context.Background())
	parsed, err := golangjwt.Parse(idToken, func(tok *golangjwt.Token) (interface{}, error) {
		require.Equal(t, kid, tok.Header["kid"])
		return &key.PublicKey, nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	claims, ok := parsed.Claims.(golangjwt.MapClaims)
	require.True(t, ok)
	require.Equal(t, "u-sso", claims["sub"])
	require.Equal(t, "n-1", claims["nonce"])
	require.Equal(t, "https://forum.example.com", claims["iss"])

	// The opaque access token authenticates at userinfo...
	e.GET("/sso/userinfo").WithHeader("Authorization", "Bearer "+accessToken).
		Expect().Status(200).JSON().Object().Value("sub").String().IsEqual("u-sso")

	// ...while the id_token is a proof of authentication, not a credential.
	e.GET("/sso/userinfo").WithHeader("Authorization", "Bearer "+idToken).
		Expect().Status(401)
}
