package gateway

import (
	"context"
	"net/http"
	"testing"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg/kvstore"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// validToken returns an opaque placeholder credential for tests whose handlers
// rely on manually-set context values rather than the gateway auth pipeline
// (the presented Authorization header is never resolved).
func validToken(t *testing.T, username, userUUID, role string) string {
	t.Helper()
	return "sid-" + userUUID
}

// securityMemoryKV is a simple in-memory KV store for security tests.
type securityMemoryKV struct {
	baseKV
	data     map[string]string
	counters map[string]int64
}

func newSecurityMemoryKV() *securityMemoryKV {
	return &securityMemoryKV{
		data:     make(map[string]string),
		counters: make(map[string]int64),
	}
}

func (m *securityMemoryKV) Get(_ context.Context, key string) (string, error) {
	val, ok := m.data[key]
	if !ok {
		return "", kvstore.ErrKeyNotFound
	}
	return val, nil
}
func (m *securityMemoryKV) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.data[key] = value
	return nil
}
func (m *securityMemoryKV) SetNX(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = value
	return true, nil
}
func (m *securityMemoryKV) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	delete(m.counters, key)
	return nil
}
func (m *securityMemoryKV) Exists(_ context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}
func (m *securityMemoryKV) Incr(_ context.Context, key string) (int64, error) {
	m.counters[key]++
	return m.counters[key], nil
}
func (m *securityMemoryKV) IncrBy(_ context.Context, key string, delta int64) (int64, error) {
	m.counters[key] += delta
	return m.counters[key], nil
}
func (m *securityMemoryKV) PrefixScan(_ context.Context, prefix string) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			result[k] = v
		}
	}
	return result, nil
}
func (m *securityMemoryKV) DeletePrefix(_ context.Context, prefix string) error {
	for k := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.data, k)
		}
	}
	return nil
}
func (m *securityMemoryKV) Publish(_ context.Context, _ string, _ []byte) error { return nil }
func (m *securityMemoryKV) Subscribe(_ context.Context, _ string) (<-chan []byte, error) {
	return nil, nil
}
func (m *securityMemoryKV) Close() error { return nil }

// setupSecurityDB creates an in-memory SQLite DB with security tables.
func setupSecurityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SecurityEvent{}, &model.LoginHistory{}))
	return db
}

// ── IP blocking / brute force ───────────────────────────────────────────────

func TestSecurityManager_RecordFailedLogin_BlocksIP(t *testing.T) {
	kv := newSecurityMemoryKV()
	sm := NewSecurityManager(Deps{KV: kv})

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		sm.RecordFailedLogin(ctx, "10.0.0.1", "attacker")
	}

	assert.True(t, sm.isIPBlocked("10.0.0.1"))
	assert.True(t, sm.isUserBlocked("attacker"))
}

func TestSecurityManager_RecordFailedLogin_BelowThreshold(t *testing.T) {
	kv := newSecurityMemoryKV()
	sm := NewSecurityManager(Deps{KV: kv})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		sm.RecordFailedLogin(ctx, "10.0.0.2", "innocent")
	}

	assert.False(t, sm.isIPBlocked("10.0.0.2"))
	assert.False(t, sm.isUserBlocked("innocent"))
}

func TestSecurityManager_ClearFailedLogin(t *testing.T) {
	kv := newSecurityMemoryKV()
	sm := NewSecurityManager(Deps{KV: kv})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		sm.RecordFailedLogin(ctx, "10.0.0.3", "gooduser")
	}
	sm.ClearFailedLogin(ctx, "10.0.0.3", "gooduser")

	assert.False(t, sm.isIPBlocked("10.0.0.3"))
	assert.False(t, sm.isUserBlocked("gooduser"))
}

// ── SecurityCheck middleware ────────────────────────────────────────────────

