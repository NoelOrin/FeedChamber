package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/pkg/kvstore"
)

// memKV is the slice of kvstore.Store the engine actually uses; the remaining
// verbs error out so an accidental new dependency inside Check shows up as a
// failing test rather than a silent no-op.
type memKV struct {
	data   map[string]int64
	failOn error
}

func newMemKV() *memKV { return &memKV{data: map[string]int64{}} }

func (m *memKV) SetNX(_ context.Context, key, _ string, _ time.Duration) (bool, error) {
	if m.failOn != nil {
		return false, m.failOn
	}
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = 0
	return true, nil
}

func (m *memKV) Incr(_ context.Context, key string) (int64, error) {
	if m.failOn != nil {
		return 0, m.failOn
	}
	m.data[key]++
	return m.data[key], nil
}

func (m *memKV) Get(context.Context, string) (string, error) {
	return "", kvstore.ErrKeyNotFound
}
func (m *memKV) Set(context.Context, string, string, time.Duration) error { return nil }
func (m *memKV) Delete(context.Context, string) error                     { return nil }
func (m *memKV) Exists(context.Context, string) (bool, error)             { return false, nil }
func (m *memKV) PrefixScan(context.Context, string) (map[string]string, error) {
	return nil, errors.New("unused by the limiter")
}
func (m *memKV) DeletePrefix(context.Context, string) error { return nil }
func (m *memKV) IncrBy(context.Context, string, int64) (int64, error) {
	return 0, errors.New("unused by the limiter")
}
func (m *memKV) Publish(context.Context, string, []byte) error { return nil }
func (m *memKV) Subscribe(context.Context, string) (<-chan []byte, error) {
	return nil, errors.New("unused by the limiter")
}
func (m *memKV) Close() error { return nil }

// counterValue 从 metrics.Registry 里读某个计数器在给定标签下的值。
// 走 Gather 而非 prometheus/testutil，是为了不给一个测试引入新的传递依赖。
func counterValue(t *testing.T, family, label, labelValue string) float64 {
	t.Helper()
	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() != family {
			continue
		}
		for _, m := range f.GetMetric() {
			match := false
			for _, lp := range m.GetLabel() {
				if lp.GetName() == label && lp.GetValue() == labelValue {
					match = true
				}
			}
			if !match {
				continue
			}
			if c := m.GetCounter(); c != nil {
				return c.GetValue()
			}
		}
		return 0
	}
	return 0
}

// 存储键形状是契约的一部分：改前缀等于把正在计数的窗口全部清零，
// 上线瞬间所有 IP/端点都获得一个全新配额。
func TestKeyFormatsArePinned(t *testing.T) {
	if got := EndpointKey("1.2.3.4", "login"); got != "rl:1.2.3.4:login" {
		t.Errorf("EndpointKey = %q, want rl:1.2.3.4:login", got)
	}
	if got := GlobalKey("1.2.3.4"); got != "rl:ratelimit:1.2.3.4" {
		t.Errorf("GlobalKey = %q, want rl:ratelimit:1.2.3.4", got)
	}
	if EndpointKey("1.2.3.4", "login") == GlobalKey("1.2.3.4") {
		t.Error("endpoint and global policies must not share a counter")
	}
}

func TestCheckCountsAgainstLimit(t *testing.T) {
	kv := newMemKV()
	l := NewLimiter(kv)
	ctx := context.Background()
	key := EndpointKey("9.9.9.9", "probe")

	for i := 0; i < 2; i++ {
		if got := l.Check(ctx, "probe", key, 2, time.Minute); got != Allowed {
			t.Fatalf("request %d: got %v, want Allowed", i+1, got)
		}
	}
	if got := l.Check(ctx, "probe", key, 2, time.Minute); got != Denied {
		t.Fatalf("3rd request: got %v, want Denied", got)
	}
	if kv.data[key] != 3 {
		t.Errorf("counter = %d, want 3", kv.data[key])
	}
}

func TestCheckFailOpenAndMetrics(t *testing.T) {
	policy := "probe-failopen"
	rej := counterValue(t, "feedchamber_ratelimit_rejections_total", "name", policy)
	fo := counterValue(t, "feedchamber_ratelimit_failopen_total", "name", policy)

	kv := newMemKV()
	kv.failOn = errors.New("kv down")
	got := NewLimiter(kv).Check(context.Background(), policy, GlobalKey("1.1.1.1"), 1, time.Minute)

	if got != FailOpen {
		t.Fatalf("got %v, want FailOpen", got)
	}
	if d := counterValue(t, "feedchamber_ratelimit_failopen_total", "name", policy) - fo; d != 1 {
		t.Errorf("failopen metric delta = %v, want 1", d)
	}
	if d := counterValue(t, "feedchamber_ratelimit_rejections_total", "name", policy) - rej; d != 0 {
		t.Errorf("a fail-open must not count as a rejection, delta = %v", d)
	}
}

func TestCheckDeniedIncrementsRejections(t *testing.T) {
	policy := "probe-denied"
	before := counterValue(t, "feedchamber_ratelimit_rejections_total", "name", policy)
	l := NewLimiter(newMemKV())
	key := EndpointKey("2.2.2.2", policy)
	ctx := context.Background()
	l.Check(ctx, policy, key, 1, time.Minute)
	if got := l.Check(ctx, policy, key, 1, time.Minute); got != Denied {
		t.Fatalf("got %v, want Denied", got)
	}
	if d := counterValue(t, "feedchamber_ratelimit_rejections_total", "name", policy) - before; d != 1 {
		t.Errorf("rejection metric delta = %v, want 1", d)
	}
}

// 未装配 KV 时必须是放行而不是拒绝：内嵌 NATS 关掉或测试环境下限流本就不存在。
func TestCheckWithoutKVAllows(t *testing.T) {
	if got := NewLimiter(nil).Check(context.Background(), "p", GlobalKey("x"), 1, time.Second); got != Allowed {
		t.Fatalf("got %v, want Allowed", got)
	}
	var nilLimiter *Limiter
	if got := nilLimiter.Check(context.Background(), "p", GlobalKey("x"), 1, time.Second); got != Allowed {
		t.Fatalf("nil receiver: got %v, want Allowed", got)
	}
}
