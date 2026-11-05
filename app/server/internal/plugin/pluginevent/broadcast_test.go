package pluginevent_test

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/plugin"
	"feedchamber/internal/plugin/pluginevent"

	"github.com/stretchr/testify/require"
)

// 适配器必须把 eventbus.Event 拆成 (eventType, payload)，payload 原样透传。
func TestBroadcasterAdaptsTypeAndPayload(t *testing.T) {
	bus := testnats.NewEventBus(t)

	type delivery struct {
		eventType string
		payload   map[string]any
	}
	got := make(chan delivery, 1)
	pluginevent.Broadcaster(bus).SubscribeBroadcast(eventbus.EventPostCreated,
		func(eventType string, payload map[string]any) {
			select {
			case got <- delivery{eventType: eventType, payload: payload}:
			default:
			}
		})

	bus.Publish(eventbus.Event{
		Type:    eventbus.EventPostCreated,
		Payload: map[string]any{"post_id": "p9"},
	})

	select {
	case d := <-got:
		require.Equal(t, eventbus.EventPostCreated, d.eventType)
		require.Equal(t, "p9", d.payload["post_id"])
	case <-time.After(5 * time.Second):
		t.Fatal("广播事件没有送达适配层")
	}
}

// 适配器同时实现 TraceBroadcaster，且 ctx 版本拿到的是 eventbus 重建过的 ctx
// （非 nil）——否则插件钩子日志会丢掉 trace_id。
func TestBroadcasterImplementsTraceBroadcaster(t *testing.T) {
	bus := testnats.NewEventBus(t)
	b := pluginevent.Broadcaster(bus)

	_, ok := b.(plugin.TraceBroadcaster)
	require.True(t, ok, "适配器必须实现 TraceBroadcaster，否则 trace 在接缝处丢失")

	got := make(chan context.Context, 1)
	b.(plugin.TraceBroadcaster).
		SubscribeBroadcastContext(eventbus.EventPostClosed, func(ctx context.Context, _ string, _ map[string]any) {
			select {
			case got <- ctx:
			default:
			}
		})

	bus.Publish(eventbus.Event{Type: eventbus.EventPostClosed, Payload: map[string]any{}})

	select {
	case ctx := <-got:
		require.NotNil(t, ctx, "ctx 版本不得收到 nil context")
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 版本没有收到事件")
	}
}

// nil bus / nil handler 都必须是安全空操作，不能 panic：
// 装配顺序出错时应当表现为「没有事件」，而不是崩溃。
func TestBroadcasterToleratesNilBusAndHandler(t *testing.T) {
	require.NotPanics(t, func() {
		pluginevent.Broadcaster(nil).SubscribeBroadcast("post.created", func(string, map[string]any) {})
	})

	bus := testnats.NewEventBus(t)
	require.NotPanics(t, func() {
		pluginevent.Broadcaster(bus).SubscribeBroadcast("post.created", nil)
	})
}
