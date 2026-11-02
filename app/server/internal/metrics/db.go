package metrics

import (
	"feedchamber/internal/db"

	"github.com/prometheus/client_golang/prometheus"
)

// ── DB query metrics ───────────────────────────────────────

var (
	// DBSlowQueriesTotal counts queries that exceeded the slow-query threshold.
	//
	// Deliberately unlabeled: GORM's logger does not hand us a normalized SQL
	// shape, and raw SQL is unbounded text — exactly the kind of value the
	// cardinality guard forbids. The query itself belongs in the log line; this
	// counter only answers "is the database slow right now".
	DBSlowQueriesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "feedchamber_db_slow_queries_total",
			Help: "Queries slower than the configured slow-query threshold.",
		},
	)

	// DBQueriesTotal counts every query GORM executed, so the slow ratio is
	// meaningful. Without a denominator a rising slow count cannot be told
	// apart from rising traffic.
	DBQueriesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "feedchamber_db_queries_total",
			Help: "Queries executed through GORM.",
		},
	)
)

func init() {
	Registry.MustRegister(DBSlowQueriesTotal, DBQueriesTotal)
	// metrics already imports db (collector.go reads the connection pool), so
	// the wiring runs in this direction: db exposes a hook, metrics fills it.
	// The reverse would be an import cycle.
	db.SetQueryObserver(ObserveDBQuery)
}

// ObserveDBQuery records one executed query and whether it was slow.
func ObserveDBQuery(slow bool) {
	DBQueriesTotal.Inc()
	if slow {
		DBSlowQueriesTotal.Inc()
	}
}
