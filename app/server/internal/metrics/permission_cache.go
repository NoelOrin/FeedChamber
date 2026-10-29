package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── Permission cache metrics ───────────────────────────────

var (
	// PermissionCacheEntries reports how many entries each permission cache
	// holds. Three caches exist (role, space, group) and all three are plain
	// in-memory maps that grow with the permission configuration; nothing else
	// exposes their size, so an accidental explosion (a bad sync fanning out
	// space permissions, say) was previously only observable by reading memory.
	PermissionCacheEntries = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "orinforum_permission_cache_entries",
			Help: "Entries held in each permission cache.",
		},
		[]string{"cache"},
	)
)

// Permission cache names. Static: there are exactly three.
const (
	PermissionCacheRole  = "role"
	PermissionCacheSpace = "space"
	PermissionCacheGroup = "group"
)

func init() {
	Registry.MustRegister(PermissionCacheEntries)
	for _, cache := range []string{PermissionCacheRole, PermissionCacheSpace, PermissionCacheGroup} {
		PermissionCacheEntries.WithLabelValues(cache)
	}
}

// ObservePermissionCacheSize sets the size of one cache. Called from the single
// point that rebuilds it (LoadCache and the targeted reloads), so the gauge
// always reflects a completed rebuild rather than a partially populated map.
func ObservePermissionCacheSize(cache string, entries int) {
	PermissionCacheEntries.WithLabelValues(cache).Set(float64(entries))
}
