package eventbus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/pkg/tracing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// familyCounterSum sums every child of a counter family straight from the
// registry. Deliberately not prometheus/testutil: that import would rewrite
// go.mod/go.sum, which are already mid-edit in this working tree.
func familyCounterSum(t *testing.T, family string) float64 {
	t.Helper()

	mfs, err := metrics.Registry.Gather()
	require.NoError(t, err)

	var total float64
	for _, mf := range mfs {
		if mf.GetName() != family {
			continue
		}
		for _, m := range mf.GetMetric() {
			total += m.GetCounter().GetValue()
		}
	}
	return total
}

func TestPublishPropagatesTraceToConsumer(t *testing.T) {
	bus := testnats.NewEventBus(t)

	tc := tracing.New()
	received := make(chan context.Context, 1)
	bus.Subscribe(eventbus.EventPostCreated, func(e eventbus.Event) {
		received <- e.Ctx
	})

	bus.Publish(eventbus.Event{
		Type:    eventbus.EventPostCreated,
		Ctx:     tracing.ContextWithTrace(context.Background(), tc),
		Payload: map[string]any{"post_id": "p1"},
	})

	select {
	case rctx := <-received:
		consumerTC, ok := tracing.TraceFromContext(rctx)
		require.True(t, ok, "consumer Event.Ctx must carry a trace")
		// Same trace, different span: the consumer is a child hop.
		assert.Equal(t, tc.TraceID, consumerTC.TraceID)
		assert.NotEqual(t, tc.SpanID, consumerTC.SpanID)
		assert.Equal(t, tc.SpanID, consumerTC.ParentSpanID)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for traced event delivery")
	}
}

func TestPublishWithoutTraceStartsFreshTrace(t *testing.T) {
	bus := testnats.NewEventBus(t)

	received := make(chan context.Context, 1)
	bus.Subscribe(eventbus.EventUserCreated, func(e eventbus.Event) {
		received <- e.Ctx
	})

	// Plain background context: nothing to propagate, but the consumer must
	// still end up with a loggable trace.
	bus.Publish(eventbus.Event{Type: eventbus.EventUserCreated, Ctx: context.Background()})

	select {
	case ctx := <-received:
		tc, ok := tracing.TraceFromContext(ctx)
		assert.True(t, ok, "consumer must get a fresh trace when the message has none")
		assert.True(t, tc.Valid())
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for untraced event delivery")
	}
}

func TestPublishDoesNotLeakCallerContext(t *testing.T) {
	bus := testnats.NewEventBus(t)

	// A cancelled publisher context must not stop delivery: Publish detaches.
	pctx, cancel := context.WithCancel(context.Background())
	received := make(chan eventbus.Event, 1)
	bus.Subscribe(eventbus.EventContentReplied, func(e eventbus.Event) { received <- e })

	bus.Publish(eventbus.Event{
		Type:    eventbus.EventContentReplied,
		Ctx:     pctx,
		Payload: map[string]any{"user_id": "u1"},
	})
	cancel()

	select {
	case e := <-received:
		assert.Equal(t, "u1", e.Payload["user_id"])
		// The consumer ctx must be a fresh background ctx, not the publisher's:
		// had it been reused, Err() would report context.Canceled here and
		// notification work would silently stop.
		require.NoError(t, e.Ctx.Err())
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: cancelling the publisher must not drop the event")
	}
}

func TestHandlerPanicIsCountedAndDoesNotBreakOthers(t *testing.T) {
	bus := testnats.NewEventBus(t)

	before := familyCounterSum(t, "feedchamber_eventbus_dispatch_errors_total")

	var wg sync.WaitGroup
	wg.Add(1)
	bus.Subscribe(eventbus.EventPostClosed, func(e eventbus.Event) {
		panic("boom")
	})
	bus.Subscribe(eventbus.EventPostClosed, func(e eventbus.Event) {
		defer wg.Done()
	})

	bus.Publish(eventbus.Event{Type: eventbus.EventPostClosed, Ctx: context.Background()})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("panic in one handler must not stop the others")
	}

	after := familyCounterSum(t, "feedchamber_eventbus_dispatch_errors_total")
	assert.Greater(t, after, before, "recovered panic must increment the error counter")
}
