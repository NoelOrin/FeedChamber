package middleware

import (
	"runtime/debug"

	"feedchamber/internal/metrics"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/logger"

	"github.com/kataras/iris/v12"
)

// Recover converts a panic anywhere downstream — party middleware, the gateway
// pipeline, a handler, or a service it calls — into a standard 500 envelope
// instead of letting it reach net/http, which recovers per-connection and drops
// the socket without a response.
//
// Register it after Logger: app-level middleware wraps the deeper chain, so a
// panic unwinds into this deferred func first and the request log still gets
// written, with the status this middleware set.
func Recover() iris.Handler {
	return func(ctx iris.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			metrics.PanicsTotal.Inc()
			// The stack goes to the log only; the client just sees code 5001.
			logger.FromContext(ctx.Request().Context()).
				With(
					logger.F("panic", r),
					logger.F("method", ctx.Method()),
					logger.F("path", ctx.Path()),
				).
				Error("panic recovered", logger.F("stack", string(debug.Stack())))

			// A partial body cannot be replaced; mark the error and let net/http
			// close the connection, which is the only honest option once bytes
			// are on the wire.
			if ctx.ResponseWriter().Written() > 0 {
				ctx.StopExecution()
				return
			}
			pkg.Fail(ctx, pkg.INTERNAL_ERROR)
			ctx.StopExecution()
		}()
		ctx.Next()
	}
}
