package eventbus_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"orinforum/internal/pkg/eventbus"
	"orinforum/internal/pkg/testnats"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishDeliversToSubscriber(t *testing.T) {
	bus := testnats.NewEventBus(t)

	received := make(chan eventbus.Event, 1)
	bus.Subscribe(eventbus.EventContentReplied, func(e eventbus.Event) {
		received <- e
	})

	bus.Publish(eventbus.Event{
		Type:    eventbus.EventContentReplied,
		Ctx:     context.Background(),
		Payload: map[string]any{"user_id": "u1", "sender_name": "Alice"},
	})

	select {
	case e := <-received:
		assert.Equal(t, "u1", e.Payload["user_id"])
		assert.Equal(t, "Alice", e.Payload["sender_name"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event delivery")
	}
}

func TestPublishMultipleSubscribers(t *testing.T) {
	bus := testnats.NewEventBus(t)

	var (
		mu    sync.Mutex
		count int
		done  = make(chan struct{})
	)

	handler := func(e eventbus.Event) {
		mu.Lock()
		count++
		if count == 2 {
			close(done)
		}
		mu.Unlock()
	}

	bus.Subscribe(eventbus.EventContentLiked, handler)
	bus.Subscribe(eventbus.EventContentLiked, handler)

	bus.Publish(eventbus.Event{Type: eventbus.EventContentLiked, Ctx: context.Background()})

	select {
	case <-done:
		mu.Lock()
		assert.Equal(t, 2, count)
		mu.Unlock()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for both subscribers")
	}
}

func TestPublishNoSubscriber(t *testing.T) {
	bus := testnats.NewEventBus(t)

	// 不订阅，发布不应 panic
	bus.Publish(eventbus.Event{Type: "unknown.event", Ctx: context.Background()})
}

func TestPublishDoesNotDeliverToOtherTypes(t *testing.T) {
	bus := testnats.NewEventBus(t)

	called := make(chan struct{}, 1)
	bus.Subscribe(eventbus.EventContentLiked, func(e eventbus.Event) {
		called <- struct{}{}
	})

	bus.Publish(eventbus.Event{Type: eventbus.EventUserFollowed, Ctx: context.Background()})

	select {
	case <-called:
		t.Fatal("should not have received EventUserFollowed")
	case <-time.After(200 * time.Millisecond):
		// expected — no delivery
	}
}

func TestHandlerPanicDoesNotKillWorker(t *testing.T) {
	bus := testnats.NewEventBus(t)

	var panicCount atomic.Int32
	healthy := make(chan eventbus.Event, 1)

	bus.Subscribe(eventbus.EventContentReplied, func(e eventbus.Event) {
		if panicCount.Add(1) == 1 {
			panic("boom")
		}
		healthy <- e
	})

	bus.Publish(eventbus.Event{Type: eventbus.EventContentReplied, Ctx: context.Background()})
	require.Eventually(t, func() bool {
		return panicCount.Load() == 1
	}, time.Second, 10*time.Millisecond)

	bus.Publish(eventbus.Event{
		Type:    eventbus.EventContentReplied,
		Ctx:     context.Background(),
		Payload: map[string]any{"after_panic": true},
	})

	select {
	case e := <-healthy:
		assert.Equal(t, true, e.Payload["after_panic"])
		assert.Equal(t, int32(2), panicCount.Load())
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not recover from panic")
	}
}
