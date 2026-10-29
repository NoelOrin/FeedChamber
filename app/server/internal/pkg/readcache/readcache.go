// Package readcache 是「读多写少」数据的 JSON 读穿缓存，落在 NATS JetStream KV 上。
//
// 为什么是 KV 而不是进程内 map：KV 已经承载会话、限流、设置这些共享状态，
// 多实例部署时缓存天然共享；代价是一次 NATS 往返（同机 <1ms），远低于一次
// PostgreSQL 查询（压测机实测热点表 5 行也要 7~40ms）。
//
// 语义是**尽力而为**：KV 不可用、反序列化失败、nil Cache 一律按 miss 处理，
// 调用方永远能回落到数据库。缓存只加速，不参与正确性判断。
package readcache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/pkg/kvstore"
)

// Cache 绑定一个 KV store 与一个 TTL。
type Cache struct {
	kv  kvstore.Store
	ttl time.Duration
}

// New 构造缓存。kv 为空或 ttl<=0 时返回 nil，nil Cache 的方法全部安全
// （Get 恒 miss、Put/Invalidate 是空操作），调用方不需要到处判空。
func New(kv kvstore.Store, ttl time.Duration) *Cache {
	if kv == nil || ttl <= 0 {
		return nil
	}
	// KV 的 per-key TTL 落库精度是**秒**（见 kvstore.setTTLMarker），
	// 亚秒级 TTL 会被写成"当前秒"从而立即过期，这里统一向上取整到秒。
	if ttl < time.Second {
		ttl = time.Second
	}
	return &Cache{kv: kv, ttl: ttl}
}

// Get 命中时把 JSON 反序列化进 dst 并返回 true。
//
// name 只用于指标标签（post_list / tag_list / space_tree…），必须来自固定
// 枚举，不能拼进用户输入——否则会撑爆 Prometheus 的标签基数。
func (c *Cache) Get(ctx context.Context, name, key string, dst any) bool {
	if c == nil {
		return false
	}
	raw, err := c.kv.Get(ctx, key)
	if err != nil {
		metrics.ReadCacheMisses.WithLabelValues(name).Inc()
		return false
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		// 结构变更后的旧值：当作 miss，下一次 Put 会覆盖掉
		metrics.ReadCacheMisses.WithLabelValues(name).Inc()
		return false
	}
	metrics.ReadCacheHits.WithLabelValues(name).Inc()
	return true
}

// Put 写入缓存；序列化失败或 KV 写入失败都静默忽略（读路径不能因为缓存写失败而报错）。
func (c *Cache) Put(ctx context.Context, key string, value any) {
	if c == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = c.kv.Set(ctx, key, string(raw), c.ttl)
}

// InvalidatePrefix 删除一类缓存，供写路径在数据变更后调用。
// 前缀必须与 Key 的 prefix 完全一致，否则会留下永远读不到的孤儿键。
func (c *Cache) InvalidatePrefix(ctx context.Context, prefix string) {
	if c == nil {
		return
	}
	_ = c.kv.DeletePrefix(ctx, prefix)
}

// Key 把任意参数序列化后取哈希，拼出稳定、定长、KV 合法的 key。
//
// 直接拼接参数会把用户输入（tag、作者名、搜索词）带进 key：JetStream 对
// key 字符集有限制，且长度无上限，哈希是唯一稳妥的做法。参数顺序即语义顺序，
// 新增过滤条件时**追加**到末尾，不要插到中间（会让既有键全部失效，属预期内的
// 一次性冷启动，但别无意中打散）。
func Key(prefix string, parts ...any) string {
	h := sha256.New()
	var length [8]byte
	for _, part := range parts {
		raw, err := json.Marshal(part)
		if err != nil {
			// 无法序列化的类型不应进入 key；用空值占位保证位置稳定
			raw = []byte("null")
		}
		binary.BigEndian.PutUint64(length[:], uint64(len(raw)))
		h.Write(length[:])
		h.Write(raw)
	}
	return prefix + hex.EncodeToString(h.Sum(nil))[:32]
}
