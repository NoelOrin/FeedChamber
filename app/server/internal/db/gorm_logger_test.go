package db

import (
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
)

// TestSlowQueryIsClassified pins the two halves of the DB instrumentation: the
// observer hook actually receives every statement, and a genuinely slow one is
// classified as slow.
//
// The second half matters because the threshold is a duration comparison that
// is easy to invert or to compare against the wrong unit; a test that only
// checked "hook called" would pass on a logger that never flags anything.
func TestSlowQueryIsClassified(t *testing.T) {
	store, err := New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "slow.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var seen []bool
	SetQueryObserver(func(slow bool) { seen = append(seen, slow) })
	t.Cleanup(func() { SetQueryObserver(nil) })

	// Fast query first: it must be observed and must NOT be flagged slow.
	if err := store.SQL().DB().Raw("SELECT 1").Scan(new(int)).Error; err != nil {
		t.Fatalf("fast query: %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("observer called %d times for one statement, want 1", len(seen))
	}
	if seen[0] {
		t.Error("trivial SELECT 1 classified as slow")
	}

	// Recursive CTE: ~3M iterations, comfortably past the 200ms threshold.
	start := time.Now()
	if err := store.SQL().DB().Raw(
		`WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x < 3000000) SELECT count(*) FROM c`,
	).Scan(new(int)).Error; err != nil {
		t.Fatalf("slow query: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < slowQueryThreshold {
		t.Skipf("query completed in %v, below the %v threshold on this machine; classification not exercised", elapsed, slowQueryThreshold)
	}
	if len(seen) < 2 {
		t.Fatalf("observer called %d times, want 2", len(seen))
	}
	if !seen[len(seen)-1] {
		t.Errorf("query taking %v not classified as slow", elapsed)
	}
}
