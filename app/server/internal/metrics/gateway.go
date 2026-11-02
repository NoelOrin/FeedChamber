package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── Gateway metrics ────────────────────────────────────────

var (
	// RatelimitRejectionsTotal counts requests turned away by a limiter.
	// name 是策略名，取自同一个命名空间：全局每 IP 上限为 "global"
	// （internal/gateway.RateLimiter），其余是路由上声明的每端点预算
	// （login / register / post_create / …，internal/middleware.RateLimit）。
	// 两类策略共用 pkg/ratelimit 引擎，因此指标、日志与失败语义完全一致。
	RatelimitRejectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_ratelimit_rejections_total",
			Help: "Requests rejected by a rate limiter, by policy name.",
		},
		[]string{"name"},
	)

	// RatelimitFailopenTotal counts limiters that allowed traffic through
	// after the KV backend failed, i.e. protection was silently absent.
	RatelimitFailopenTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_ratelimit_failopen_total",
			Help: "Rate limiter fail-open occurrences, by policy name.",
		},
		[]string{"name"},
	)
)

func init() {
	Registry.MustRegister(RatelimitRejectionsTotal, RatelimitFailopenTotal)
}

// SeedRatelimitPolicy creates the zero-valued series for one policy name.
//
// Policy names are chosen by the call sites (route registration for endpoint
// budgets, the constant "global" for the gateway ceiling), so this package
// cannot enumerate them. Each wiring point calls this once at construction, so
// the family exists before the first rejection — otherwise a healthy instance
// that never saw abuse would look identical to a broken instrumentation to the
// absent() canary in deploy/prometheus/rules/feedchamber.yml.
func SeedRatelimitPolicy(name string) {
	RatelimitRejectionsTotal.WithLabelValues(name)
	RatelimitFailopenTotal.WithLabelValues(name)
}
