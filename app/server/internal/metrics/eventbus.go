package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── EventBus metrics ───────────────────────────────────────

var (
	// EventbusPublishTotal counts hand-offs to NATS by outcome.
	EventbusPublishTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_eventbus_publish_total",
			Help: "Events published to NATS by event type and result.",
		},
		[]string{"event", "result"},
	)

	// EventbusDispatchDuration records per-handler dispatch latency.
	EventbusDispatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "feedchamber_eventbus_dispatch_duration_seconds",
			Help:    "Event handler execution time in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event"},
	)

	// EventbusDispatchErrorsTotal counts recovered panics in handlers.
	EventbusDispatchErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_eventbus_dispatch_errors_total",
			Help: "Event handler panics recovered during dispatch.",
		},
		[]string{"event"},
	)

	// EventbusQueueDepth samples the in-process buffer occupancy.
	EventbusQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "feedchamber_eventbus_queue_depth",
			Help: "Number of events buffered and not yet dispatched.",
		},
	)

	// EventbusDroppedTotal counts events lost to a full buffer.
	EventbusDroppedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_eventbus_dropped_total",
			Help: "Events dropped because the dispatch buffer was full.",
		},
		[]string{"event"},
	)
)

func init() {
	Registry.MustRegister(
		EventbusPublishTotal,
		EventbusDispatchDuration,
		EventbusDispatchErrorsTotal,
		EventbusQueueDepth,
		EventbusDroppedTotal,
	)
}

// SeedEventbusEvent creates the zero-valued series for one event type.
//
// The label values are event names, which this package cannot enumerate without
// importing eventbus (metrics ← eventbus already, so the reverse import would
// be a cycle). The producer therefore seeds its own type list from New(), which
// keeps the absent() canary in deploy/prometheus/rules/feedchamber.yml meaningful:
// a missing family means "wiring never ran", not "no traffic yet".
func SeedEventbusEvent(eventType string) {
	EventbusPublishTotal.WithLabelValues(eventType, "ok")
	EventbusDispatchDuration.WithLabelValues(eventType)
	EventbusDispatchErrorsTotal.WithLabelValues(eventType)
	EventbusDroppedTotal.WithLabelValues(eventType)
}
