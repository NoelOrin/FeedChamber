package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"orinforum/internal/metrics"
	"orinforum/internal/pkg/tracing"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// traceParentHeader carries the W3C trace context across NATS so a consumer's
// log lines can be tied back to the HTTP request that published the event.
const traceParentHeader = "traceparent"

// Event 业务事件。
type Event struct {
	Type    string         // 事件类型，如 "content.replied"
	Payload map[string]any // 事件数据，各类型自定义
	Ctx     context.Context
}

// Handler 事件处理器。
type Handler func(event Event)

// EventBus 基于 NATS core pub/sub 的异步事件总线。
// 同一事件类型使用一个队列订阅，集群内每个事件只会被一个实例消费，
// 消费后仍按本地订阅列表分发给该进程内的所有 Handler。
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	subs     map[string]*nats.Subscription
	ch       chan Event
	wg       sync.WaitGroup
	done     chan struct{}
	once     sync.Once
	nc       *nats.Conn
}

const (
	defaultQueueSize   = 100
	defaultWorkerCnt   = 4
	eventSubjectPrefix = "nats.eventbus."
	eventQueue         = "orinforum_eventbus"
)

type wireEvent struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// New 连接 NATS 并创建 EventBus 实例。
func New(url string) (*EventBus, error) {
	if url == "" {
		return nil, fmt.Errorf("eventbus: NATS URL is required")
	}
	nc, err := nats.Connect(url,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
		nats.Timeout(5*time.Second),
		// See kvstore.NewNATSStore: infinite reconnects mean a partition is
		// invisible unless the state transitions are exported as metrics.
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			metrics.ObserveNATSDisconnected(metrics.NATSClientEventBus)
			logrus.WithError(err).Warn("eventbus: NATS disconnected")
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			metrics.ObserveNATSReconnected(metrics.NATSClientEventBus)
			logrus.Infof("eventbus: NATS reconnected to %s", c.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("eventbus nats connect: %w", err)
	}
	metrics.SeedNATSClient(metrics.NATSClientEventBus)

	eb := &EventBus{
		handlers: make(map[string][]Handler),
		subs:     make(map[string]*nats.Subscription),
		ch:       make(chan Event, defaultQueueSize),
		done:     make(chan struct{}),
		nc:       nc,
	}
	// Seed every known event type's zero-valued series so the metric families
	// exist before the first publish. Seeding here (not in the metrics package)
	// keeps the dependency direction eventbus → metrics.
	for _, et := range AllEventTypes() {
		metrics.SeedEventbusEvent(et)
	}
	for i := 0; i < defaultWorkerCnt; i++ {
		eb.wg.Add(1)
		go eb.worker()
	}
	return eb, nil
}

func (eb *EventBus) dispatch(event Event) {
	eb.mu.RLock()
	handlers := eb.handlers[event.Type]
	eb.mu.RUnlock()
	for _, h := range handlers {
		start := time.Now()
		func() {
			defer func() {
				if r := recover(); r != nil {
					metrics.EventbusDispatchErrorsTotal.WithLabelValues(event.Type).Inc()
					logrus.Errorf("eventbus: handler panic for event %q: %v", event.Type, r)
				}
			}()
			h(event)
		}()
		metrics.EventbusDispatchDuration.WithLabelValues(event.Type).Observe(time.Since(start).Seconds())
	}
}

func (eb *EventBus) worker() {
	defer eb.wg.Done()
	for {
		select {
		case <-eb.done:
			// 关闭后把 channel 中残留的事件处理完再退出
			for {
				select {
				case event := <-eb.ch:
					eb.dispatch(event)
					metrics.EventbusQueueDepth.Set(float64(len(eb.ch)))
				default:
					return
				}
			}
		case event := <-eb.ch:
			eb.dispatch(event)
			metrics.EventbusQueueDepth.Set(float64(len(eb.ch)))
		}
	}
}

func (eb *EventBus) handleNATSMessage(msg *nats.Msg) {
	var wire wireEvent
	if err := json.Unmarshal(msg.Data, &wire); err != nil {
		logrus.Errorf("eventbus: decode event %q: %v", msg.Subject, err)
		return
	}
	eb.enqueue(Event{
		Type:    wire.Type,
		Payload: wire.Payload,
		// Detached on purpose: the publishing request may already be gone.
		// Only the trace travels, never the caller's cancellation.
		Ctx: consumerContext(msg.Header),
	})
}

// consumerContext rebuilds the publisher's trace (when the message carries one)
// onto a background context so handler logs stay correlated without inheriting
// the publisher's deadline or cancellation. Messages from publishers that do not
// carry a trace start a fresh one, so every consumed event is loggable.
func consumerContext(header nats.Header) context.Context {
	tc, ok := tracing.ParseTraceparent(header.Get(traceParentHeader))
	if !ok {
		tc = tracing.New()
	} else {
		tc = tc.Child()
	}
	return tracing.ContextWithTrace(context.Background(), tc)
}

func (eb *EventBus) enqueue(event Event) {
	select {
	case <-eb.done:
	case eb.ch <- event:
		metrics.EventbusQueueDepth.Set(float64(len(eb.ch)))
	default:
		metrics.EventbusDroppedTotal.WithLabelValues(event.Type).Inc()
	}
}

// Subscribe 订阅指定类型的事件。
func (eb *EventBus) Subscribe(eventType string, handler Handler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if eb.nc == nil {
		logrus.Errorf("eventbus: subscribe %q after shutdown", eventType)
		return
	}
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)

	if len(eb.handlers[eventType]) != 1 || eb.subs[eventType] != nil {
		return
	}
	subject := eventSubjectPrefix + eventType
	sub, err := eb.nc.QueueSubscribe(subject, eventQueue, func(msg *nats.Msg) {
		eb.handleNATSMessage(msg)
	})
	if err != nil {
		logrus.Errorf("eventbus: subscribe %q: %v", subject, err)
		return
	}
	eb.subs[eventType] = sub
	_ = eb.nc.Flush()
}

