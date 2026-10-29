package middleware

import (
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

func TestRecoverWritesEnvelopeForPanicInHandler(t *testing.T) {
	app := iris.New()
	app.Use(Recover())
	app.Post("/boom", func(ctx iris.Context) {
		panic("handler exploded")
	})

	e := httptest.New(t, app)
	e.POST("/boom").Expect().
		Status(iris.StatusInternalServerError).
		ContentType("application/json").
		JSON().Object().
		ValueEqual("code", 5001).
		ValueEqual("msg", "服务器内部错误").
		ValueEqual("data", nil)
}

// The gateway pipeline runs as per-party middleware, so a panic there has to be
// caught by the app-level Recover too — otherwise a bad credential would drop
// the connection instead of answering 500.
func TestRecoverCatchesPanicInPartyMiddleware(t *testing.T) {
	app := iris.New()
	app.Use(Logger())
	app.Use(Recover())

	protected := app.Party("/api/v1")
	protected.Use(func(ctx iris.Context) {
		panic("gateway middleware exploded")
	})
	protected.Post("/posts/list", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"code": 0})
	})

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/list").Expect().
		Status(iris.StatusInternalServerError).
		JSON().Object().ValueEqual("code", 5001)
}

func TestRecoverPassesThroughNormalRequests(t *testing.T) {
	app := iris.New()
	app.Use(Recover())
	app.Post("/ok", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"code": 0, "msg": "success"})
	})

	e := httptest.New(t, app)
	e.POST("/ok").Expect().
		Status(iris.StatusOK).
		JSON().Object().ValueEqual("code", 0).ValueEqual("msg", "success")
}
