package gateway

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupSessionDB creates an in-memory SQLite DB with the user_sessions table.
func setupSessionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.UserSession{}))
	return db
}

// newSessionManager builds a SessionManager (and its backing store) over a
// fresh in-memory DB + stub KV.
func newSessionManager(t *testing.T) (*SessionManager, *SessionStore, *stubKV, *gorm.DB) {
	t.Helper()
	db := setupSessionDB(t)
	kv := newStubKV()
	deps := Deps{DB: db, KV: kv}
	store := NewSessionStore(deps)
	require.NotNil(t, store)
	return NewSessionManager(deps), store, kv, db
}

// ── TrackingMiddleware ─────────────────────────────────────────────────────

// Tracking only bumps last_active on an existing row — Login owns creation.
func TestSessionManager_TrackingMiddleware_TouchesExisting(t *testing.T) {
	sm, store, _, db := newSessionManager(t)
	old := time.Now().Add(-time.Hour).Unix()
	require.NoError(t, store.Create(context.Background(), &model.UserSession{
		SessionID: "sid-track", UserUUID: "uuid-1", Username: "alice", Role: "user",
		IP: "1.2.3.4", UserAgent: "old-agent", LastActive: old,
		CreatedAt: old, ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}))
	sess, err := store.Lookup(context.Background(), "sid-track")
	require.NoError(t, err)
	sess.LastActive = old // pretend the cache predates the window

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("session", sess)
		ctx.Values().Set("user_uuid", "uuid-1")
		sm.TrackingMiddleware(ctx)
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusOK)

	var updated model.UserSession
	require.NoError(t, db.Where("session_id = ?", "sid-track").First(&updated).Error)
	assert.Greater(t, updated.LastActive, old)
}

// The tracking middleware never creates rows, even when a session is
// authenticated but its row vanished.
func TestSessionManager_TrackingMiddleware_NeverCreatesRows(t *testing.T) {
	sm, _, _, db := newSessionManager(t)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("session", &model.UserSession{
			SessionID: "ghost", UserUUID: "uuid-x", ExpiresAt: time.Now().Add(time.Hour).Unix(),
		})
		sm.TrackingMiddleware(ctx)
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusOK)

	var count int64
	db.Model(&model.UserSession{}).Where("session_id = ?", "ghost").Count(&count)
	assert.Equal(t, int64(0), count, "tracking must not insert")
}

