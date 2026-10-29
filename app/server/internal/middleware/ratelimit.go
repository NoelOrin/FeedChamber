package middleware

import (
	"time"

	"orinforum/internal/metrics"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/ratelimit"

	"github.com/kataras/iris/v12"
)

// RateLimit returns a middleware that enforces a per-IP budget for one named
// endpoint. It shares pkg/ratelimit's engine with internal/gateway's global per-IP
// ceiling; the two differ only in where the policy comes from — this one is
// declared at route registration (login, register, post creation, ...), the
// global one is toggled by the rate_limit_enabled setting.
//
// If the limiter is nil the middleware is a no-op (fail open).
//
// Parameters:
//   - limiter:  the shared *ratelimit.Limiter instance
//   - endpoint: a short label naming the policy in metrics and logs (e.g. "login")
//   - limit:    maximum number of allowed requests per window
//   - window:   rolling window duration
func RateLimit(limiter *ratelimit.Limiter, endpoint string, limit int, window time.Duration) iris.Handler {
	// Seed the policy's zero-valued series at registration time, so the family
	// exists before the first rejection (see metrics.SeedRatelimitPolicy).
	metrics.SeedRatelimitPolicy(endpoint)
	return func(ctx iris.Context) {
		if limiter == nil {
			ctx.Next()
			return
		}

		key := ratelimit.EndpointKey(pkg.ClientIP(ctx), endpoint)
		reqCtx := ctx.Request().Context()

		if limiter.Check(reqCtx, endpoint, key, limit, window) == ratelimit.Denied {
			pkg.Fail(ctx, pkg.RATE_LIMITED)
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}
