package gateway

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"feedchamber/internal/model"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates an in-memory SQLite database and migrates OAuth models.
func testHashedSecret(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("test-client-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.OAuthClient{}, &model.OAuthAuthorize{}, &model.OAuthToken{}, &model.User{})
	require.NoError(t, err)
	return db
}

// newTestProvider creates an OAuthProvider backed by a test DB.
func newTestProvider(t *testing.T, db *gorm.DB) *OAuthProvider {
	t.Helper()
	return NewOAuthProvider(Deps{DB: db})
}

// newTestApp creates an Iris app with the provider routes registered for testing.
func newTestApp(t *testing.T, db *gorm.DB) *iris.Application {
	t.Helper()
	p := newTestProvider(t, db)

	app := iris.New()
	// Simulate the gateway credential middleware: set user context values for
	// the protected routes.
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("username", "admin")
		ctx.Values().Set("user_uuid", "test-user-ulid")
		ctx.Values().Set("user_role", "admin")
		ctx.Next()
	})

	// Register provider routes matching the real router pattern
	v1 := app.Party("/api/v1")
	v1.Post("/oauth/clients", p.RegisterClient)
	v1.Post("/oauth/clients/list", p.ListClients)
	v1.Post("/oauth/clients/{id}/update", p.UpdateClient)
	v1.Post("/oauth/clients/{id}/reset-secret", p.ResetClientSecret)
	v1.Post("/oauth/clients/{id}/delete", p.DeleteClient)
	v1.Get("/oauth/authorize", p.Authorize)
	v1.Post("/oauth/authorize", p.Authorize)
	v1.Post("/oauth/token", p.Token)
	v1.Post("/oauth/revoke", p.Revoke)

	return app
}

// ── RegisterClient tests ─────────────────────────────────────────────────────

func TestOAuthProvider_RegisterClient_Success(t *testing.T) {
	db := setupTestDB(t)
	app := newTestApp(t, db)

	e := httptest.New(t, app)
	resp := e.POST("/api/v1/oauth/clients").
		WithJSON(map[string]any{
			"name":          "Test App",
			"redirect_uris": `["https://example.com/callback"]`,
			"scopes":        "read write",
		}).
		Expect().
		Status(http.StatusOK).
		JSON().Object()

	resp.Value("code").Number().IsEqual(0)
	resp.Value("data").Object().Value("name").String().IsEqual("Test App")
	resp.Value("data").Object().Value("client_id").String().NotEmpty()
	resp.Value("data").Object().Value("client_secret").String().NotEmpty()

	// Verify the client was stored in DB
	var count int64
	db.Model(&model.OAuthClient{}).Count(&count)
	assert.Equal(t, int64(1), count)

	// The issued secret must be usable: bcrypt's 72-byte input limit used to
	// break every registration (the secret was 80 hex chars).
	secret := resp.Value("data").Object().Value("client_secret").String().Raw()
	clientID := resp.Value("data").Object().Value("client_id").String().Raw()

	tokenApp := iris.New()
	tokenApp.Post("/token", newTestProvider(t, db).Token)
	e2 := httptest.New(t, tokenApp)
	e2.POST("/token").
		WithFormField("grant_type", "client_credentials").
		WithFormField("client_id", clientID).
		WithFormField("client_secret", secret).
		WithFormField("scope", "read").
		Expect().Status(http.StatusOK)
}

func TestOAuthProvider_RegisterClient_MissingName(t *testing.T) {
	db := setupTestDB(t)
	app := newTestApp(t, db)

	e := httptest.New(t, app)
	e.POST("/api/v1/oauth/clients").
		WithJSON(map[string]any{
			"redirect_uris": `["https://example.com/callback"]`,
		}).
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("code").Number().IsEqual(2001)
}

func TestOAuthProvider_RegisterClient_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	app := newTestApp(t, db)

	e := httptest.New(t, app)
	e.POST("/api/v1/oauth/clients").
		WithText("not json").
		WithHeader("Content-Type", "application/json").
		Expect().
		Status(http.StatusBadRequest)
}

func TestOAuthProvider_RegisterClient_InvalidRedirectURIs(t *testing.T) {
	db := setupTestDB(t)
	app := newTestApp(t, db)

	e := httptest.New(t, app)
	e.POST("/api/v1/oauth/clients").
		WithJSON(map[string]any{
			"name":          "Test App",
			"redirect_uris": "not-json",
		}).
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("code").Number().IsEqual(2001)
}

