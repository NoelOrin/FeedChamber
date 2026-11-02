package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── NATS connection metrics ───────────────────────────────

// NATS client names. Both are static: KV and EventBus each open their own
// connection, so a failure in one does not imply the other.
const (
	NATSClientKV       = "kv"
	NATSClientEventBus = "eventbus"
)

var (
	// NATSConnected is 1 while the client's connection is usable. It is set to
	// 1 on construction (nats.Connect succeeded) and flipped by the disconnect
	// / reconnect handlers, so a network partition that outlives the 2s
	// reconnect wait is visible instead of only appearing as a log line.
	NATSConnected = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "feedchamber_nats_connected",
			Help: "1 while the NATS client connection is usable, 0 while disconnected.",
		},
		[]string{"client"},
	)

	// NATSDisconnectsTotal counts transitions out of a connected state.
	NATSDisconnectsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_nats_disconnects_total",
			Help: "NATS connection losses, by client.",
		},
		[]string{"client"},
	)

	// NATSReconnectsTotal counts successful recoveries. A reconnect count that
	// climbs while disconnects does not means the link is flapping.
	NATSReconnectsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_nats_reconnects_total",
			Help: "Successful NATS reconnections, by client.",
		},
		[]string{"client"},
	)
)

func init() {
	Registry.MustRegister(NATSConnected, NATSDisconnectsTotal, NATSReconnectsTotal)
	// Both client names are static, so seed them here rather than at the
	// wiring points: the family must exist even on a build that somehow skips
	// one of the two constructors.
	for _, client := range []string{NATSClientKV, NATSClientEventBus} {
		SeedNATSClient(client)
	}
}

// SeedNATSClient creates the zero-valued series for one client name and marks
// it connected. Callers are the producers (kvstore, eventbus), keeping the
// dependency direction producer → metrics.
func SeedNATSClient(client string) {
	NATSConnected.WithLabelValues(client).Set(1)
	NATSDisconnectsTotal.WithLabelValues(client)
	NATSReconnectsTotal.WithLabelValues(client)
}

// ObserveNATSDisconnected records a lost connection.
func ObserveNATSDisconnected(client string) {
	NATSConnected.WithLabelValues(client).Set(0)
	NATSDisconnectsTotal.WithLabelValues(client).Inc()
}

// ObserveNATSReconnected records a recovered connection.
func ObserveNATSReconnected(client string) {
	NATSConnected.WithLabelValues(client).Set(1)
	NATSReconnectsTotal.WithLabelValues(client).Inc()
}
