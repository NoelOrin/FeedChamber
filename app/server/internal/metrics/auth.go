package metrics

import "github.com/prometheus/client_golang/prometheus"

// Login failure reasons. Values MUST stay bounded: they come from the three
// distinct branches in service/auth.go Login, never from error text.
const (
	LoginReasonUnknownUser  = "unknown_user"
	LoginReasonNoCredential = "no_credential"
	LoginReasonBadPassword  = "bad_password"
)

// ── Auth metrics ───────────────────────────────────────────

var (
	// LoginFailuresTotal counts rejected logins by reason. Brute-force
	// threshold events stay in SecurityEvent storage; this is the trend view.
	LoginFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_login_failures_total",
			Help: "Rejected login attempts by failure reason.",
		},
		[]string{"reason"},
	)
)

func init() {
	Registry.MustRegister(LoginFailuresTotal)
	LoginFailuresTotal.WithLabelValues(LoginReasonUnknownUser)
	LoginFailuresTotal.WithLabelValues(LoginReasonNoCredential)
	LoginFailuresTotal.WithLabelValues(LoginReasonBadPassword)
}