func TestOAuthProvider_ListClients_Success(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&model.OAuthClient{
		ClientID:     "list-client",
		ClientSecret: "hashed",
		Name:         "List App",
		RedirectURIs: `["https://example.com/callback"]`,
		Scopes:       "read",
		CreatedAt:    time.Now().Unix(),
	}).Error)
	app := newTestApp(t, db)

	e := httptest.New(t, app)
	resp := e.POST("/api/v1/oauth/clients/list").
		WithJSON(map[string]any{}).
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	resp.Value("code").Number().IsEqual(0)
	resp.Value("data").Array().Length().IsEqual(1)
	resp.Value("data").Array().First().Object().Value("client_id").String().IsEqual("list-client")
}

func TestOAuthProvider_DeleteClient_Success(t *testing.T) {
	db := setupTestDB(t)
	client := &model.OAuthClient{
		ClientID:     "delete-client",
		ClientSecret: "hashed",
		Name:         "Delete App",
		RedirectURIs: `["https://example.com/callback"]`,
		CreatedAt:    time.Now().Unix(),
	}
	require.NoError(t, db.Create(client).Error)
	app := newTestApp(t, db)

	e := httptest.New(t, app)
	e.POST("/api/v1/oauth/clients/" + fmt.Sprint(client.ID) + "/delete").
		WithJSON(map[string]any{}).
		Expect().
		Status(http.StatusOK)
	var count int64
	require.NoError(t, db.Model(&model.OAuthClient{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

// ── Authorize tests ─────────────────────────────────────────────────────────

func TestOAuthProvider_Authorize_Success(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	// Pre-register a client
	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
		CreatedAt:    0,
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("username", "testuser")
		ctx.Values().Set("user_uuid", "test-user-ulid")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Get("/authorize", p.Authorize)
	app.Post("/authorize", p.Authorize)

	e := httptest.New(t, app)

	// GET request
	resp := e.GET("/authorize").
		WithQuery("response_type", "code").
		WithQuery("client_id", "test-client-id").
		WithQuery("state", "xyz123").
		WithQuery("scope", "read").
		Expect().
		Status(http.StatusOK).
		JSON().Object()

	resp.Value("code").Number().IsEqual(0)
	resp.Value("data").Object().Value("code").String().NotEmpty()
	resp.Value("data").Object().Value("state").String().IsEqual("xyz123")
}

func TestOAuthProvider_Authorize_RejectsUnknownRedirectURI(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)
	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
		RedirectURIs: `["https://example.com/callback"]`,
		CreatedAt:    0,
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("username", "testuser")
		ctx.Values().Set("user_uuid", "test-user-ulid")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Get("/authorize", p.Authorize)

	e := httptest.New(t, app)
	e.GET("/authorize").
		WithQuery("response_type", "code").
		WithQuery("client_id", "test-client-id").
		WithQuery("redirect_uri", "https://evil.example/callback").
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid_request")
}

func TestOAuthProvider_Authorize_InvalidClient(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("username", "testuser")
		ctx.Values().Set("user_uuid", "test-user-ulid")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Get("/authorize", p.Authorize)

	e := httptest.New(t, app)
	e.GET("/authorize").
		WithQuery("response_type", "code").
		WithQuery("client_id", "nonexistent-client").
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid_client")
}

func TestOAuthProvider_Authorize_MissingClientID(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("username", "testuser")
		ctx.Values().Set("user_uuid", "test-user-ulid")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Get("/authorize", p.Authorize)

	e := httptest.New(t, app)
	e.GET("/authorize").
		WithQuery("response_type", "code").
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid_request")
}

func TestOAuthProvider_Authorize_NoAuth(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	// Pre-register a client
	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
		CreatedAt:    0,
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	// No auth middleware — user_uuid will be empty
	app.Get("/authorize", p.Authorize)

	e := httptest.New(t, app)
	e.GET("/authorize").
		WithQuery("response_type", "code").
		WithQuery("client_id", "test-client-id").
		Expect().
		Status(http.StatusUnauthorized)
}

// ── Token tests ─────────────────────────────────────────────────────────────

func TestOAuthProvider_Token_AuthorizationCode_Success(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	// Pre-register a client
	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
		Scopes:       "read write",
	}
	require.NoError(t, db.Create(client).Error)

	// Create a test user (the token row is bound to their id)
	user := &model.User{
		ID:       "test-user-ulid",
		Username: "testuser",
		Email:    "testuser@example.com",
		Role:     "user",
	}
	require.NoError(t, db.Create(user).Error)

	// Create an authorization code
	authCode := &model.OAuthAuthorize{
		Code:      "test-auth-code",
		ClientID:  "test-client-id",
		UserID:    "test-user-ulid",
		Scope:     "read",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	require.NoError(t, db.Create(authCode).Error)

	app := iris.New()
	app.Post("/token", p.Token)

	e := httptest.New(t, app)
	resp := e.POST("/token").
		WithFormField("grant_type", "authorization_code").
		WithFormField("code", "test-auth-code").
		WithFormField("client_id", "test-client-id").
		WithFormField("client_secret", "test-client-secret").
		Expect().
		Status(http.StatusOK).
		JSON().Object()

	raw := resp.Value("access_token").String().Raw()
	assert.True(t, strings.HasPrefix(raw, oauthAccessTokenPrefix), "access token must be opaque, got %q", raw)
	resp.Value("token_type").String().IsEqual("Bearer")
	resp.Value("scope").String().IsEqual("read")
	// No refresh token: the provider supports no refresh_token grant.
	resp.NotContainsKey("refresh_token")

	var row model.OAuthToken
	require.NoError(t, db.Where("access_token = ?", hashOAuthToken(raw)).First(&row).Error)
	assert.Equal(t, "test-user-ulid", row.UserID)
	assert.Equal(t, "test-client-id", row.ClientID)

	// Authorization codes are one-time use.
	var remaining int64
	require.NoError(t, db.Model(&model.OAuthAuthorize{}).Where("code = ?", "test-auth-code").Count(&remaining).Error)
	assert.Zero(t, remaining)
}

func TestOAuthProvider_Token_AuthorizationCode_InvalidCode(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
		Scopes:       "read write",
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	app.Post("/token", p.Token)

	e := httptest.New(t, app)
	e.POST("/token").
		WithFormField("grant_type", "authorization_code").
		WithFormField("code", "nonexistent-code").
		WithFormField("client_id", "test-client-id").
		WithFormField("client_secret", "test-client-secret").
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid_grant")
}

func TestOAuthProvider_Token_ClientCredentials_Success(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
		Scopes:       "read write",
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	app.Post("/token", p.Token)

	e := httptest.New(t, app)
	resp := e.POST("/token").
		WithFormField("grant_type", "client_credentials").
		WithFormField("client_id", "test-client-id").
		WithFormField("client_secret", "test-client-secret").
		WithFormField("scope", "read write").
		Expect().
		Status(http.StatusOK).
		JSON().Object()

	raw := resp.Value("access_token").String().Raw()
	assert.True(t, strings.HasPrefix(raw, oauthAccessTokenPrefix), "access token must be opaque, got %q", raw)
	resp.Value("token_type").String().IsEqual("Bearer")
	resp.Value("scope").String().IsEqual("read write")
	// No refresh token is issued: the provider has no refresh_token grant.
	resp.NotContainsKey("refresh_token")

	// The plaintext is never stored; only its digest is.
	var row model.OAuthToken
	require.NoError(t, db.Where("access_token = ?", hashOAuthToken(raw)).First(&row).Error)
	assert.Empty(t, row.UserID, "client_credentials token is not bound to a user")
	assert.Equal(t, "read write", row.Scope)
	assert.False(t, row.Revoked)
	assert.True(t, row.ExpiresAt.After(time.Now()))
}

func TestOAuthProvider_Token_ClientCredentials_InvalidScope(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
		Scopes:       "read",
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	app.Post("/token", p.Token)

	e := httptest.New(t, app)
	e.POST("/token").
		WithFormField("grant_type", "client_credentials").
		WithFormField("client_id", "test-client-id").
		WithFormField("client_secret", "test-client-secret").
		WithFormField("scope", "write").
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid_scope")
}

func TestOAuthProvider_Token_InvalidClient(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	app := iris.New()
	app.Post("/token", p.Token)

	e := httptest.New(t, app)
	e.POST("/token").
		WithFormField("grant_type", "client_credentials").
		WithFormField("client_id", "bad-client").
		WithFormField("client_secret", "bad-secret").
		Expect().
		Status(http.StatusUnauthorized)
}

func TestOAuthProvider_Token_UnsupportedGrantType(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	app := iris.New()
	app.Post("/token", p.Token)

	e := httptest.New(t, app)
	e.POST("/token").
		WithFormField("grant_type", "password").
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("unsupported_grant_type")
}

func TestOAuthProvider_Token_MissingGrantType(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	app := iris.New()
	app.Post("/token", p.Token)

	e := httptest.New(t, app)
	e.POST("/token").
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid_request")
}

// ── Revoke tests ────────────────────────────────────────────────────────────

func TestOAuthProvider_Revoke_Success(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
	}
	require.NoError(t, db.Create(client).Error)

	// Issue a real opaque token (the plaintext is what the client would hold;
	// only its digest is stored).
	tokenStr, hash, err := newOAuthAccessToken()
	require.NoError(t, err)

	tokenRecord := &model.OAuthToken{
		ClientID:  "test-client-id",
		UserID:    "test-user-ulid",
		TokenHash: hash,
		Scope:     "read",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, db.Create(tokenRecord).Error)

	app := iris.New()
	app.Post("/revoke", p.Revoke)

	e := httptest.New(t, app)
	e.POST("/revoke").
		WithFormField("token", tokenStr).
		WithFormField("client_id", "test-client-id").
		WithFormField("client_secret", "test-client-secret").
		Expect().
		Status(http.StatusOK)

	// Verify the token was marked as revoked
	var revokedToken model.OAuthToken
	err = db.Where("access_token = ?", hash).First(&revokedToken).Error
	require.NoError(t, err)
	assert.True(t, revokedToken.Revoked)
}

func TestOAuthProvider_Revoke_InvalidToken(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	app.Post("/revoke", p.Revoke)

	e := httptest.New(t, app)
	// Per RFC 7009 §2.2, revoking an invalid token still returns 200
	e.POST("/revoke").
		WithFormField("token", "invalid-token-string").
		WithFormField("client_id", "test-client-id").
		WithFormField("client_secret", "test-client-secret").
		Expect().
		Status(http.StatusOK)
}

func TestOAuthProvider_Revoke_MissingToken(t *testing.T) {
	db := setupTestDB(t)
	p := newTestProvider(t, db)

	client := &model.OAuthClient{
		ClientID:     "test-client-id",
		ClientSecret: testHashedSecret(t),
		Name:         "Test App",
	}
	require.NoError(t, db.Create(client).Error)

	app := iris.New()
	app.Post("/revoke", p.Revoke)

	e := httptest.New(t, app)
	e.POST("/revoke").
		WithFormField("client_id", "test-client-id").
		WithFormField("client_secret", "test-client-secret").
		Expect().
		Status(http.StatusBadRequest)
}

// ── Helper tests ────────────────────────────────────────────────────────────

func TestRandomString_Length(t *testing.T) {
	s := randomString(16)
	assert.Equal(t, 32, len(s)) // hex encoding doubles length
	s2 := randomString(32)
	assert.Equal(t, 64, len(s2))
	assert.NotEqual(t, s, s2)
}

func TestNewOAuthAccessToken_Format(t *testing.T) {
	raw, hash, err := newOAuthAccessToken()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(raw, oauthAccessTokenPrefix))
	assert.Greater(t, len(raw), len(oauthAccessTokenPrefix)+30)
	assert.Equal(t, 64, len(hash), "stored digest is sha256 hex")
	assert.Equal(t, hash, hashOAuthToken(raw), "hash must be deterministic")
	assert.NotEqual(t, raw, hash, "plaintext and digest must differ")

	raw2, hash2, err := newOAuthAccessToken()
	require.NoError(t, err)
	assert.NotEqual(t, raw, raw2)
	assert.NotEqual(t, hash, hash2)
}

func TestOAuthError(t *testing.T) {
	errResp := oauthError("invalid_client", "Bad credentials")
	assert.Equal(t, "invalid_client", errResp["error"])
	assert.Equal(t, "Bad credentials", errResp["error_description"])
}

func TestErrInvalidClient(t *testing.T) {
	err := ErrInvalidClient
	assert.Equal(t, "invalid client credentials", err.Error())
}
