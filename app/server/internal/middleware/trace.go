package middleware

import (
	"feedchamber/internal/pkg/tracing"

	"github.com/kataras/iris/v12"
)

// Trace establishes the request trace and writes it into request.Context().
//
// It MUST be registered before Logger() as an app-level middleware: Iris runs
// app.Use handlers before per-party ones, so the trace cannot be established in
// internal/gateway's Tracing (which sits on the protected party) if the request log
// is to carry a trace id.
//
// Precedence when enabled: inbound traceparent > inbound X-Request-ID (32
// lowercase hex, which is nginx's $request_id shape) > freshly generated.
func Trace(tracingEnabled bool) iris.Handler {
	return func(ctx iris.Context) {
		tc := newTraceForRequest(ctx, tracingEnabled)

		req := ctx.Request()
		ctx.ResetRequest(req.WithContext(tracing.ContextWithTrace(req.Context(), tc)))

		// X-Request-ID keeps its existing behaviour (always emitted) so clients
		// and the proxy access log share one id with the app log.
		ctx.Header("X-Request-ID", tc.TraceIDHex())
		if tracingEnabled {
			ctx.Header("traceparent", tc.Traceparent())
		}

		ctx.Next()
	}
}

func newTraceForRequest(ctx iris.Context, tracingEnabled bool) tracing.Context {
	if !tracingEnabled {
		return tracing.New()
	}
	if inbound, ok := tracing.ParseTraceparent(ctx.GetHeader("Traceparent")); ok {
		return inbound.Child()
	}
	if id, ok := tracing.ParseTraceIDHex(ctx.GetHeader("X-Request-ID")); ok {
		if adopted, ok := tracing.FromTraceID(id); ok {
			return adopted
		}
	}
	return tracing.New()
}

// RequestTrace returns the trace established by Trace() for this request.
func RequestTrace(ctx iris.Context) (tracing.Context, bool) {
	return tracing.TraceFromContext(ctx.Request().Context())
}