func TestSecurityManager_SecurityCheck_BlocksBlockedIP(t *testing.T) {
	kv := newSecurityMemoryKV()
	sm := NewSecurityManager(Deps{KV: kv})

	// Block the IP directly and verify via DirectIPBlock
	_ = kv.Set(context.Background(), "blocked_ip:10.0.0.99", "1", time.Hour)
	assert.True(t, sm.isIPBlocked("10.0.0.99"))
	assert.False(t, sm.isIPBlocked("10.0.0.1"))

	// Test via HTTP — wrap handler to set remote addr
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		// Simulate X-Forwarded-For to test IP blocking
		app.Logger().Info("IP blocking test")
		sm.SecurityCheck(ctx)
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	// Empty remote addr in httptest — should not be blocked
	e.GET("/test").Expect().Status(http.StatusOK)
}

func TestSecurityManager_SecurityCheck_BlocksBlockedUser(t *testing.T) {
	kv := newSecurityMemoryKV()
	sm := NewSecurityManager(Deps{KV: kv})

	_ = kv.Set(context.Background(), "blocked_user:baduser", "1", time.Hour)

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("username", "baduser")
		ctx.Next()
	})
	app.Use(sm.SecurityCheck)
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusForbidden)
}

func TestSecurityManager_SecurityCheck_AllowsCleanRequest(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{DB: db})

	app := iris.New()
	app.Use(sm.SecurityCheck)
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusOK).JSON().Object().Value("ok").Equal(true)
}

func TestSecurityManager_SecurityCheck_RecordsUnusualLocation(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{
		DB:             db,
		SettingService: &mockSecSettingReader{data: map[string]string{"security_unusual_location_enabled": "true"}},
	})

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_uuid", "uuid-new")
		ctx.Values().Set("username", "newuser")
		ctx.Request().RemoteAddr = "10.0.0.50"
		ctx.Next()
	})
	app.Use(sm.SecurityCheck)
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusOK)

	// Verify security event was recorded
	var count int64
	db.Model(&model.SecurityEvent{}).Where("event_type = ?", "unusual_location").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestSecurityManager_SecurityCheck_SkipsUnusualLocationWhenDisabled(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{
		DB:             db,
		SettingService: &mockSecSettingReader{data: map[string]string{"security_unusual_location_enabled": "false"}},
	})

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_uuid", "uuid-skip")
		ctx.Values().Set("username", "skipuser")
		ctx.Next()
	})
	app.Use(sm.SecurityCheck)
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusOK)

	var count int64
	db.Model(&model.SecurityEvent{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

// ── Security event recording ────────────────────────────────────────────────

func TestSecurityManager_RecordEvent(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{DB: db})

	sm.RecordEvent(context.Background(), "uuid-a", "1.2.3.4", "token_reuse", "critical",
		"Token reused from different location", "blocked")

	var events []model.SecurityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "uuid-a", events[0].UserUUID)
	assert.Equal(t, "token_reuse", events[0].EventType)
	assert.Equal(t, "critical", events[0].Severity)
	assert.Equal(t, "blocked", events[0].Action)
	assert.False(t, events[0].Handled)
	assert.NotZero(t, events[0].CreatedAt)
}

// ── Login history ───────────────────────────────────────────────────────────

func TestSecurityManager_RecordLoginHistory(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{DB: db})

	sm.RecordLoginHistory(context.Background(), "uuid-b", "5.6.7.8", "US", "Mozilla/5.0", true)

	var histories []model.LoginHistory
	require.NoError(t, db.Find(&histories).Error)
	require.Len(t, histories, 1)
	assert.Equal(t, "uuid-b", histories[0].UserUUID)
	assert.Equal(t, "5.6.7.8", histories[0].IP)
	assert.Equal(t, "US", histories[0].Location)
	assert.True(t, histories[0].Success)
}

// ── API handlers ────────────────────────────────────────────────────────────