func TestSessionManager_TrackingMiddleware_NoSession_Noop(t *testing.T) {
	sm, _, _, db := newSessionManager(t)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		sm.TrackingMiddleware(ctx) // no session value set
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusOK)

	var count int64
	db.Model(&model.UserSession{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

// ── ListSessions ────────────────────────────────────────────────────────────

func TestSessionManager_ListSessions(t *testing.T) {
	sm, store, _, db := newSessionManager(t)
	now := time.Now()

	mk := func(sid string, lastActive time.Time, expiresAt time.Time) {
		require.NoError(t, db.Create(&model.UserSession{
			SessionID: sid, UserUUID: "uuid-3", Username: "charlie", Role: "user",
			IP: "10.0.0.1", UserAgent: "Mozilla/5.0", DeviceName: "Chrome on Mac",
			LastActive: lastActive.Unix(), CreatedAt: now.Add(-time.Hour).Unix(), ExpiresAt: expiresAt.Unix(),
		}).Error)
	}
	mk("sess-1", now, now.Add(23*time.Hour))
	mk("sess-2", now.Add(-30*time.Minute), now.Add(22*time.Hour))
	mk("sess-expired", now.Add(-48*time.Hour), now.Add(-time.Hour))

	current, err := store.Lookup(context.Background(), "sess-1")
	// the expired row must not resolve
	require.NoError(t, err)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("session", current)
		ctx.Values().Set("user_uuid", "uuid-3")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Get("/sessions", sm.ListSessions)

	e := httptest.New(t, app)
	resp := e.GET("/sessions").Expect().Status(http.StatusOK).JSON().Object()

	resp.Value("code").Equal(0)
	resp.Value("msg").Equal("success")
	data := resp.Value("data").Array()
	data.Length().Equal(2) // only active sessions

	first := data.First().Object()
	first.Value("session_id").Equal("sess-1")
	first.Value("is_current").Equal(true)

	second := data.Last().Object()
	second.Value("session_id").Equal("sess-2")
	second.Value("is_current").Equal(false)
}

// ── RevokeSession ──────────────────────────────────────────────────────────

func TestSessionManager_RevokeSession(t *testing.T) {
	sm, store, kv, db := newSessionManager(t)

	require.NoError(t, store.Create(context.Background(), &model.UserSession{
		SessionID: "sid-to-revoke", UserUUID: "uuid-4", Username: "dave", Role: "user",
		IP: "10.0.0.1", UserAgent: "test", LastActive: time.Now().Unix(),
		CreatedAt: time.Now().Add(-time.Hour).Unix(), ExpiresAt: time.Now().Add(23 * time.Hour).Unix(),
	}))
	var row model.UserSession
	require.NoError(t, db.Where("session_id = ?", "sid-to-revoke").First(&row).Error)

	other := &model.UserSession{
		SessionID: "sid-current", UserUUID: "uuid-4", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("session", other)
		ctx.Values().Set("user_uuid", "uuid-4")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Post("/sessions/{id}/revoke", sm.RevokeSession)

	e := httptest.New(t, app)
	e.POST(fmt.Sprintf("/sessions/%d/revoke", row.ID)).Expect().Status(http.StatusOK)

	// Row and cache entry are both gone — the credential stops resolving.
	var count int64
	db.Model(&model.UserSession{}).Where("id = ?", row.ID).Count(&count)
	assert.Equal(t, int64(0), count)
	assert.NotContains(t, kv.data, sessionCachePrefix+"sid-to-revoke")
	_, err := store.Lookup(context.Background(), "sid-to-revoke")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionManager_RevokeSession_NotFound(t *testing.T) {
	sm, _, _, _ := newSessionManager(t)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_uuid", "uuid-5")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Post("/sessions/{id}/revoke", sm.RevokeSession)

	e := httptest.New(t, app)
	e.POST("/sessions/999/revoke").Expect().Status(http.StatusNotFound)
}

func TestSessionManager_RevokeSession_CannotRevokeCurrent(t *testing.T) {
	sm, store, _, db := newSessionManager(t)

	require.NoError(t, store.Create(context.Background(), &model.UserSession{
		SessionID: "sid-me", UserUUID: "uuid-6", Username: "frank", Role: "user",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}))
	var row model.UserSession
	require.NoError(t, db.Where("session_id = ?", "sid-me").First(&row).Error)
	current, err := store.Lookup(context.Background(), "sid-me")
	require.NoError(t, err)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("session", current)
		ctx.Values().Set("user_uuid", "uuid-6")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Post("/sessions/{id}/revoke", sm.RevokeSession)

	e := httptest.New(t, app)
	e.POST(fmt.Sprintf("/sessions/%d/revoke", row.ID)).Expect().Status(http.StatusBadRequest)
}

// ── LogoutAll ──────────────────────────────────────────────────────────────

func TestSessionManager_LogoutAll(t *testing.T) {
	sm, store, kv, db := newSessionManager(t)
	now := time.Now()
	later := now.Add(24 * time.Hour).Unix()
	for _, sid := range []string{"all-a", "all-b", "all-c"} {
		require.NoError(t, store.Create(context.Background(), &model.UserSession{
			SessionID: sid, UserUUID: "uuid-7", Username: "grace", Role: "user", ExpiresAt: later,
		}))
	}
	require.NoError(t, store.Create(context.Background(), &model.UserSession{
		SessionID: "other-user", UserUUID: "uuid-8", Username: "other", Role: "user", ExpiresAt: later,
	}))

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("session", &model.UserSession{SessionID: "all-a", UserUUID: "uuid-7", ExpiresAt: later})
		ctx.Values().Set("user_uuid", "uuid-7")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Post("/sessions/logout-all", sm.LogoutAll)

	e := httptest.New(t, app)
	e.POST("/sessions/logout-all").Expect().Status(http.StatusOK)

	var count int64
	db.Model(&model.UserSession{}).Where("user_uuid = ?", "uuid-7").Count(&count)
	assert.Equal(t, int64(0), count, "all sessions for the user gone")
	db.Model(&model.UserSession{}).Where("user_uuid = ?", "uuid-8").Count(&count)
	assert.Equal(t, int64(1), count, "other users unaffected")

	for key := range kv.data {
		assert.False(t, len(key) > len(sessionCachePrefix) && key[:len(sessionCachePrefix)] == sessionCachePrefix &&
			(key[len(sessionCachePrefix):] == "all-a" || key[len(sessionCachePrefix):] == "all-b" || key[len(sessionCachePrefix):] == "all-c"),
			"revoked sessions must not linger in the cache")
	}
}

// ── LogoutHandler ──────────────────────────────────────────────────────────

func TestSessionManager_LogoutHandler(t *testing.T) {
	sm, store, kv, db := newSessionManager(t)

	require.NoError(t, store.Create(context.Background(), &model.UserSession{
		SessionID: "sid-logout", UserUUID: "uuid-8", Username: "heidi", Role: "user",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}))
	sess, err := store.Lookup(context.Background(), "sid-logout")
	require.NoError(t, err)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("session", sess)
		ctx.Values().Set("user_uuid", "uuid-8")
		ctx.Values().Set("user_role", "user")
		ctx.Next()
	})
	app.Post("/auth/logout", sm.LogoutHandler)

	e := httptest.New(t, app)
	resp := e.POST("/auth/logout").Expect().Status(http.StatusOK)

	var count int64
	db.Model(&model.UserSession{}).Where("session_id = ?", "sid-logout").Count(&count)
	assert.Equal(t, int64(0), count)
	assert.NotContains(t, kv.data, sessionCachePrefix+"sid-logout")

	cleared := false
	for _, c := range resp.Raw().Cookies() {
		if c.Name == pkg.SessionCookie && c.Value == "" && c.MaxAge < 0 {
			cleared = true
		}
	}
	assert.True(t, cleared, "master_session cookie must be cleared on logout")
}

func TestSessionManager_LogoutHandler_NoSession(t *testing.T) {
	sm, _, _, _ := newSessionManager(t)

	app := iris.New()
	app.Post("/auth/logout", sm.LogoutHandler)

	e := httptest.New(t, app)
	e.POST("/auth/logout").Expect().Status(http.StatusOK) // should not panic
}

// ── Auth-required guards ───────────────────────────────────────────────────

func TestSessionManager_ListSessions_NoAuth(t *testing.T) {
	sm := NewSessionManager(Deps{})

	app := iris.New()
	app.Get("/sessions", sm.ListSessions)

	e := httptest.New(t, app)
	e.GET("/sessions").Expect().Status(http.StatusUnauthorized)
}

func TestSessionManager_RevokeSession_NoAuth(t *testing.T) {
	sm := NewSessionManager(Deps{})

	app := iris.New()
	app.Post("/sessions/{id}/revoke", sm.RevokeSession)

	e := httptest.New(t, app)
	e.POST("/sessions/1/revoke").Expect().Status(http.StatusUnauthorized)
}

func TestSessionManager_LogoutAll_NoAuth(t *testing.T) {
	sm := NewSessionManager(Deps{})

	app := iris.New()
	app.Post("/sessions/logout-all", sm.LogoutAll)

	e := httptest.New(t, app)
	e.POST("/sessions/logout-all").Expect().Status(http.StatusUnauthorized)
}
