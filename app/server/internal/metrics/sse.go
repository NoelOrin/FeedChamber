package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── SSE connection metrics ─────────────────────────────────

// SSE channels. Static: both are known at build time.
const (
	SSEChannelNotification = "notification"
	SSEChannelRealtime     = "realtime"
)

var (
	// SSEConnections tracks live server-sent-event streams. Neither handler
	// keeps a registry, so each connection inc/defs itself on entry/exit.
	SSEConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "orinforum_sse_connections",
			Help: "Currently open SSE streams by channel.",
		},
		[]string{"channel"},
	)

	// SSEWriteFailuresTotal counts failed writes to an SSE stream. Before this
	// existed, a dead stream just returned from the handler: the only visible
	// trace was the connection gauge dropping, which is indistinguishable from
	// a client closing the tab. A climbing rate against a flat connection count
	// means the server cannot deliver to clients that are still connected
	// (typically a proxy buffering or dropping the stream).
	SSEWriteFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orinforum_sse_write_failures_total",
			Help: "Failed SSE writes that terminated a stream, by channel.",
		},
		[]string{"channel"},
	)
)

func init() {
	Registry.MustRegister(SSEConnections, SSEWriteFailuresTotal)
	// Seed the known channels so both families exist before first connect.
	for _, channel := range []string{SSEChannelNotification, SSEChannelRealtime} {
		SSEConnections.WithLabelValues(channel)
		SSEWriteFailuresTotal.WithLabelValues(channel)
	}
}

// ObserveSSEWriteFailure records a write error that ends a stream.
func ObserveSSEWriteFailure(channel string) {
	SSEWriteFailuresTotal.WithLabelValues(channel).Inc()
}
