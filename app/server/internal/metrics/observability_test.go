package metrics

import (
	"sort"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// familiesGathered returns the metric family names currently exported.
func familiesGathered(t *testing.T) map[string]bool {
	t.Helper()
	mfs, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Registry.Gather() error = %v", err)
	}
	out := make(map[string]bool, len(mfs))
	for _, mf := range mfs {
		out[mf.GetName()] = true
	}
	return out
}

// TestSeededFamiliesExist locks the behaviour that families whose label values
// are statically known are exported before any traffic, so dashboards and alert
// rules do not start out "no data".
func TestSeededFamiliesExist(t *testing.T) {
	got := familiesGathered(t)

	for _, name := range []string{
		"orinforum_http_requests_total",
		"orinforum_http_request_duration_seconds",
		"orinforum_permission_decisions_total",
		"orinforum_sse_connections",
		"orinforum_login_failures_total",
		"orinforum_webhook_deliveries_total",
		"orinforum_kv_cas_exhausted_total",
		"orinforum_eventbus_queue_depth",
		"orinforum_webhook_pending_deliveries",
		"orinforum_webhook_delivery_duration_seconds",
		"orinforum_nats_connected",
		"orinforum_nats_disconnects_total",
		"orinforum_nats_reconnects_total",
		"orinforum_sse_write_failures_total",
		"orinforum_db_queries_total",
		"orinforum_db_slow_queries_total",
		"orinforum_permission_cache_entries",
		"orinforum_backup_jobs_total",
		"orinforum_backup_job_duration_seconds",
	} {
		if !got[name] {
			t.Errorf("family %s missing before first observation", name)
		}
	}
}

// TestLabelledFamiliesAppearAfterObservation covers the families whose label
// values are only known at runtime (event names, KV ops, limiter names). Their
// producers seed zero-valued series via SeedEventbusEvent / SeedKVOperation /
// SeedRatelimitPolicy; this test pins that observation still exports them.
func TestLabelledFamiliesAppearAfterObservation(t *testing.T) {
	start := time.Now()

	EventbusPublishTotal.WithLabelValues("post.created", "ok").Inc()
	EventbusDispatchDuration.WithLabelValues("post.created").Observe(time.Since(start).Seconds())
	EventbusDispatchErrorsTotal.WithLabelValues("post.created").Inc()
	EventbusDroppedTotal.WithLabelValues("post.created").Inc()
	ObserveKV("get", start, nil)
	RatelimitRejectionsTotal.WithLabelValues("gateway").Inc()
	RatelimitFailopenTotal.WithLabelValues("gateway").Inc()
	ObservePermission("global", true)
	SSEConnections.WithLabelValues("realtime").Inc()
	LoginFailuresTotal.WithLabelValues(LoginReasonBadPassword).Inc()
	WebhookDeliveriesTotal.WithLabelValues(WebhookOutcomeDelivered).Inc()

	got := familiesGathered(t)
	for _, name := range []string{
		"orinforum_eventbus_publish_total",
		"orinforum_eventbus_dispatch_duration_seconds",
		"orinforum_eventbus_dispatch_errors_total",
		"orinforum_eventbus_dropped_total",
		"orinforum_kv_operation_duration_seconds",
		"orinforum_ratelimit_rejections_total",
		"orinforum_ratelimit_failopen_total",
	} {
		if !got[name] {
			t.Errorf("family %s missing after observation", name)
		}
	}
}

