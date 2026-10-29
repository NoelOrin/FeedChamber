package gateway

import (
	"context"
	"net/http"
	"testing"
	"time"

	"orinforum/internal/model"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The v1-level Preflight also covers the protected party, so a protected request
// historically ran the global limiter twice under one key. These tests pin the
// stage markers: one request must consume exactly one unit of the counter, no
// matter whether the caller mounts Preflight+Enforce or Preflight+Handler.

func newPipelineFixture(t *testing.T) (*Gateway, *leakyKV, *model.UserSession) {
	t.Helper()
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
		SessionID: "sid-pipeline", UserUUID: "uuid-1", Username: "root", Role: "admin",
		IP: "1.2.3.4", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Create(context.Background(), sess))
	return gw, kv, sess
}

func limiterCount(kv *leakyKV) int64 {
	// httptest leaves RemoteAddr empty, so the key is the empty-IP variant.
	e, ok := kv.data["rl:ratelimit:"]
	if !ok {
		return 0
	}
	return e.count
}

func TestPipeline_PreflightThenEnforceCountsLimiterOnce(t *testing.T) {
	gw, kv, sess := newPipelineFixture(t)

	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(gw.Preflight())
	protected := v1.Party("")
	protected.Use(gw.Enforce())
	// 必须在网关 path→permission 映射表内，否则会被"未映射即拒绝"的兜底挡掉。
	protected.Post("/dashboard", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	for i := 0; i < 5; i++ {
		e.POST("/api/v1/dashboard").WithHeader("Authorization", "Bearer "+sess.SessionID).
			Expect().Status(http.StatusOK)
	}
	assert.Equal(t, int64(5), limiterCount(kv), "5 个请求应各计数一次，而不是两次")
}

func TestPipeline_EnforceStillRejectsAnonymous(t *testing.T) {
	// 幂等不得放过匿名请求：Preflight 只做可选解析，未解析出身份时 Enforce 仍要拒。
	gw, _, _ := newPipelineFixture(t)

	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(gw.Preflight())
	protected := v1.Party("")
	protected.Use(gw.Enforce())
	// 必须在网关 path→permission 映射表内，否则会被"未映射即拒绝"的兜底挡掉。
	protected.Post("/dashboard", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	e.POST("/api/v1/dashboard").Expect().Status(http.StatusUnauthorized).
		JSON().Object().ValueEqual("code", 1001).ValueEqual("msg", "令牌不存在")
}

func TestPipeline_EnforceStillRejectsDeadCredential(t *testing.T) {
	gw, _, _ := newPipelineFixture(t)

	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(gw.Preflight())
	protected := v1.Party("")
	protected.Use(gw.Enforce())
	// 必须在网关 path→permission 映射表内，否则会被"未映射即拒绝"的兜底挡掉。
	protected.Post("/dashboard", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	e.POST("/api/v1/dashboard").WithHeader("Authorization", "Bearer not-a-session").
		Expect().Status(http.StatusUnauthorized).
		JSON().Object().ValueEqual("code", 1002)
}

func TestPipeline_APIKeyNeverResolvedByPreflight(t *testing.T) {
	// Preflight 不认 API Key。带 ak_ 前缀的请求必须落到 Enforce 的 API Key 分支：
	// 该环境未启用 API Key（设置项缺失）→ 403/1013。若它被当成匿名会话请求，
	// 得到的会是 401/1001，这两种拒绝码正好区分开两条路径。
	gw, _, _ := newPipelineFixture(t)

	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(gw.Preflight())
	protected := v1.Party("")
	protected.Use(gw.Enforce())
	// 必须在网关 path→permission 映射表内，否则会被"未映射即拒绝"的兜底挡掉。
	protected.Post("/dashboard", func(ctx iris.Context) { ctx.JSON(iris.Map{"ok": true}) })

	e := httptest.New(t, app)
	e.POST("/api/v1/dashboard").WithHeader("Authorization", "Bearer ak_missing-key").
		Expect().Status(http.StatusForbidden).
		JSON().Object().ValueEqual("code", 1013)
}
