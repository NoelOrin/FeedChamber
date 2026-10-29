package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// mockBanChecker implements pkg.BannedChecker for pipeline tests.
// The gateway only calls IsBanned; IsBannedInSpace exists because the seam
// interface is shared with the middleware layer (see internal/pkg/seams.go).
type mockBanChecker struct {
	banned   bool
	bannedIn map[string]bool
}

func (m *mockBanChecker) IsBanned(ctx context.Context, userUUID string) bool { return m.banned }

func (m *mockBanChecker) IsBannedInSpace(ctx context.Context, userUUID, spaceID string) bool {
	return m.bannedIn[spaceID]
}

// newAuthTestStore builds a SessionStore backed by in-memory SQLite + stub KV.
func newAuthTestStore(t *testing.T) (*SessionStore, *stubKV, *gorm.DB) {
	t.Helper()
	return newStore(t)
}

// seedSession creates a live session row (and its cache entry) for a user.
func seedSession(t *testing.T, store *SessionStore, sid, userUUID, username, role string, expiresAt int64) *model.UserSession {
	t.Helper()
	sess := &model.UserSession{
		SessionID: sid, UserUUID: userUUID, Username: username, Role: role,
		IP: "10.0.0.1", UserAgent: "curl/8", DeviceName: "curl/8",
		CreatedAt: time.Now().Unix(), ExpiresAt: expiresAt,
	}
	require.NoError(t, store.Create(context.Background(), sess))
	return sess
}

// setupApp creates an iris app with AuthChecker.Middleware wired in.
func setupApp(ac *AuthChecker) *iris.Application {
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !ac.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	return app
}

// setupOptionalApp creates an iris app with AuthChecker.OptionalMiddleware wired in.
func setupOptionalApp(ac *AuthChecker) *iris.Application {
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ac.OptionalMiddleware(ctx)
		ctx.Next()
	})
	return app
}

// echoHandler reports the context values the middleware is expected to set.
func echoHandler(ctx iris.Context) {
	_, hasSession := ctx.Values().Get("session").(*model.UserSession)
	ctx.JSON(iris.Map{
		"username":    ctx.Values().Get("username"),
		"user_uuid":   ctx.Values().Get("user_uuid"),
		"user_role":   ctx.Values().Get("user_role"),
		"has_session": hasSession,
		"auth_method": ctx.Values().GetString("auth_method"),
	})
}

func TestAuthChecker_Middleware_ValidSession_Bearer(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	sid := seedSession(t, store, "sid-alice", "uuid-1", "alice", "admin", time.Now().Add(time.Hour).Unix()).SessionID

	e := httptest.New(t, app)
	resp := e.GET("/test").WithHeader("Authorization", "Bearer "+sid).
		Expect().Status(http.StatusOK).JSON().Object().Raw()
	assert.Equal(t, "alice", resp["username"])
	assert.Equal(t, "uuid-1", resp["user_uuid"])
	assert.Equal(t, "admin", resp["user_role"])
	assert.True(t, resp["has_session"].(bool))
}

// The same opaque credential must work whether it arrives as a Bearer token
// or as the HttpOnly cookie — clients keep one contract, two transports.
func TestAuthChecker_Middleware_ValidSession_Cookie(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	sid := seedSession(t, store, "sid-cookie", "uuid-9", "cookie", "user", time.Now().Add(time.Hour).Unix()).SessionID

	e := httptest.New(t, app)
	resp := e.GET("/test").WithCookie(pkg.SessionCookie, sid).
		Expect().Status(http.StatusOK).JSON().Object().Raw()
	assert.Equal(t, "cookie", resp["username"])
	assert.Equal(t, "uuid-9", resp["user_uuid"])
	assert.True(t, resp["has_session"].(bool))
}

// Bearer wins over the cookie when both are present (and both resolve to the
// same credential for a session-authenticated browser).
func TestAuthChecker_Middleware_BearerTakesPriorityOverCookie(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	a := seedSession(t, store, "sid-a", "uuid-a", "alice", "user", time.Now().Add(time.Hour).Unix()).SessionID
	b := seedSession(t, store, "sid-b", "uuid-b", "bob", "user", time.Now().Add(time.Hour).Unix()).SessionID
	_ = b

	e := httptest.New(t, app)
	resp := e.GET("/test").
		WithHeader("Authorization", "Bearer "+a).
		WithCookie(pkg.SessionCookie, "decoy-not-a-session").
		Expect().Status(http.StatusOK).JSON().Object().Raw()
	assert.Equal(t, "alice", resp["username"])
}

func TestAuthChecker_Middleware_NoCredential(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1001)
}

func TestAuthChecker_Middleware_InvalidHeaderFormat(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Basic dGVzdDp0ZXN0").Expect().Status(http.StatusUnauthorized)
	e.GET("/test").WithHeader("Authorization", "NotBearer token").Expect().Status(http.StatusUnauthorized)
}

// Unknown / forged sids do not resolve, so they are not authenticated —
// the old "revoked" case is simply a lookup miss now.
func TestAuthChecker_Middleware_UnknownSession(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer not-a-real-session-id").
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1002)
}

