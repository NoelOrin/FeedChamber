package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── Notification dispatcher metrics ───────────────────────

var (
	// NotifDispatcherDroppedTotal counts notification-create tasks dropped
	// because the bounded dispatcher queue was full. The queue is fed from
	// EventBus workers; a full queue must never block them (see
	// NotifDispatcher.enqueue), so the overflow is shed and made visible here.
	NotifDispatcherDroppedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_notif_dispatcher_dropped_total",
			Help: "Notification create tasks dropped because the bounded dispatcher queue was full, by notification type.",
		},
		[]string{"type"},
	)
)

func init() {
	Registry.MustRegister(NotifDispatcherDroppedTotal)
}
