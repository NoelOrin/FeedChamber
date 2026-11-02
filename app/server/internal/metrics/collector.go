package metrics

import (
	"feedchamber/internal/db"

	"github.com/prometheus/client_golang/prometheus"
)

// dbPoolCollector exposes database connection pool stats as Prometheus metrics.
type dbPoolCollector struct {
	store db.Store

	maxOpenConns *prometheus.Desc
	openConns    *prometheus.Desc
	inUse        *prometheus.Desc
	idle         *prometheus.Desc
	waitCount    *prometheus.Desc
	waitDuration *prometheus.Desc
}

// RegisterDBPoolCollector registers the DB pool stats collector with the
// default registry. Call once after the DB store is initialized.
func RegisterDBPoolCollector(store db.Store) {
	c := &dbPoolCollector{
		store: store,
		maxOpenConns: prometheus.NewDesc(
			"feedchamber_db_pool_max_open_connections",
			"Maximum number of open connections to the database.",
			nil, nil,
		),
		openConns: prometheus.NewDesc(
			"feedchamber_db_pool_open_connections",
			"Number of currently open connections to the database.",
			nil, nil,
		),
		inUse: prometheus.NewDesc(
			"feedchamber_db_pool_in_use",
			"Number of database connections currently in use.",
			nil, nil,
		),
		idle: prometheus.NewDesc(
			"feedchamber_db_pool_idle",
			"Number of idle database connections.",
			nil, nil,
		),
		waitCount: prometheus.NewDesc(
			"feedchamber_db_pool_wait_count_total",
			"Total number of connections waited for.",
			nil, nil,
		),
		waitDuration: prometheus.NewDesc(
			"feedchamber_db_pool_wait_duration_seconds_total",
			"Total time waited for connections in seconds.",
			nil, nil,
		),
	}
	Registry.MustRegister(c)
}

func (c *dbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.maxOpenConns
	ch <- c.openConns
	ch <- c.inUse
	ch <- c.idle
	ch <- c.waitCount
	ch <- c.waitDuration
}

func (c *dbPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.store.PoolStats()
	if stats == nil {
		// Backend does not expose pool stats. Export zeros
		// so alerts don't fire on missing metric.
		ch <- prometheus.MustNewConstMetric(c.maxOpenConns, prometheus.GaugeValue, 0)
		ch <- prometheus.MustNewConstMetric(c.openConns, prometheus.GaugeValue, 0)
		ch <- prometheus.MustNewConstMetric(c.inUse, prometheus.GaugeValue, 0)
		ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, 0)
		ch <- prometheus.MustNewConstMetric(c.waitCount, prometheus.CounterValue, 0)
		ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, 0)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.maxOpenConns, prometheus.GaugeValue, float64(stats.MaxOpenConnections))
	ch <- prometheus.MustNewConstMetric(c.openConns, prometheus.GaugeValue, float64(stats.OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.inUse, prometheus.GaugeValue, float64(stats.InUse))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stats.Idle))
	ch <- prometheus.MustNewConstMetric(c.waitCount, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, float64(stats.WaitDurationMs)/1000.0)
}
