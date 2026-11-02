package metrics

import "github.com/prometheus/client_golang/prometheus"

// ── Read-through cache metrics ─────────────────────────────
//
// 只有命中率能回答「缓存到底有没有把数据库从热路径上摘掉」；命中率下降
// 通常意味着 key 里混进了每次都变的参数（时间戳、随机数、用户输入顺序）。
var (
	// ReadCacheHits 命中次数，按缓存名（固定枚举）区分。
	ReadCacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_readcache_hits_total",
			Help: "Read-through cache hits, labelled by cache name.",
		},
		[]string{"name"},
	)

	// ReadCacheMisses 未命中次数（含 KV 故障与反序列化失败）。
	ReadCacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedchamber_readcache_misses_total",
			Help: "Read-through cache misses, labelled by cache name.",
		},
		[]string{"name"},
	)
)

// ReadCacheNames 是缓存名的**固定枚举**：标签基数由它兜底，任何地方都不
// 允许把用户输入当 name 传进来。新增缓存时在这里加一行，并在 init 里 seed。
var ReadCacheNames = []string{"post_list", "tag_list", "space_tree", "space_flat"}

func init() {
	Registry.MustRegister(ReadCacheHits, ReadCacheMisses)
	// 零值 seed：没有流量时族也存在，抓取端不会拿到 "no data"
	for _, name := range ReadCacheNames {
		ReadCacheHits.WithLabelValues(name)
		ReadCacheMisses.WithLabelValues(name)
	}
}
