package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ── KV store metrics ───────────────────────────────────────

var (
	// KVOperationDuration records latency of each KV operation.
	KVOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "orinforum_kv_operation_duration_seconds",
			Help:    "KV store operation latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"op", "outcome"},
	)

	// KVCasExhaustedTotal counts compare-and-swap retries that gave up.
	KVCasExhaustedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "orinforum_kv_cas_exhausted_total",
			Help: "KV increment operations that exhausted all CAS attempts.",
		},
	)
)

func init() {
	Registry.MustRegister(KVOperationDuration, KVCasExhaustedTotal)
}

// ObserveKV records one KV operation. outcome is "ok" or "error".
func ObserveKV(op string, start time.Time, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	KVOperationDuration.WithLabelValues(op, outcome).Observe(time.Since(start).Seconds())
}

// SeedKVOperation creates the zero-valued series for one KV operation name.
// Callers are the producer (kvstore), which owns the op vocabulary and already
// imports this package — see SeedEventbusEvent for why metrics must not know it.
func SeedKVOperation(op string) {
	KVOperationDuration.WithLabelValues(op, "ok")
}
