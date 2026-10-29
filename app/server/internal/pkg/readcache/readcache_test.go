package readcache

import (
	"context"
	"testing"
	"time"

	"orinforum/internal/pkg/testnats"
)

type entry struct {
	Items []string `json:"items"`
	Total int      `json:"total"`
}

// nil Cache 必须是安全值：调用方不需要在每个读取点判空。
func TestNilCacheIsSafe(t *testing.T) {
	var c *Cache
	ctx := context.Background()
	var dst entry
	if c.Get(ctx, "test", "any", &dst) {
		t.Fatal("nil cache must report a miss")
	}
	c.Put(ctx, "any", entry{Total: 1})
	c.InvalidatePrefix(ctx, "any")
}

func TestRoundTripAndExpiry(t *testing.T) {
	kv := testnats.NewKVStore(t, "readcache_rt")
	c := New(kv, time.Second)
	ctx := context.Background()
	key := Key("cache:rt:", 1, "a", (*string)(nil))

	c.Put(ctx, key, entry{Items: []string{"x"}, Total: 1})

	var got entry
	if !c.Get(ctx, "rt", key, &got) {
		t.Fatal("want hit for a fresh key")
	}
	if got.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	time.Sleep(1200 * time.Millisecond)
	if c.Get(ctx, "rt", key, &got) {
		t.Fatal("want miss after ttl")
	}
}

// 同一组参数必须得到同一个 key，任何一个参数变化都必须换 key——
// 否则过滤条件不同的列表会互相串数据。
func TestKeyStability(t *testing.T) {
	a := Key("p:", 1, "x", true)
	b := Key("p:", 1, "x", true)
	c := Key("p:", 1, "x", false)
	d := Key("p:", 2, "x", true)
	if a != b {
		t.Fatalf("key must be stable: %s != %s", a, b)
	}
	if a == c || a == d {
		t.Fatal("key must differ when a filter differs")
	}
	if len(a) > 64 {
		t.Fatalf("key too long for KV: %d", len(a))
	}
}

func TestInvalidatePrefixOnlyDropsMatching(t *testing.T) {
	kv := testnats.NewKVStore(t, "readcache_inv")
	c := New(kv, time.Minute)
	ctx := context.Background()

	listKey := Key("cache:posts:list:", 1)
	tagKey := Key("cache:tags:list:", 1)
	c.Put(ctx, listKey, entry{Total: 1})
	c.Put(ctx, tagKey, entry{Total: 2})

	c.InvalidatePrefix(ctx, "cache:posts:list:")

	var got entry
	if c.Get(ctx, "posts", listKey, &got) {
		t.Fatal("invalidated prefix must not be served")
	}
	if !c.Get(ctx, "tags", tagKey, &got) || got.Total != 2 {
		t.Fatal("unrelated prefix must survive invalidation")
	}
}

// 反序列化失败（例如结构变更后读到旧值）必须按 miss 处理，而不是 panic。
func TestCorruptValueIsMiss(t *testing.T) {
	kv := testnats.NewKVStore(t, "readcache_corrupt")
	c := New(kv, time.Minute)
	ctx := context.Background()
	key := Key("cache:bad:", 1)
	if err := kv.Set(ctx, key, "{not json", time.Minute); err != nil {
		t.Fatalf("seed corrupt value: %v", err)
	}
	var got entry
	if c.Get(ctx, "bad", key, &got) {
		t.Fatal("corrupt payload must be reported as a miss")
	}
}
