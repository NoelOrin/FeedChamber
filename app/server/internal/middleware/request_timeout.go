package middleware

import (
	"context"
	"errors"
	"strings"
	"time"

	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
)

// RequestTimeout 返回一个中间件，为每个请求设置超时。
// SSE 等长连接端点（以 /stream 结尾）自动跳过超时。
//
// Iris Context 和 ResponseWriter 不是跨 goroutine 安全的。这里仅给请求
// context 设置 deadline，让下游 DB/HTTP 等可感知取消；硬超时由 http.Server
// 的 ReadTimeout/WriteTimeout 负责，避免异步执行 handler 链吞掉响应或 panic。
func RequestTimeout(timeout time.Duration) iris.Handler {
	return func(ctx iris.Context) {
		// SSE 等长连接端点跳过超时
		if strings.HasSuffix(ctx.Path(), "/stream") {
			ctx.Next()
			return
		}

		if timeout <= 0 {
			ctx.Next()
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx.Request().Context(), timeout)
		defer cancel()

		ctx.ResetRequest(ctx.Request().WithContext(reqCtx))
		ctx.Next()

		if errors.Is(reqCtx.Err(), context.DeadlineExceeded) && ctx.ResponseWriter().Written() == 0 {
			pkg.Fail(ctx, pkg.REQUEST_TIMEOUT)
		}
	}
}
