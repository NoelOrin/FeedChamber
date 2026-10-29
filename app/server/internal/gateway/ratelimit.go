package gateway

import (
	"context"
	"time"

	"orinforum/internal/metrics"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/ratelimit"

	"github.com/kataras/iris/v12"
)

// globalPolicyName is the metrics/log label for the gateway's single global
// per-IP ceiling. It runs through the same pkg/ratelimit engine as the named
// per-endpoint budgets in middleware.RateLimit, so the two now share key shape,
// failure semantics, metrics and log format; they differ only in where the
// policy comes from (settings toggle vs route registration).
const globalPolicyName = "global"

// RateLimiter provides the global per-IP rate limiting middleware.
type RateLimiter struct {
	limiter *ratelimit.Limiter
	reader  pkg.SettingReader
}

// rateLimitConfig holds the effective global policy.
type rateLimitConfig struct {
	Enabled bool
	Limit   int
	Window  time.Duration
}

// NewRateLimiter creates a new RateLimiter instance. A nil deps.KV yields an
// allow-everything limiter inside the engine; a wrongly wired type is a compile
// error rather than a silent degradation.
func NewRateLimiter(deps Deps) *RateLimiter {
	// Seed the global policy's zero-valued series at construction, so a healthy
	// instance that never hit the ceiling still exports the family (the alert
	// rules use absent() as a wiring canary).
	metrics.SeedRatelimitPolicy(globalPolicyName)
	return &RateLimiter{
		limiter: ratelimit.NewLimiter(deps.KV),
		reader:  deps.SettingService,
	}
}

// loadConfig reads the rate limit toggle from settings and returns the config.
// Defaults: enabled=false (opt-in), 60 requests per 1 minute window.
func (r *RateLimiter) loadConfig(ctx context.Context) rateLimitConfig {
	cfg := rateLimitConfig{
		Enabled: false,
		Limit:   60,
		Window:  time.Minute,
	}
	if r.reader == nil {
		return cfg
	}
	val, err := r.reader.Get(ctx, model.SettingRateLimitEnabled)
	if err != nil || val != "true" {
		return cfg
	}
	cfg.Enabled = true
	return cfg
}

// Middleware returns true if the request is allowed (or rate limiting is
// disabled / unavailable). Returns false and writes a 429 response when the
// client has exceeded the limit. Fail-open cases allow through and are recorded
// by the engine.
func (r *RateLimiter) Middleware(ctx iris.Context) bool {
	reqCtx := ctx.Request().Context()

	cfg := r.loadConfig(reqCtx)
	if !cfg.Enabled {
		return true
	}

	key := ratelimit.GlobalKey(pkg.ClientIP(ctx))
	if r.limiter.Check(reqCtx, globalPolicyName, key, cfg.Limit, cfg.Window) == ratelimit.Denied {
		pkg.Fail(ctx, pkg.RATE_LIMITED)
		ctx.StopExecution()
		return false
	}
	return true
}