// A revoked session (row deleted) must stop authenticating immediately.
func TestAuthChecker_Middleware_RevokedSessionStopsImmediately(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	sess := seedSession(t, store, "sid-revoke-me", "uuid-r", "revoked", "user", time.Now().Add(time.Hour).Unix())

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer "+sess.SessionID).Expect().Status(http.StatusOK)

	require.NoError(t, store.Revoke(context.Background(), sess.SessionID))

	e.GET("/test").WithHeader("Authorization", "Bearer "+sess.SessionID).
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1002)
}

// An expired session row (or cache entry) must also fail to resolve.
func TestAuthChecker_Middleware_ExpiredSession(t *testing.T) {
	store, kv, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	sess := seedSession(t, store, "sid-expired", "uuid-e", "expired", "user", time.Now().Add(time.Hour).Unix())
	// Age it past expiry on both row and cache.
	past := time.Now().Add(-time.Minute).Unix()
	store.db.Model(&model.UserSession{}).Where("session_id = ?", sess.SessionID).Update("expires_at", past)
	sess.ExpiresAt = past
	payload, err := json.Marshal(sess)
	require.NoError(t, err)
	kv.data[sessionCachePrefix+sess.SessionID] = string(payload)

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer "+sess.SessionID).
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1002)
}

// A JWT-shaped string is not a credential anymore: only live session ids and
// opaque oauth_tokens rows authenticate.
func TestAuthChecker_Middleware_LegacyJWTRejected(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	legacy := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX3V1aWQiOiJ1MSJ9.c2lnbmF0dXJl"

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer "+legacy).
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1002)
}

// seedOAuthToken stores an opaque token row and returns the plaintext, the way
// the provider issues it.
func seedOAuthToken(t *testing.T, store *SessionStore, raw, clientID, userID, scope string, expiresAt time.Time) *model.OAuthToken {
	t.Helper()
	row := &model.OAuthToken{
		ClientID:  clientID,
		UserID:    userID,
		TokenHash: hashOAuthToken(raw),
		Scope:     scope,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	require.NoError(t, store.db.Create(row).Error)
	return row
}

func TestAuthChecker_Middleware_OAuthClientTokenWorks(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	const raw = "of_at_client_token_plaintext"
	seedOAuthToken(t, store, raw, "client-1", "", "read write", time.Now().Add(time.Hour))

	e := httptest.New(t, app)
	resp := e.GET("/test").WithHeader("Authorization", "Bearer "+raw).
		Expect().Status(http.StatusOK).JSON().Object()
	resp.Value("auth_method").Equal("oauth_client")
	resp.Value("username").Equal("client-1")
}

func TestAuthChecker_Middleware_OAuthUserTokenUsesCurrentRole(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	user := &model.User{ID: "u-oauth", Username: "oauth-user", Role: "moderator"}
	require.NoError(t, store.db.Create(user).Error)

	const raw = "of_at_user_token_plaintext"
	seedOAuthToken(t, store, raw, "client-1", user.ID, "openid profile", time.Now().Add(time.Hour))

	e := httptest.New(t, app)
	resp := e.GET("/test").WithHeader("Authorization", "Bearer "+raw).
		Expect().Status(http.StatusOK).JSON().Object()
	resp.Value("auth_method").Equal("oauth_client")
	resp.Value("username").Equal("oauth-user")
	resp.Value("user_role").Equal("moderator")
}

func TestAuthChecker_Middleware_RevokedOAuthTokenRejected(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	const raw = "of_at_revoked_token_plaintext"
	row := seedOAuthToken(t, store, raw, "client-1", "", "read", time.Now().Add(time.Hour))
	require.NoError(t, store.db.Model(row).Update("revoked", true).Error)

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer "+raw).
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1014)
}

func TestAuthChecker_Middleware_ExpiredOAuthTokenRejected(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	const raw = "of_at_expired_token_plaintext"
	seedOAuthToken(t, store, raw, "client-1", "", "read", time.Now().Add(-time.Minute))

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer "+raw).
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1003)
}

// A user-bound token whose user row disappeared must stop authenticating.
func TestAuthChecker_Middleware_OAuthTokenForDeletedUserRejected(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	const raw = "of_at_orphan_token_plaintext"
	seedOAuthToken(t, store, raw, "client-1", "user-that-was-deleted", "read", time.Now().Add(time.Hour))

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer "+raw).
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().Value("code").Equal(1002)
}

func TestAuthChecker_OptionalMiddleware_NoCredential(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupOptionalApp(ac)
	app.Get("/test", echoHandler)

	e := httptest.New(t, app)
	resp := e.GET("/test").Expect().Status(http.StatusOK).JSON().Object().Raw()
	assert.False(t, resp["has_session"].(bool))
	assert.Empty(t, resp["username"], "anonymous optional call must not resolve a session")
}

