package eventbus_test

import (
	"sync"
	"testing"
	"time"

	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/testnats"

	"github.com/stretchr/testify/require"
)

// TestSubscribeBroadcastDeliversToAllHandlers 钉住广播语义：
// 同一事件类型注册多个广播 handler 时，每个都要收到。
func TestSubscribeBroadcastDeliversToAllHandlers(t *testing.T) {
	bus := testnats.NewEventBus(t)
	defer bus.Shutdown()

	var mu sync.Mutex
	got := map[string]int{}
	for _, name := range []string{"a", "b", "c"} {
		name := name
		bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {
			mu.Lock()
			got[name]++
			mu.Unlock()
		})
	}

	bus.Publish(eventbus.Event{Type: eventbus.EventPostCreated, Payload: map[string]any{"post_id": "p1"}})

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got["a"] == 1 && got["b"] == 1 && got["c"] == 1
	}, 3*time.Second, 20*time.Millisecond)
}

// TestSubscribeBroadcastDoesNotChangeQueueSemantics 钉住「不破坏现有语义」：
// 队列订阅仍然是每事件只消费一次。
func TestSubscribeBroadcastDoesNotChangeQueueSemantics(t *testing.T) {
	bus := testnats.NewEventBus(t)
	defer bus.Shutdown()

	var mu sync.Mutex
	count := 0
	bus.Subscribe(eventbus.EventPostCreated, func(e eventbus.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Publish(eventbus.Event{Type: eventbus.EventPostCreated, Payload: map[string]any{}})

	// 等足够久确认没有重复消费。
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 1
	}, 3*time.Second, 20*time.Millisecond)

	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count, "队列订阅必须只消费一次")
}

// TestSubscribeBroadcastDoesNotDoubleDeliverQueueHandlers 钉住「两种订阅互不污染」：
// 同一事件类型同时存在队列订阅与广播订阅时，队列 handler 仍只消费一次，
// 广播 handler 收到一次。这是插件接入后的真实形态（既有 14 个队列消费者
// + 新增插件广播消费者共用事件类型），也是上一条测试单独覆盖不到的场景。
func TestSubscribeBroadcastDoesNotDoubleDeliverQueueHandlers(t *testing.T) {
	bus := testnats.NewEventBus(t)
	defer bus.Shutdown()

	var (
		mu            sync.Mutex
		queueCount    int
		broadcastSeen int
	)
	bus.Subscribe(eventbus.EventPostCreated, func(e eventbus.Event) {
		mu.Lock()
		queueCount++
		mu.Unlock()
	})
	bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {
		mu.Lock()
		broadcastSeen++
		mu.Unlock()
	})

	bus.Publish(eventbus.Event{Type: eventbus.EventPostCreated, Payload: map[string]any{}})

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return queueCount >= 1 && broadcastSeen >= 1
	}, 3*time.Second, 20*time.Millisecond)

	// 等足够久，确认队列 handler 没有被广播路径再跑一遍。
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, queueCount, "队列 handler 不能被广播订阅重复触发")
	require.Equal(t, 1, broadcastSeen, "广播 handler 必须收到一次")
}

// TestSubscribeBroadcastAfterShutdownDoesNotPanic 钉住 Shutdown 后订阅是安全空操作：
// 广播订阅在 Shutdown 里被清理（nc 置 nil），之后注册不能再解引用空连接。
func TestSubscribeBroadcastAfterShutdownDoesNotPanic(t *testing.T) {
	bus := testnats.NewEventBus(t)
	bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {})
	bus.Shutdown()

	require.NotPanics(t, func() {
		bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {})
	})
}
