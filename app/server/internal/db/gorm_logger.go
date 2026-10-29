package db

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

// slowQueryThreshold is when a query stops being routine. A forum's queries are
// index-backed lookups; anything past this is either a missing index or a
// pathological scan, and both are worth a log line and a counter bump.
const slowQueryThreshold = 200 * time.Millisecond

// queryObserver receives one call per executed statement. It is a package-level
// hook rather than a direct dependency because the metrics package already
// imports this one (metrics/collector.go reads the connection pool), so db
// importing metrics would be an import cycle.
//
// Nil by default: the db package works standalone in tests, and an unset hook
// is a no-op. metrics/db.go installs the real implementation in its init.
var queryObserver func(slow bool)

// SetQueryObserver installs the per-query hook. Called once by the metrics
// package; passing nil disables observation.
func SetQueryObserver(fn func(slow bool)) {
	queryObserver = fn
}

// newSlowQueryLogger returns a GORM logger that feeds the DB query metrics and
// logs slow statements.
//
// GORM's default logger writes every error to stdout through the standard
// library, bypassing LOG_LEVEL/LOG_FORMAT entirely. This one routes through
// logrus (so the configured level and JSON format apply) and reports both every
// query and the slow subset, which is what makes "database is slow right now"
// answerable without reading logs.
//
// The SQL text goes to the log only, never to a metric label: it is unbounded
// text and would blow up cardinality (see the observability cardinality guard).
func newSlowQueryLogger() logger.Interface {
	return &slowQueryLogger{
		level:     logger.Warn,
		threshold: slowQueryThreshold,
	}
}

type slowQueryLogger struct {
	level     logger.LogLevel
	threshold time.Duration
}

func (l *slowQueryLogger) LogMode(level logger.LogLevel) logger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *slowQueryLogger) Info(_ context.Context, _ string, _ ...any) {}

func (l *slowQueryLogger) Warn(_ context.Context, msg string, args ...any) {
	logrus.Warnf(msg, args...)
}

func (l *slowQueryLogger) Error(_ context.Context, msg string, args ...any) {
	logrus.Errorf(msg, args...)
}

// Trace is called by GORM after every statement. It is the single point where
// query metrics are emitted.
func (l *slowQueryLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), _ error) {
	elapsed := time.Since(begin)
	slow := elapsed > l.threshold

	if queryObserver != nil {
		queryObserver(slow)
	}

	if !slow {
		return
	}

	sql, rows := fc()
	logrus.WithFields(logrus.Fields{
		"latency_ms": elapsed.Milliseconds(),
		"rows":       rows,
		"sql":        sql,
	}).Warn("slow query")
}