// TestProducerSeedingExportsRuntimeFamilies pins the contract that makes the
// absent() canary in deploy/prometheus/rules/orinforum.yml work: the producers
// seed zero-valued series at startup, so all three runtime-labelled families
// exist before the first event/operation/rejection — a healthy quiet instance
// must NOT look like broken instrumentation.
//
// The KV family is a histogram and only exists in TSDB as <name>_bucket/_sum/
// _count, so the alert MUST probe the _count series; a zero-observation
// histogram still exports it (asserted below).
func TestProducerSeedingExportsRuntimeFamilies(t *testing.T) {
	// Unique label values: this test asserts a zero-observation series, and the
	// other tests in this process observe the real names.
	SeedEventbusEvent("seed.probe")
	SeedKVOperation("seed_probe_op")
	SeedRatelimitPolicy("seed_probe_policy")

	mfs, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Registry.Gather() error = %v", err)
	}
	byName := make(map[string]*dto.MetricFamily, len(mfs))
	for _, mf := range mfs {
		byName[mf.GetName()] = mf
	}

	for _, name := range []string{
		"orinforum_eventbus_publish_total",
		"orinforum_kv_operation_duration_seconds",
		"orinforum_ratelimit_rejections_total",
		"orinforum_ratelimit_failopen_total",
	} {
		if _, ok := byName[name]; !ok {
			t.Errorf("family %s must exist after producer seeding (absent() would fire)", name)
		}
	}

	kv := byName["orinforum_kv_operation_duration_seconds"]
	if kv == nil {
		t.Fatal("KV histogram family missing after seeding")
	}
	foundZeroCount := false
	for _, m := range kv.GetMetric() {
		if !hasLabelValue(m, "op", "seed_probe_op") {
			continue
		}
		foundZeroCount = m.GetHistogram().GetSampleCount() == 0
		if !foundZeroCount {
			t.Errorf("seeded op=seed_probe_op series should have zero observations, got count=%d", m.GetHistogram().GetSampleCount())
		}
	}
	if !foundZeroCount {
		t.Error("seeded op=seed_probe_op histogram series not found: absent(_count) would still fire")
	}
}

// hasLabelValue reports whether a sample carries name="want".
func hasLabelValue(m *dto.Metric, name, want string) bool {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue() == want
		}
	}
	return false
}

// TestLabelNamesAreBounded pins the label contract behind the cardinality
// guard: high-cardinality values (trace id, user id, raw path segments) must
// never become labels, so any change here is a deliberate API decision.
func TestLabelNamesAreBounded(t *testing.T) {
	mfs, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Registry.Gather() error = %v", err)
	}

	want := map[string][]string{
		"orinforum_eventbus_publish_total":            {"event", "result"},
		"orinforum_kv_operation_duration_seconds":     {"op", "outcome"},
		"orinforum_ratelimit_rejections_total":        {"name"},
		"orinforum_permission_decisions_total":        {"scope", "result"},
		"orinforum_sse_connections":                   {"channel"},
		"orinforum_login_failures_total":              {"reason"},
		"orinforum_webhook_deliveries_total":          {"outcome"},
		"orinforum_http_requests_total":               {"method", "path", "status"},
		"orinforum_eventbus_queue_depth":              {},
		"orinforum_kv_cas_exhausted_total":            {},
		"orinforum_webhook_delivery_duration_seconds": {},
		"orinforum_nats_connected":                    {"client"},
		"orinforum_nats_disconnects_total":            {"client"},
		"orinforum_nats_reconnects_total":             {"client"},
		"orinforum_sse_write_failures_total":          {"channel"},
		"orinforum_webhook_pending_deliveries":        {},
		"orinforum_db_queries_total":                  {},
		"orinforum_db_slow_queries_total":             {},
		"orinforum_permission_cache_entries":          {"cache"},
		"orinforum_backup_jobs_total":                 {"kind", "outcome"},
		"orinforum_backup_job_duration_seconds":       {"kind"},
	}

	byName := make(map[string][]string, len(mfs))
	for _, mf := range mfs {
		var labels []string
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				labels = append(labels, lp.GetName())
			}
			break // one sample carries the family's full label set
		}
		sort.Strings(labels)
		byName[mf.GetName()] = labels
	}

	for name, labels := range want {
		got, ok := byName[name]
		if !ok {
			t.Errorf("family %s not gathered", name)
			continue
		}
		wantSorted := append([]string(nil), labels...)
		sort.Strings(wantSorted)
		if len(got) != len(wantSorted) {
			t.Errorf("%s labels = %v, want %v", name, got, wantSorted)
			continue
		}
		for i, l := range wantSorted {
			if got[i] != l {
				t.Errorf("%s label %d = %q, want %q", name, i, got[i], l)
			}
		}
	}
}
