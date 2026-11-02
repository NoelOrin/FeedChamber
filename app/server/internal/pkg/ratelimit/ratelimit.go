// Package ratelimit is the single rate-limiting engine for the server.
//
// Both HTTP entry points go through Limiter.Check: internal/gateway's global per-IP
// ceiling and the per-endpoint budgets declared at route registration
// (login, register, post creation, ...). Before this existed twice: the gateway
// carried its own copy of the SetNX/Incr/compare window, with its own key shape,
// its own metrics call sites and its own log wording — so "is this request
// limited, and what did we record about it" had two answers.
package ratelimit

import (
	"context"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/pkg/logger"
)

// Outcome is the result of one limiter check.
type Outcome int

const (
	// Allowed: within budget.
	Allowed Outcome = iota
	// Denied: budget exhausted; the caller must reject the request.
	Denied
	// FailOpen: the KV backend failed, so protection was absent for this
	// request. Callers keep serving — a storage hiccup must not lock users out —
	// but the metric makes the gap visible.
	FailOpen
)

const counterPrefix = "rl:"

// EndpointKey is the counter key for a named per-endpoint policy.
// Storage format is load-bearing: changing it silently resets every window
// that is currently counting.
func EndpointKey(ip, policy string) string {
	return counterPrefix + ip + ":" + policy
}

// GlobalKey is the counter key for the global per-IP ceiling. The extra
// "ratelimit:" segment is the shape the gateway has always used; it is kept so
// upgrading does not drop in-flight windows.
func GlobalKey(ip string) string {
	return counterPrefix + "ratelimit:" + ip
}

// Limiter provides rate limiting backed by a kvstore.Store.
type Limiter struct {
	kv kvstore.Store
}

// NewLimiter creates a new Limiter using the given KV store.
func NewLimiter(kv kvstore.Store) *Limiter {
	return &Limiter{kv: kv}
}

// Check performs one windowed count for a named policy and records the outcome
// (metric + structured log line, trace-correlated via ctx) so callers only have
// to decide what to do about it.
//
// Strategy (sliding-window approximation via atomic counter):
//  1. SetNX initialises the counter to "0" with TTL=window — the first caller
//     starts the window.
//  2. Incr atomically increments.
//  3. A count above limit denies.
//
// SetNX and Incr are separate operations, so two concurrent first-calls can both
// initialise; the counter may overshoot the limit by the number of racing
// goroutines, which is acceptable for abuse prevention.
func (l *Limiter) Check(ctx context.Context, policy, key string, limit int, window time.Duration) Outcome {
	if l == nil || l.kv == nil {
		return Allowed
	}

	if _, err := l.kv.SetNX(ctx, key, "0", window); err != nil {
		l.recordFailure(ctx, policy, err)
		return FailOpen
	}

	count, err := l.kv.Incr(ctx, key)
	if err != nil {
		l.recordFailure(ctx, policy, err)
		return FailOpen
	}

	if count > int64(limit) {
		metrics.RatelimitRejectionsTotal.WithLabelValues(policy).Inc()
		return Denied
	}
	return Allowed
}

// recordFailure counts and logs a fail-open event: protection was silently
// absent for this request. The logger comes from ctx so the line carries the
// request's trace_id.
func (l *Limiter) recordFailure(ctx context.Context, policy string, err error) {
	metrics.RatelimitFailopenTotal.WithLabelValues(policy).Inc()
	logger.FromContext(ctx).
		WithComponent("ratelimit").
		WithError(err).
		Warn("rate limit backend failed, failing open", logger.F("policy", policy))
}
