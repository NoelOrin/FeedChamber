package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── Permission decision metrics ────────────────────────────

var (
	// PermissionDecisionsTotal counts permission lookups by scope and verdict.
	// Roles are deliberately not a label: see the cardinality guard.
	PermissionDecisionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orinforum_permission_decisions_total",
			Help: "Permission checks by scope (global|space) and result (allow|deny).",
		},
		[]string{"scope", "result"},
	)
)

func init() {
	Registry.MustRegister(PermissionDecisionsTotal)
	// Seed so a fresh instance still exports the family.
	PermissionDecisionsTotal.WithLabelValues("global", "allow")
	PermissionDecisionsTotal.WithLabelValues("global", "deny")
	PermissionDecisionsTotal.WithLabelValues("space", "allow")
	PermissionDecisionsTotal.WithLabelValues("space", "deny")
}

// ObservePermission records one authorization decision.
func ObservePermission(scope string, allowed bool) {
	result := "deny"
	if allowed {
		result = "allow"
	}
	PermissionDecisionsTotal.WithLabelValues(scope, result).Inc()
}