// Publish 异步发布事件，通过 NATS 分发到集群中的事件消费者。
// 调用方的 Ctx 不会随事件传递（它可能在请求结束后被取消，导致通知静默失败）：
// 这里只取其中的 trace 上下文写入消息头，接收方据此在独立 background context
// 上重建同一 trace。
func (eb *EventBus) Publish(event Event) {
	tc, hasTrace := tracing.TraceFromContext(event.Ctx)
	event.Ctx = context.Background()
	eb.mu.RLock()
	nc := eb.nc
	eb.mu.RUnlock()
	if nc == nil {
		metrics.EventbusPublishTotal.WithLabelValues(event.Type, "closed").Inc()
		logrus.Errorf("eventbus: publish event %q after shutdown", event.Type)
		return
	}
	data, err := json.Marshal(wireEvent{Type: event.Type, Payload: event.Payload})
	if err != nil {
		metrics.EventbusPublishTotal.WithLabelValues(event.Type, "encode_error").Inc()
		logrus.Errorf("eventbus: encode event %q: %v", event.Type, err)
		return
	}

	msg := &nats.Msg{Subject: eventSubjectPrefix + event.Type, Data: data}
	if hasTrace {
		msg.Header = nats.Header{}
		msg.Header.Set(traceParentHeader, tc.Traceparent())
	}
	if err := nc.PublishMsg(msg); err != nil {
		metrics.EventbusPublishTotal.WithLabelValues(event.Type, "nats_error").Inc()
		logrus.Errorf("eventbus: publish event %q: %v", event.Type, err)
		return
	}
	metrics.EventbusPublishTotal.WithLabelValues(event.Type, "ok").Inc()
}

// Shutdown 优雅关闭，等待所有已入队事件处理完毕。
func (eb *EventBus) Shutdown() {
	eb.once.Do(func() {
		close(eb.done)
		eb.mu.Lock()
		for _, sub := range eb.subs {
			if sub != nil {
				_ = sub.Unsubscribe()
			}
		}
		eb.subs = make(map[string]*nats.Subscription)
		eb.mu.Unlock()
	})
	eb.wg.Wait()
	eb.mu.Lock()
	nc := eb.nc
	eb.nc = nil
	eb.mu.Unlock()
	if nc != nil {
		nc.Close()
	}
}

// 事件常量见 events.go（本包唯一权威来源，禁止在此重复定义）
