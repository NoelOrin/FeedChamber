package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry is the default Prometheus registry used by OrinForum.
var Registry = prometheus.NewRegistry()

func init() {
	Registry.MustRegister(collectors.NewGoCollector())
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// ── HTTP request metrics ─────────────────────────────────────

var (
	// RequestDuration records HTTP request latency in seconds.
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "orinforum_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// RequestsTotal counts HTTP requests by method, path, and status.
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orinforum_http_requests_total",
			Help: "Total count of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// RequestSizeBytes records request body sizes.
	RequestSizeBytes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "orinforum_http_request_size_bytes",
			Help:    "HTTP request body size in bytes.",
			Buckets: prometheus.ExponentialBuckets(64, 2, 12), // 64B to 128KB
		},
	)

	// ResponseSizeBytes records response body sizes.
	ResponseSizeBytes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "orinforum_http_response_size_bytes",
			Help:    "HTTP response body size in bytes.",
			Buckets: prometheus.ExponentialBuckets(64, 2, 12),
		},
	)

	// PanicsTotal counts panics recovered by middleware.Recover. Deliberately
	// unlabeled: the panic value and route are unbounded/high-cardinality and
	// belong in the log, not in a metric label.
	PanicsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "orinforum_http_panics_total",
			Help: "Total count of panics recovered while serving HTTP requests.",
		},
	)
)

func init() {
	Registry.MustRegister(
		RequestDuration,
		RequestsTotal,
		RequestSizeBytes,
		ResponseSizeBytes,
		PanicsTotal,
	)
	// Seed zero values so metric families appear in /metrics immediately.
	RequestsTotal.WithLabelValues("GET", "/health", "200")
	RequestDuration.WithLabelValues("GET", "/health", "200")
}