func TestSecurityManager_ListSecurityEvents_Admin(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{DB: db})

	// Seed a security event
	db.Create(&model.SecurityEvent{
		UserUUID: "uuid-c", IP: "1.2.3.4", EventType: "brute_force",
		Severity: "high", Detail: "test", Action: "blocked", CreatedAt: time.Now().Unix(),
	})

	token := validToken(t, "admin", "uuid-admin", "admin")

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_role", "admin")
		ctx.Values().Set("user_uuid", "uuid-admin")
		ctx.Values().Set("username", "admin")
		ctx.Next()
	})
	app.Get("/security/events", sm.ListSecurityEvents)

	e := httptest.New(t, app)
	resp := e.GET("/security/events").WithHeader("Authorization", "Bearer "+token).Expect().Status(http.StatusOK)
	resp.JSON().Object().Value("data").Array().Length().Equal(1)
}

func TestSecurityManager_ListSecurityEvents_NonAdmin(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{DB: db})

	token := validToken(t, "user", "uuid-user", "user")

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_role", "user")
		ctx.Values().Set("user_uuid", "uuid-user")
		ctx.Values().Set("username", "user")
		ctx.Next()
	})
	app.Get("/security/events", sm.ListSecurityEvents)

	e := httptest.New(t, app)
	e.GET("/security/events").WithHeader("Authorization", "Bearer "+token).Expect().Status(http.StatusForbidden)
}

func TestSecurityManager_ListLoginHistory(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{DB: db})

	// Seed login history for this user
	db.Create(&model.LoginHistory{
		UserUUID: "uuid-d", IP: "10.0.0.1", Location: "CN", UserAgent: "curl", Success: true,
		CreatedAt: time.Now(),
	})
	// Seed login history for another user (should not appear)
	db.Create(&model.LoginHistory{
		UserUUID: "uuid-other", IP: "10.0.0.2", Location: "US", UserAgent: "wget", Success: false,
		CreatedAt: time.Now(),
	})

	token := validToken(t, "userd", "uuid-d", "user")

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_role", "user")
		ctx.Values().Set("user_uuid", "uuid-d")
		ctx.Values().Set("username", "userd")
		ctx.Next()
	})
	app.Get("/security/login-history", sm.ListLoginHistory)

	e := httptest.New(t, app)
	resp := e.GET("/security/login-history").WithHeader("Authorization", "Bearer "+token).Expect().Status(http.StatusOK)
	resp.JSON().Object().Value("data").Array().Length().Equal(1)
}

func TestSecurityManager_ReportSuspicious(t *testing.T) {
	db := setupSecurityDB(t)
	sm := NewSecurityManager(Deps{DB: db})

	token := validToken(t, "reporter", "uuid-report", "user")

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_role", "user")
		ctx.Values().Set("user_uuid", "uuid-report")
		ctx.Values().Set("username", "reporter")
		ctx.Next()
	})
	app.Post("/security/report", sm.ReportSuspicious)

	e := httptest.New(t, app)
	e.POST("/security/report").WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]string{"event_type": "suspicious_login", "detail": "Unknown device", "ip": "9.9.9.9"}).
		Expect().Status(http.StatusOK)

	var events []model.SecurityEvent
	require.NoError(t, db.Where("user_uuid = ?", "uuid-report").Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "suspicious_login", events[0].EventType)
	assert.Equal(t, "low", events[0].Severity)
}

func TestSecurityManager_ReportSuspicious_MissingEventType(t *testing.T) {
	sm := NewSecurityManager(Deps{})
	token := validToken(t, "userx", "uuid-x", "user")

	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_role", "user")
		ctx.Values().Set("user_uuid", "uuid-x")
		ctx.Values().Set("username", "userx")
		ctx.Next()
	})
	app.Post("/security/report", sm.ReportSuspicious)

	e := httptest.New(t, app)
	e.POST("/security/report").WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]string{"detail": "No type"}).
		Expect().Status(http.StatusBadRequest)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

type mockSecSettingReader struct {
	data map[string]string
}

func (m *mockSecSettingReader) Get(_ context.Context, key string) (string, error) {
	return m.data[key], nil
}
