package metrics

import "github.com/prometheus/client_golang/prometheus"

// Webhook delivery outcomes.
const (
	WebhookOutcomeDelivered      = "delivered"
	WebhookOutcomeRetryScheduled = "retry_scheduled"
	WebhookOutcomeFailedTerminal = "failed_terminal"
)

// ── Webhook delivery metrics ───────────────────────────────

var (
	// WebhookDeliveriesTotal counts delivery attempts by outcome.
	WebhookDeliveriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_webhook_deliveries_total",
			Help: "Webhook delivery attempts by outcome.",
		},
		[]string{"outcome"},
	)

	// WebhookDeliveryDuration records the HTTP leg of a delivery.
	WebhookDeliveryDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "feedchamber_webhook_delivery_duration_seconds",
			Help:    "Webhook delivery request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	// WebhookPendingDeliveries is the true pending backlog, counted by a
	// COUNT(*) query on every worker tick.
	//
	// It used to be set from len(ListPendingDeliveries(ctx, 20)) — the worker's
	// batch size — so the gauge saturated at 20 no matter how deep the backlog
	// actually was. A backlog of 500 looked identical to 20 on the dashboard.
	WebhookPendingDeliveries = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "feedchamber_webhook_pending_deliveries",
			Help: "Webhook deliveries currently pending (true backlog).",
		},
	)
)

func init() {
	Registry.MustRegister(WebhookDeliveriesTotal, WebhookDeliveryDuration, WebhookPendingDeliveries)
	WebhookDeliveriesTotal.WithLabelValues(WebhookOutcomeDelivered)
	WebhookDeliveriesTotal.WithLabelValues(WebhookOutcomeRetryScheduled)
	WebhookDeliveriesTotal.WithLabelValues(WebhookOutcomeFailedTerminal)
}
