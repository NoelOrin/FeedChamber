package testnats

import (
	"testing"

	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/pkg/natsembedded"
)

// Start 启动一个嵌入式 NATS 测试服务并返回客户端 URL。
func Start(t testing.TB) string {
	t.Helper()
	ns, err := natsembedded.Start()
	if err != nil {
		t.Fatalf("start test NATS server: %v", err)
	}
	t.Cleanup(ns.Shutdown)
	return ns.ClientURL()
}

// NewKVStore 创建指向嵌入式 NATS 的测试 KV Store。
func NewKVStore(t testing.TB, bucket string) kvstore.Store {
	t.Helper()
	store, err := kvstore.New(kvstore.Config{
		NATSURL:    Start(t),
		NATSBucket: bucket,
	})
	if err != nil {
		t.Fatalf("create test KV store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// NewEventBus 创建指向嵌入式 NATS 的测试 EventBus。
func NewEventBus(t testing.TB) *eventbus.EventBus {
	t.Helper()
	bus, err := eventbus.New(Start(t))
	if err != nil {
		t.Fatalf("create test event bus: %v", err)
	}
	t.Cleanup(bus.Shutdown)
	return bus
}
