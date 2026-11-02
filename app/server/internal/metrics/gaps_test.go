package metrics

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
)

// unlabelledCounter reads the single sample of a family with no labels.
func unlabelledCounter(t *testing.T, mf *dto.MetricFamily) float64 {
	t.Helper()
	if mf == nil || len(mf.GetMetric()) == 0 {
		return 0
	}
	return mf.GetMetric()[0].GetCounter().GetValue()
}

// TestDBCountersWiredToGormLogger pins the cross-package wiring: db exposes a
// hook (because metrics already imports db, so the reverse would be an import
// cycle) and metrics fills it. If the hook is ever dropped, these counters
// silently stop moving while every other test still passes.
func TestDBCountersWiredToGormLogger(t *testing.T) {
	before := gatherByName(t)
	q0 := unlabelledCounter(t, before["feedchamber_db_queries_total"])
	s0 := unlabelledCounter(t, before["feedchamber_db_slow_queries_total"])

	ObserveDBQuery(false)
	ObserveDBQuery(true)

	after := gatherByName(t)
	q1 := unlabelledCounter(t, after["feedchamber_db_queries_total"])
	s1 := unlabelledCounter(t, after["feedchamber_db_slow_queries_total"])

	if q1 != q0+2 {
		t.Errorf("db_queries_total = %v, want %v", q1, q0+2)
	}
	if s1 != s0+1 {
		t.Errorf("db_slow_queries_total = %v, want %v", s1, s0+1)
	}
}

// TestPermissionCacheGaugesAreSeeded pins that all three caches export a series
// before the first LoadCache, so the dashboard is not blank on a fresh instance.
func TestPermissionCacheGaugesAreSeeded(t *testing.T) {
	mf := gatherByName(t)["feedchamber_permission_cache_entries"]
	if mf == nil {
		t.Fatal("feedchamber_permission_cache_entries missing before first load")
	}
	for _, cache := range []string{PermissionCacheRole, PermissionCacheSpace, PermissionCacheGroup} {
		if _, ok := gaugeValue(t, mf, "cache", cache); !ok {
			t.Errorf("cache=%s series missing", cache)
		}
	}
}

// TestPermissionCacheSizeIsSettable pins that the gauge reflects the last
// rebuild rather than accumulating.
func TestPermissionCacheSizeIsSettable(t *testing.T) {
	ObservePermissionCacheSize(PermissionCacheRole, 7)
	if v, _ := gaugeValue(t, gatherByName(t)["feedchamber_permission_cache_entries"], "cache", PermissionCacheRole); v != 7 {
		t.Errorf("entries = %v, want 7", v)
	}
	// A rebuild that shrinks the cache must shrink the gauge: it is a size, not
	// a high-water mark.
	ObservePermissionCacheSize(PermissionCacheRole, 3)
	if v, _ := gaugeValue(t, gatherByName(t)["feedchamber_permission_cache_entries"], "cache", PermissionCacheRole); v != 3 {
		t.Errorf("entries after shrink = %v, want 3", v)
	}
}

// TestBackupOutcomesAreSeeded pins that every kind/outcome pair exists before
// the first backup, so a "no backups yet" instance is distinguishable from
// broken wiring.
func TestBackupOutcomesAreSeeded(t *testing.T) {
	byName := gatherByName(t)
	jobs := byName["feedchamber_backup_jobs_total"]
	if jobs == nil {
		t.Fatal("feedchamber_backup_jobs_total missing before first backup")
	}
	for _, kind := range []string{BackupKindCreate, BackupKindRestore, BackupKindCleanup} {
		for _, outcome := range []string{BackupOutcomeSucceeded, BackupOutcomeFailed} {
			found := false
			for _, m := range jobs.GetMetric() {
				if hasLabelValue(m, "kind", kind) && hasLabelValue(m, "outcome", outcome) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("kind=%s outcome=%s series missing", kind, outcome)
			}
		}
	}
	if byName["feedchamber_backup_job_duration_seconds"] == nil {
		t.Error("feedchamber_backup_job_duration_seconds missing before first backup")
	}
}
