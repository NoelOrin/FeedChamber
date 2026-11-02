package middleware

import (
	"testing"
	"time"

	"feedchamber/internal/pkg/ratelimit"
	"feedchamber/internal/pkg/testnats"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

func TestRequestTimeoutPreservesJSONResponse(t *testing.T) {
	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(PathBodyLimit(1*1024*1024, nil))
	v1.Use(RequestTimeout(30 * time.Second))
	v1.Post("/auth/login", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"code": 0, "msg": "success"})
	})

	e := httptest.New(t, app)
	e.POST("/api/v1/auth/login").
		WithJSON(iris.Map{"login": "admin", "password": "admin123"}).
		Expect().
		Status(iris.StatusOK).
		ContentType("application/json").
		JSON().Object().ValueEqual("code", 0).ValueEqual("msg", "success")
}

func TestRateLimitedRouteExecutesMainHandler(t *testing.T) {
	kv := testnats.NewKVStore(t, "request_timeout_test")

	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(PathBodyLimit(1*1024*1024, nil))
	v1.Post("/auth/login", RateLimit(ratelimit.NewLimiter(kv), "login", 10, time.Minute), func(ctx iris.Context) {
		ctx.JSON(iris.Map{"code": 0, "msg": "success"})
	})

	e := httptest.New(t, app)
	e.POST("/api/v1/auth/login").
		WithJSON(iris.Map{"login": "admin", "password": "admin123"}).
		Expect().
		Status(iris.StatusOK).
		ContentType("application/json").
		JSON().Object().ValueEqual("code", 0).ValueEqual("msg", "success")
}

func TestRequestTimeoutSkipsStream(t *testing.T) {
	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(RequestTimeout(time.Nanosecond))
	v1.Get("/notifications/stream", func(ctx iris.Context) {
		ctx.WriteString("event: ping\n\n")
	})

	e := httptest.New(t, app)
	e.GET("/api/v1/notifications/stream").
		Expect().
		Status(iris.StatusOK).
		Body().Equal("event: ping\n\n")
}
