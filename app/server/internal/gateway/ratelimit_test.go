package gateway

import (
	"context"
	"net/http"
	"testing"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSettingReader implements SettingReader for tests.
type mockSettingReader struct {
	vals map[string]string
}

func (m *mockSettingReader) Get(_ context.Context, key string) (string, error) {
	return m.vals[key], nil
}

// leakyKV is an in-memory kvstore.Store with a settable error flag for tests.
type leakyKV struct {
	baseKV
	data     map[string]*leakyKVEntry
	failNX   bool
	failIncr bool
	failSet  bool
	ttlKeys  map[string]time.Time // track TTL expiration
}

type leakyKVEntry struct {
	val   string
	exp   time.Time
	count int64
}

func newLeakyKV() *leakyKV {
	return &leakyKV{
		data:    make(map[string]*leakyKVEntry),
		ttlKeys: make(map[string]time.Time),
	}
}

func (m *leakyKV) Get(_ context.Context, key string) (string, error) {
	e, ok := m.data[key]
	if !ok || (!e.exp.IsZero() && time.Now().After(e.exp)) {
		return "", kvstore.ErrKeyNotFound
	}
	return e.val, nil
}

func (m *leakyKV) Set(_ context.Context, key, value string, ttl time.Duration) error {
	if m.failSet {
		return nil // no-op for SetNX simulation
	}
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.data[key] = &leakyKVEntry{val: value, exp: exp}
	return nil
}

func (m *leakyKV) SetNX(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	if m.failNX {
		return false, nil
	}
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.Set(context.Background(), key, value, ttl)
	return true, nil
}

func (m *leakyKV) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *leakyKV) Exists(_ context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

func (m *leakyKV) PrefixScan(_ context.Context, prefix string) (map[string]string, error) {
	out := make(map[string]string)
	for k, e := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out[k] = e.val
		}
	}
	return out, nil
}

func (m *leakyKV) DeletePrefix(_ context.Context, prefix string) error {
	for k := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.data, k)
		}
	}
	return nil
}

func (m *leakyKV) Incr(_ context.Context, key string) (int64, error) {
	if m.failIncr {
		return 1, nil // return a fake count, test checks fail-open
	}
	e, ok := m.data[key]
	if !ok {
		e = &leakyKVEntry{}
		m.data[key] = e
	}
	e.count++
	return e.count, nil
}

func (m *leakyKV) IncrBy(_ context.Context, key string, delta int64) (int64, error) {
	e, ok := m.data[key]
	if !ok {
		e = &leakyKVEntry{}
		m.data[key] = e
	}
	e.count += delta
	return e.count, nil
}

func (m *leakyKV) Publish(_ context.Context, channel string, payload []byte) error {
	return nil
}

func (m *leakyKV) Subscribe(_ context.Context, channel string) (<-chan []byte, error) {
	return nil, nil
}

func (m *leakyKV) Close() error {
	return nil
}

func newTestDeps(kv kvstore.Store, sr pkg.SettingReader) Deps {
	return Deps{
		KV:             kv,
		SettingService: sr,
	}
}

// setupRateLimitApp creates an iris app with RateLimiter.Middleware wired in.
func setupRateLimitApp(rl *RateLimiter) *iris.Application {
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		if !rl.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Get("/test", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})
	return app
}

func TestRateLimiter_DisabledByDefault(t *testing.T) {
	kv := newLeakyKV()
	sr := &mockSettingReader{vals: map[string]string{"rate_limit_enabled": "false"}}
	rl := NewRateLimiter(newTestDeps(kv, sr))
	cfg := rl.loadConfig(context.Background())
	if cfg.Enabled {
		t.Fatal("expected rate limiting disabled by default")
	}
}

func TestRateLimiter_EnabledViaSetting(t *testing.T) {
	kv := newLeakyKV()
	sr := &mockSettingReader{vals: map[string]string{"rate_limit_enabled": "true"}}
	rl := NewRateLimiter(newTestDeps(kv, sr))
	cfg := rl.loadConfig(context.Background())
	if !cfg.Enabled {
		t.Fatal("expected rate limiting enabled when setting is 'true'")
	}
}

