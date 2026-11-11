package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	iristest "github.com/kataras/iris/v12/httptest"

	"feedchamber/internal/pkg"
)

// 本文件钉住 PathBodyLimit 的三条性质，它们此前都没有测试：
//
//  1. **超限必须返回 413 信封**。Iris 的 MaxBytesReader 让 handler 侧 ReadJSON 报
//     "http: request body too large"，默认会被写成 400 —— 前端会当成参数错误，
//     而实际是请求体过大。
//  2. **正常请求不被破坏**。
//  3. **无请求体的请求不得被替换 ResponseWriter**。旧实现给每个请求都装
//     bodyLimitWriter 把响应攒进 bytes.Buffer（为事后改写 400→413），代价是
//     /upload/file/{key} 的 10MB 下载要在内存里整份攒完才发给客户端。

// buildBodyLimitApp 返回 app 与**带 bodyLimit 的 v1 party**。
//
// 路由必须注册在这个 party 上，不能注册到 app 根：app.Post 不继承 party 的
// 中间件链，测试会变成「什么都没限制」的假绿（最初就是这样写错的）。
func buildBodyLimitApp(limit int64) (*iris.Application, iris.Party) {
	app := iris.New()
	v1 := app.Party("/api/v1")
	v1.Use(PathBodyLimit(limit, nil))
	return app, v1
}

// 超限 → 413 + BODY_TOO_LARGE 信封。
func TestPathBodyLimitRejectsOversizedBodyWith413(t *testing.T) {
	app, v1 := buildBodyLimitApp(64)
	v1.Post("/echo", func(ctx iris.Context) {
		var p map[string]any
		if err := ctx.ReadJSON(&p); err != nil {
			pkg.Fail(ctx, pkg.INVALID_PARAMS, "invalid request body")
			return
		}
		pkg.Success(ctx, p)
	})

	big := `{"x":"` + strings.Repeat("a", 500) + `"}`
	r := iristest.New(t, app).POST("/api/v1/echo").
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(big)).Expect().Status(iris.StatusRequestEntityTooLarge)

	e := decodeEnvelope(t, r.Body().Raw())
	if int(pkg.BODY_TOO_LARGE) != e.Code {
		t.Errorf("超限信封 code = %d, 期望 %d", e.Code, pkg.BODY_TOO_LARGE)
	}
}

// 未超限 → 200，响应体原样透出。
func TestPathBodyLimitPassesNormalBody(t *testing.T) {
	app, v1 := buildBodyLimitApp(1 << 20)
	v1.Post("/echo", func(ctx iris.Context) {
		var p struct {
			X string `json:"x"`
		}
		if err := ctx.ReadJSON(&p); err != nil {
			pkg.Fail(ctx, pkg.INVALID_PARAMS, "invalid request body")
			return
		}
		pkg.Success(ctx, p.X)
	})

	r := iristest.New(t, app).POST("/api/v1/echo").
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"x":"ok"}`)).Expect().Status(iris.StatusOK)

	e := decodeEnvelope(t, r.Body().Raw())
	if e.Code != 0 || e.Data != "ok" {
		t.Errorf("正常响应被破坏：code=%d data=%v", e.Code, e.Data)
	}
}

// GET（无请求体）必须走快路径：不包装请求体、不碰 ResponseWriter。
//
// 判据用「handler 内能否观察到请求体被替换成 MaxBytesReader 的包装」——
// 旧实现无条件包装，新实现在 GET/HEAD/OPTIONS 上直接放行。
func TestPathBodyLimitSkipsBodylessMethods(t *testing.T) {
	var bodyType string

	app, v1 := buildBodyLimitApp(1 << 20)
	v1.Get("/file", func(ctx iris.Context) {
		bodyType = reflect.TypeOf(ctx.Request().Body).String()
		_, _ = ctx.WriteString("payload")
	})

	iristest.New(t, app).GET("/api/v1/file").Expect().
		Status(iris.StatusOK).Body().IsEqual("payload")

	// http.NoBody 或 *http.body —— 只要不是 limitExceededReader 就说明没包装。
	if strings.Contains(bodyType, "limitExceededReader") {
		t.Errorf("GET 请求的 body 被包成了 limitExceededReader（%s）："+
			"无请求体方法不需要限制，也不该干扰字节流出口", bodyType)
	}
}

// 大响应必须边写边发。
//
// 关键判据不是「最终字节数对不对」（缓冲后也能对），而是**首字节到达的时刻**：
// handler 在第一块写完后阻塞在 channel 上，客户端必须能在 handler 仍然阻塞时
// 收到第一块数据。旧实现把所有 Write 收进 bytes.Buffer，handler 返回后才写出，
// 因此首块数据只能在 close(release) 之后到达 —— 这个断言会失败。
func TestPathBodyLimitStreamsLargeResponse(t *testing.T) {
	release := make(chan struct{})
	firstChunkSent := make(chan struct{})

	app, v1 := buildBodyLimitApp(1 << 20)
	v1.Get("/big", func(ctx iris.Context) {
		_, _ = ctx.Write(bytes.Repeat([]byte("x"), 1024))
		ctx.ResponseWriter().Flush()
		close(firstChunkSent)
		// 阻塞等待：只有真正流式写出，客户端此刻才能收到数据
		<-release
		_, _ = ctx.Write(bytes.Repeat([]byte("y"), 1024))
	})

	// Iris 必须先 Build 才能交给 net/http：未 Build 的 app.ServeHTTP 会 nil panic。
	if err := app.Build(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/big")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// 等 handler 发出第一块并进入阻塞
	<-firstChunkSent

	gotFirst := make(chan int)
	go func() {
		buf := make([]byte, 1024)
		n, _ := io.ReadFull(resp.Body, buf)
		gotFirst <- n
	}()

	select {
	case n := <-gotFirst:
		if n == 0 {
			t.Fatal("首块读取返回 0 字节")
		}
		// 真流式：handler 仍阻塞在 <-release，客户端已拿到首块
		close(release)
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("首块数据在 handler 返回前没有到达客户端：响应被整块缓冲了" +
			"（/upload/file/{key} 的 10MB 下载会因此在内存里整份攒完才发出）")
	}

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}
}
