package middleware

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/logger"
	"feedchamber/internal/pkg/tracing"

	"github.com/kataras/iris/v12"
	irisctx "github.com/kataras/iris/v12/context"
)

func CORS() iris.Handler {
	return func(ctx iris.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if ctx.Method() == "OPTIONS" {
			ctx.StatusCode(iris.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

func Logger() iris.Handler {
	return func(ctx iris.Context) {
		start := time.Now()

		tc, _ := RequestTrace(ctx)

		// 请求级 logger 存入 request context，Service/Repository 层用
		// logger.FromContext(ctx) 即可取到带 trace_id 的同一实例。
		reqLog := logger.Global().With(
			logger.F("trace_id", tc.TraceIDHex()),
			logger.F("span_id", tc.SpanIDHex()),
			logger.F("method", ctx.Method()),
			logger.F("path", ctx.Path()),
		)
		req := ctx.Request()
		ctx.ResetRequest(req.WithContext(logger.NewContext(req.Context(), reqLog)))

		ctx.Next()

		// 请求结束后写入 user_uuid（由 auth 中间件设置）
		userUUID := ctx.Values().GetString(pkg.CtxUserID)
		fields := []logger.Field{
			logger.F("status", ctx.GetStatusCode()),
			logger.F("latency_ms", time.Since(start).Milliseconds()),
			logger.F("user_uuid", userUUID),
			logger.F("ip", ctx.RemoteAddr()),
		}
		// 网关在路由匹配后把端点模板写回 trace，这里才拿得到 route。
		if finalTC, ok := tracing.TraceFromContext(ctx.Request().Context()); ok && finalTC.Name != "" {
			fields = append(fields, logger.F("route", finalTC.Name))
		}
		reqLog.With(fields...).Info("request completed")
	}
}

// bodyLimitWriter 把「请求体超限」翻译成 413 信封，且 **不缓冲** 正常响应。
//
// 之前它无条件把整个响应收集进 bytes.Buffer（为了事后把 Iris 的 400 改写成
// 413），代价是所有出口——包括 /api/v1/upload/file/{key} 这种字节流——都要在
// 内存里攒完整份响应才发出。实测：10MB 文件下载首个字节直到 handler 返回后
// 才到达客户端，且并发下载时内存按文件大小成倍占用。
//
// 现在的做法：正常响应直接透传（流式）；只有读取阶段已经判定超限时，才把
// handler 的错误响应整份换成 413 信封。超限发生在 handler 读 body 的那一刻，
// 早于它写响应，所以状态码还有机会改写。
type bodyLimitWriter struct {
	irisctx.ResponseWriter
	probe     *limitExceededReader
	rewritten bool
}

// tooLargeBody 是超限时写出的信封。预序列化，避免在 Write 路径里再走 ctx.JSON
// （那会递归回到本 writer）。
var tooLargeBody = []byte(`{"code":` + strconv.Itoa(int(pkg.BODY_TOO_LARGE)) +
	`,"msg":"` + pkg.GetErrMsg(pkg.BODY_TOO_LARGE) + `","data":null}`)

func (w *bodyLimitWriter) needsRewrite() bool {
	return w.probe.exceeded && !w.rewritten
}

// WriteHeader 在超限时立即写出 413 与信封。
//
// 必须在这里就写 body：handler 通常先 WriteHeader（经 pkg.Fail 的
// ctx.StatusCode）再写 body。若只在 WriteHeader 改状态码，handler 的错误
// body 仍会被写下去；若留到 Write 再判断，响应早已提交，补写会失败。
func (w *bodyLimitWriter) WriteHeader(code int) {
	if w.needsRewrite() {
		w.rewritten = true
		w.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.ResponseWriter.Write(tooLargeBody)
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyLimitWriter) Write(b []byte) (int, error) {
	if w.needsRewrite() {
		// handler 未先 WriteHeader 就直接写：这里补状态码与信封。
		w.rewritten = true
		w.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
		if _, err := w.ResponseWriter.Write(tooLargeBody); err != nil {
			return 0, err
		}
		return len(b), nil
	}
	if w.rewritten {
		// 已改写过：丢弃 handler 后续写入（那是参数错误的 body）。
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

// PathBodyLimit returns middleware that applies per-path body size limits.
// The defaultLimit is used for all requests whose path does not match any
// entry in overrides. Each override maps a URL prefix to a custom limit.
// For example: PathBodyLimit(1<<20, map[string]int64{"/upload/": 10<<20})
func PathBodyLimit(defaultLimit int64, overrides map[string]int64) iris.Handler {
	return bodyLimit(defaultLimit, overrides)
}

// limitExceededReader wraps request body to detect MaxBytesError via type
// assertion, replacing the fragile strings.Contains check on the response body.
type limitExceededReader struct {
	r        io.ReadCloser
	exceeded bool
}

func (l *limitExceededReader) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	var maxBytesErr *http.MaxBytesError
	if err != nil && errors.As(err, &maxBytesErr) {
		l.exceeded = true
	}
	return n, err
}

func (l *limitExceededReader) Close() error {
	return l.r.Close()
}

func bodyLimit(defaultLimit int64, overrides map[string]int64) iris.Handler {
	return func(ctx iris.Context) {
		// SSE 流没有请求体，且写完即挂起；与请求体上限无关。
		if strings.HasSuffix(ctx.Path(), "/stream") {
			ctx.Next()
			return
		}

		// 无请求体的方法不可能超限，直接放行。
		//
		// 这不只是省一次包装：/api/v1/upload/file/{key} 是 GET 文件下载出口，
		// 让它完全不经过任何 ResponseWriter 包装，字节流直接透传到网络。
		switch ctx.Method() {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			ctx.Next()
			return
		}

		maxBytes := defaultLimit
		for prefix, limit := range overrides {
			if strings.HasPrefix(ctx.Path(), prefix) {
				maxBytes = limit
				break
			}
		}

		probe := &limitExceededReader{
			r: http.MaxBytesReader(ctx.ResponseWriter(), ctx.Request().Body, maxBytes),
		}
		ctx.Request().Body = probe

		realWriter := ctx.ResponseWriter()
		ctx.ResetResponseWriter(&bodyLimitWriter{ResponseWriter: realWriter, probe: probe})

		ctx.Next()

		ctx.ResetResponseWriter(realWriter)
	}
}
