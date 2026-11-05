package eventbus_test

import (
	"sync/atomic"
	"testing"
	"time"

	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/testnats"

	"github.com/stretchr/testify/require"
)

// TestBroadcastHandlerNotInvokedAfterShutdown 钉住 Shutdown 的广播清理：
// 关闭前注册的广播 handler 在 Shutdown 之后不得再被投递触发。
//
// Shutdown 会退订 broadcastSubs，但保留 broadcastHandlers——这是有意的
// （handler 可能在其它 goroutine 里被读取）。若将来有人把 nc 置 nil 的次序
// 改到退订之前，或让 Publish 在 nc==nil 时仍走本地 fan-out，这条会立刻变红。
func TestBroadcastHandlerNotInvokedAfterShutdown(t *testing.T) {
	bus := testnats.NewEventBus(t)

	var calls atomic.Int32
	bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {
		calls.Add(1)
	})

	bus.Shutdown()

	// Shutdown 后再订阅一次：必须安全空操作，不得 panic。
	require.NotPanics(t, func() {
		bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {
			calls.Add(1)
		})
	})

	// 再发布一次：已关闭的连接不得投递，handler 一次都不该被调用。
	bus.Publish(eventbus.Event{Type: eventbus.EventPostCreated, Payload: map[string]any{"post_id": "p1"}})
	time.Sleep(300 * time.Millisecond)

	require.Equal(t, int32(0), calls.Load(), "Shutdown 之后广播 handler 不得再被调用")
}
