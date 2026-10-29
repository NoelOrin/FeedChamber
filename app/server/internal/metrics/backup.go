package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── Backup job metrics ─────────────────────────────────────

// Backup outcomes.
const (
	BackupOutcomeSucceeded = "succeeded"
	BackupOutcomeFailed    = "failed"
)

var (
	// BackupJobsTotal counts backup operations by kind and outcome.
	//
	// Backups are the one subsystem whose failure is silent until the day a
	// restore is needed: a nightly failure produces a log line nobody reads and
	// an old, still-listed "ready" row. kind separates create/restore/cleanup
	// because a failing restore is far more urgent than a failing cleanup.
	BackupJobsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_backup_jobs_total",
			Help: "Backup operations by kind and outcome.",
		},
		[]string{"kind", "outcome"},
	)

	// BackupJobDuration records how long each backup operation took.
	BackupJobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "feedchamber_backup_job_duration_seconds",
			Help:    "Backup operation latency in seconds.",
			Buckets: []float64{0.5, 1, 5, 15, 60, 300, 900, 3600},
		},
		[]string{"kind"},
	)
)

// Backup kinds. Static.
const (
	BackupKindCreate  = "create"
	BackupKindRestore = "restore"
	BackupKindCleanup = "cleanup"
)

func init() {
	Registry.MustRegister(BackupJobsTotal, BackupJobDuration)
	for _, kind := range []string{BackupKindCreate, BackupKindRestore, BackupKindCleanup} {
		BackupJobsTotal.WithLabelValues(kind, BackupOutcomeSucceeded)
		BackupJobsTotal.WithLabelValues(kind, BackupOutcomeFailed)
		BackupJobDuration.WithLabelValues(kind)
	}
}