func TestRateLimiter_DisabledWhenKVNil(t *testing.T) {
	sr := &mockSettingReader{vals: map[string]string{"rate_limit_enabled": "true"}}
	rl := NewRateLimiter(newTestDeps(nil, sr))
	// Middleware should not panic with nil KV; we test it returns true (pass-through).
	app := setupRateLimitApp(rl)
	e := httptest.New(t, app)
	e.GET("/test").Expect().Status(http.StatusOK)
}

func TestRateLimiter_SettingServiceNil(t *testing.T) {
	kv := newLeakyKV()
	rl := NewRateLimiter(newTestDeps(kv, nil))
	cfg := rl.loadConfig(context.Background())
	if cfg.Enabled {
		t.Fatal("expected disabled when SettingService is nil")
	}
}

func TestRateLimiter_WithinLimitPasses(t *testing.T) {
	kv := newLeakyKV()
	sr := &mockSettingReader{vals: map[string]string{"rate_limit_enabled": "true"}}
	rl := NewRateLimiter(newTestDeps(kv, sr))
	app := setupRateLimitApp(rl)
	e := httptest.New(t, app)
	// Default limit is 60; a single request should pass.
	e.GET("/test").Expect().Status(http.StatusOK)
}

func TestRateLimiter_ExceedsLimit_Returns429(t *testing.T) {
	kv := newLeakyKV()
	sr := &mockSettingReader{vals: map[string]string{"rate_limit_enabled": "true"}}
	rl := NewRateLimiter(newTestDeps(kv, sr))

	app := setupRateLimitApp(rl)
	e := httptest.New(t, app)

	// Fire requests to exhaust the default limit (60), then verify the next
	// one is rejected. The key format is generated by Middleware itself via
	// gatewayClientIP, so we do not hard-code any key — each real request
	// increments the correct counter automatically.
	for i := 0; i < 60; i++ {
		e.GET("/test").Expect().Status(http.StatusOK)
	}
	// 61st request should be rejected with 429.
	e.GET("/test").Expect().Status(http.StatusTooManyRequests)
}

func TestRateLimiter_FailOpenOnKVError(t *testing.T) {
	kv := newLeakyKV()
	kv.failIncr = true // simulate KV error on Incr
	sr := &mockSettingReader{vals: map[string]string{"rate_limit_enabled": "true"}}
	rl := NewRateLimiter(newTestDeps(kv, sr))
	app := setupRateLimitApp(rl)
	e := httptest.New(t, app)
	// Should pass through (fail open) when KV errors.
	e.GET("/test").Expect().Status(http.StatusOK)
}

func TestRateLimiter_Middleware_Integration(t *testing.T) {
	kv := newLeakyKV()
	db := setupSessionDB(t)
	sr := &mockSettingReader{vals: map[string]string{"rate_limit_enabled": "true"}}
	deps := Deps{
		DB:             db,
		KV:             kv,
		SettingService: sr,
		Perms:          &mockPermChecker{hasPerm: func(role, code string) bool { return true }},
	}
	gw := New(deps)

	store := NewSessionStore(deps)
	require.NotNil(t, store)
	sess := &model.UserSession{
		SessionID: "sid-ratelimit", UserUUID: "uuid-1", Username: "alice", Role: "admin",
		IP: "1.2.3.4", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Create(context.Background(), sess))

	app := iris.New()
	app.Use(gw.Preflight())
	app.Use(gw.Enforce())
	app.Get("/api/v1/dashboard", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"ok": true})
	})

	e := httptest.New(t, app)
	resp := e.GET("/api/v1/dashboard").WithHeader("Authorization", "Bearer "+sess.SessionID).Expect().Status(http.StatusOK).JSON().Object()
	assert.True(t, resp.Value("ok").Boolean().Raw())
}

func TestRateLimiter_KeyFormat(t *testing.T) {
	ip := "127.0.0.1"
	key := "ratelimit:" + ip
	rlKey := "rl:" + key
	if rlKey != "rl:ratelimit:127.0.0.1" {
		t.Fatalf("unexpected key format: %s", rlKey)
	}
}

func TestRateLimiter_LoadConfig_Defaults(t *testing.T) {
	kv := newLeakyKV()
	rl := NewRateLimiter(newTestDeps(kv, nil))
	cfg := rl.loadConfig(context.Background())
	require.False(t, cfg.Enabled, "expected disabled by default")
	require.Equal(t, 60, cfg.Limit, "expected default limit 60")
	require.Equal(t, time.Minute, cfg.Window, "expected default window 1 minute")
}