func TestAuthChecker_OptionalMiddleware_ValidSession(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupOptionalApp(ac)
	app.Get("/test", echoHandler)

	sid := seedSession(t, store, "sid-opt", "uuid-4", "dave", "moderator", time.Now().Add(time.Hour).Unix()).SessionID

	e := httptest.New(t, app)
	resp := e.GET("/test").WithHeader("Authorization", "Bearer "+sid).
		Expect().Status(http.StatusOK).JSON().Object().Raw()
	assert.Equal(t, "dave", resp["username"])
	assert.Equal(t, "uuid-4", resp["user_uuid"])
	assert.Equal(t, "moderator", resp["user_role"])
}

func TestAuthChecker_OptionalMiddleware_InvalidTokenSilentlyAnonymous(t *testing.T) {
	store, _, _ := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: store.db, KV: store.kv})
	app := setupOptionalApp(ac)
	app.Get("/test", echoHandler)

	e := httptest.New(t, app)
	resp := e.GET("/test").WithHeader("Authorization", "Bearer bad-token").
		Expect().Status(http.StatusOK).JSON().Object().Raw()
	assert.Empty(t, resp["username"])
}

// Sliding renewal: a session below 50% lifetime is extended back to the full
// horizon and the cookie is re-issued.
func TestAuthChecker_Middleware_SlidingRenewal(t *testing.T) {
	store, _, db := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	// 3 days left of a 7-day TTL → below the 50% threshold.
	sess := seedSession(t, store, "sid-slide", "uuid-s", "slider", "user", time.Now().Add(3*24*time.Hour).Unix())

	e := httptest.New(t, app)
	resp := e.GET("/test").WithHeader("Authorization", "Bearer "+sess.SessionID).Expect().Status(http.StatusOK)

	var stored model.UserSession
	require.NoError(t, db.Where("session_id = ?", sess.SessionID).First(&stored).Error)
	assert.Greater(t, stored.ExpiresAt, sess.ExpiresAt, "renewal must push expires_at forward")
	assert.WithinDuration(t, time.Now().Add(model.SessionTTL), time.Unix(stored.ExpiresAt, 0), time.Minute)

	// Cookie re-issued with the new Max-Age.
	assert.True(t, hasSetCookie(resp.Raw().Cookies(), pkg.SessionCookie), "master_session cookie must be re-issued on renewal")
}

func TestAuthChecker_Middleware_FreshSessionNotRenewed(t *testing.T) {
	store, _, db := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: db, KV: store.kv})
	app := setupApp(ac)
	app.Get("/test", echoHandler)

	sess := seedSession(t, store, "sid-fresh", "uuid-f", "fresh", "user", time.Now().Add(model.SessionTTL).Unix())

	e := httptest.New(t, app)
	e.GET("/test").WithHeader("Authorization", "Bearer "+sess.SessionID).Expect().Status(http.StatusOK)

	var stored model.UserSession
	require.NoError(t, db.Where("session_id = ?", sess.SessionID).First(&stored).Error)
	assert.Equal(t, sess.ExpiresAt, stored.ExpiresAt, "a fresh session must not be touched")
}

func TestGateway_Handler_Integration(t *testing.T) {
	store, kv, db := newAuthTestStore(t)
	deps := Deps{
		DB:    db,
		KV:    kv,
		Perms: &mockPermChecker{hasPerm: func(role, code string) bool { return true }},
		Ban:   &mockBanChecker{},
	}
	gw := New(deps)

	app := iris.New()
	app.Use(gw.Preflight())
	app.Use(gw.Enforce())
	app.Get("/api/v1/dashboard", echoHandler)

	sid := seedSession(t, store, "sid-int", "uuid-6", "integrator", "admin", time.Now().Add(time.Hour).Unix()).SessionID

	e := httptest.New(t, app)
	resp := e.GET("/api/v1/dashboard").WithHeader("Authorization", "Bearer "+sid).
		Expect().Status(http.StatusOK).JSON().Object()
	resp.Value("username").Equal("integrator")
	resp.Value("user_role").Equal("admin")
}

func TestGateway_Handler_NoAuth_Unauthorized(t *testing.T) {
	_, kv, db := newAuthTestStore(t)
	deps := Deps{DB: db, KV: kv, Ban: &mockBanChecker{}}
	gw := New(deps)

	app := iris.New()
	app.Use(gw.Preflight())
	app.Use(gw.Enforce())
	app.Get("/protected", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	e.GET("/protected").Expect().Status(http.StatusUnauthorized)
}

// Test that Middleware returns false when stopped (for correct gateway pipeline behavior).
func TestAuthChecker_Middleware_ReturnFalseOnStop(t *testing.T) {
	_, kv, db := newAuthTestStore(t)
	ac := NewAuthChecker(Deps{DB: db, KV: kv})
	app := iris.New()
	called := false
	app.Use(func(ctx iris.Context) {
		result := ac.Middleware(ctx)
		assert.False(t, result)
		called = true
		ctx.StopExecution()
	})
	app.Get("/test", func(ctx iris.Context) {
		t.Fatal("should not reach handler")
	})

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusUnauthorized)
	assert.True(t, called)
}

func hasSetCookie(cookies []*http.Cookie, name string) bool {
	for _, c := range cookies {
		if c.Name == name {
			return true
		}
	}
	return false
}
