package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"orinforum/internal/pkg"
	"orinforum/internal/pkg/logger"
	"orinforum/internal/pkg/tracing"

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

// bodyLimitWriter captures the response so that after the handler runs, the
// middleware can inspect it and replace a MaxBytesReader error (Iris writes
// it as HTTP 400 + "http: request body too large") with a proper 413 JSON.
// It embeds the original iris.ResponseWriter so all interface methods are
// delegated automatically; only WriteHeader and Write are intercepted.
type bodyLimitWriter struct {
	irisctx.ResponseWriter
	statusCode int
	body       bytes.Buffer
	written    bool
}

func (w *bodyLimitWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
}

func (w *bodyLimitWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.statusCode = http.StatusOK
		w.written = true
	}
	return w.body.Write(b)
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
		// SSE streams write continuously and never return — bodyLimit
		// buffers the response until the handler finishes, which would
		// trap SSE data in the buffer forever. Skip entirely.
		if strings.HasSuffix(ctx.Path(), "/stream") {
			ctx.Next()
			return
		}

		maxBytes := defaultLimit
		if overrides != nil {
			for prefix, limit := range overrides {
				if strings.HasPrefix(ctx.Path(), prefix) {
					maxBytes = limit
					break
				}
			}
		}

		ctx.Request().Body = http.MaxBytesReader(ctx.ResponseWriter(), ctx.Request().Body, maxBytes)

		// Wrap body to detect MaxBytesError via type assertion instead of
		// fragile strings.Contains on the response body.
		bodyWrap := &limitExceededReader{r: ctx.Request().Body}
		ctx.Request().Body = bodyWrap

		// Replace the response writer with a capture writer so that if
		// the handler produces a body-too-large error response (HTTP 400),
		// we can intercept and rewrite it as 413 before it reaches the client.
		realWriter := ctx.ResponseWriter()
		cap := &bodyLimitWriter{ResponseWriter: realWriter, statusCode: 0}
		ctx.ResetResponseWriter(cap)

		ctx.Next()

		// Restore the real response writer.
		ctx.ResetResponseWriter(realWriter)

		isBodyTooLarge := bodyWrap.exceeded && cap.written &&
			cap.statusCode == http.StatusBadRequest

		if isBodyTooLarge {
			// StopWithJSON writes directly to the response writer,
			// bypassing Iris's cached status code, so the client
			// receives the correct HTTP 413.
			ctx.StopWithJSON(iris.StatusRequestEntityTooLarge, iris.Map{
				"code": int(pkg.BODY_TOO_LARGE),
				"msg":  pkg.GetErrMsg(pkg.BODY_TOO_LARGE),
			})
			return
		}

		// No body-too-large error: flush the captured response as-is.
		if cap.written {
			realWriter.Header().Set("Content-Type", ctx.GetContentType())
			if cap.statusCode > 0 {
				realWriter.WriteHeader(cap.statusCode)
			}
			if cap.body.Len() > 0 {
				_, _ = realWriter.Write(cap.body.Bytes())
			}
		}
	}
}
