package kvstore

import (
	"errors"
	"time"

	"feedchamber/internal/metrics"

	"github.com/nats-io/nats.go/jetstream"
)

// observe times one KV operation and classifies the outcome.
//
// "miss" is separated from "error" on purpose: ErrKeyNotFound is the normal
// answer for a cache lookup, and folding it into the error rate would make the
// alerting rule useless.
//
// Use as: defer observe("get", &err)() with named return values.
func observe(op string, err *error) func() {
	start := time.Now()
	return func() {
		outcome := "ok"
		switch {
		case err == nil || *err == nil:
		case errors.Is(*err, ErrKeyNotFound), errors.Is(*err, jetstream.ErrKeyNotFound):
			outcome = "miss"
		default:
			outcome = "error"
		}
		metrics.KVOperationDuration.WithLabelValues(op, outcome).Observe(time.Since(start).Seconds())
	}
}

// seedOperationMetrics creates the zero-valued series for every op name so the
// family exists before the first KV call. Without this a healthy, freshly
// started instance looks the same as broken wiring to the absent() canary in
// deploy/prometheus/rules/feedchamber.yml.
func seedOperationMetrics() {
	for _, op := range observedOps {
		metrics.SeedKVOperation(op)
	}
}
