package middleware

import (
	"encoding/json"
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"

	"feedchamber/internal/pkg"
)

// Iris 默认对「路由不存在」与「方法不匹配」直接写纯文本，绕过项目统一信封。
// 前端按 { code, msg, data } 解析响应，拿到 "Not Found" 会静默失败（解析不出 code）。
//
// 因此 app 必须装 middleware.NotFoundEnvelope()：把 404/405 补成信封，
// 且不能影响业务自己主动返回的 404（那已经是信封，不能再覆盖一遍）。

type envelope struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func decodeEnvelope(t *testing.T, body string) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("响应不是合法 JSON 信封：%q（err=%v）", body, err)
	}
	return e
}

// 路由不存在 → 404 且是信封。
func TestNotFoundReturnsEnvelope(t *testing.T) {
	app := iris.New()
	app.Configure(iris.WithFireMethodNotAllowed)
	RegisterNotFoundHandlers(app)
	app.Post("/api/v1/exists", func(ctx iris.Context) { pkg.Success(ctx, nil) })

	r := httptest.New(t, app).POST("/api/v1/nope").Expect().Status(iris.StatusNotFound)
	e := decodeEnvelope(t, r.Body().Raw())
	if int(pkg.NOT_FOUND) != e.Code {
		t.Errorf("404 信封 code = %d, 期望 %d", e.Code, pkg.NOT_FOUND)
	}
	if e.Msg == "" {
		t.Error("404 信封 msg 不得为空")
	}
}

// 方法不匹配 → 405 且是信封（而非默认的 404）。
func TestMethodNotAllowedReturnsEnvelope(t *testing.T) {
	app := iris.New()
	app.Configure(iris.WithFireMethodNotAllowed)
	RegisterNotFoundHandlers(app)
	app.Post("/api/v1/only-post", func(ctx iris.Context) { pkg.Success(ctx, nil) })

	r := httptest.New(t, app).GET("/api/v1/only-post").Expect().Status(iris.StatusMethodNotAllowed)
	e := decodeEnvelope(t, r.Body().Raw())
	if int(pkg.INVALID_PARAMS) != e.Code {
		t.Errorf("405 信封 code = %d, 期望 %d", e.Code, pkg.INVALID_PARAMS)
	}
}

// 业务主动返回的 404 必须原样保留，不能被中间件再覆盖一次。
func TestDeliberateNotFoundIsNotOverwritten(t *testing.T) {
	app := iris.New()
	app.Configure(iris.WithFireMethodNotAllowed)
	RegisterNotFoundHandlers(app)
	app.Post("/api/v1/gone", func(ctx iris.Context) {
		pkg.FailStatus(ctx, iris.StatusNotFound, pkg.NOT_FOUND, "业务自己的 404")
	})

	r := httptest.New(t, app).POST("/api/v1/gone").Expect().Status(iris.StatusNotFound)
	e := decodeEnvelope(t, r.Body().Raw())
	if e.Msg != "业务自己的 404" {
		t.Errorf("业务主动的 404 被覆盖了：msg=%q", e.Msg)
	}
}

// 正常请求不受影响（响应体仍是业务写的，且未被改写）。
func TestNormalResponseUntouched(t *testing.T) {
	app := iris.New()
	app.Configure(iris.WithFireMethodNotAllowed)
	RegisterNotFoundHandlers(app)
	app.Post("/api/v1/ok", func(ctx iris.Context) { pkg.Success(ctx, iris.Map{"a": 1}) })

	r := httptest.New(t, app).POST("/api/v1/ok").Expect().Status(iris.StatusOK)
	e := decodeEnvelope(t, r.Body().Raw())
	if e.Code != 0 {
		t.Errorf("正常响应 code = %d, 期望 0", e.Code)
	}
}
