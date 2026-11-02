package handler

// Handler-level tests for the feedchamber_sse_connections{channel} gauge
// (openspec change 2026-10-13-build-observability, task 9.5).
//
// Both Stream implementations increment the gauge right before entering their
// event loop and `defer` the decrement, so the observable contract is:
//
//	stream established  -> gauge(channel) == 1
//	client goes away    -> gauge(channel) == 0
//
// The gauge is process-global, so every assertion polls for the expected value
// with a timeout instead of sampling once: a connection that is closing still
// shows 1 for a few milliseconds after the client disconnects.

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
	dto "github.com/prometheus/client_model/go"
)

const (
	sseTestUserID    = "u1"
	sseGaugeTimeout  = 2 * time.Second
	sseGaugePoll     = 10 * time.Millisecond
	sseStreamTimeout = 5 * time.Second
)

// newSSETestApp mounts one SSE handler on a fresh Iris app behind a stub auth
// middleware that injects the user the gateway would normally resolve. The Iris
// logger is silenced so stream teardown does not pollute -v output.
func newSSETestApp(t *testing.T, path string, stream func(iris.Context)) *httptest.Server {
	t.Helper()

	app := iris.New()
	app.Logger().SetLevel("disable")
	app.Post(path, func(ctx iris.Context) {
		ctx.Values().Set(pkg.CtxUserID, sseTestUserID)
		ctx.Next()
	}, stream)
	if err := app.Build(); err != nil {
		t.Fatalf("build iris app: %v", err)
	}

	srv := httptest.NewServer(app)
	t.Cleanup(srv.Close)
	return srv
}

// sseGaugeValue reads the current value of the labelled gauge.
//
// This is what prometheus/testutil.ToFloat64 does, inlined because testutil
// pulls in an extra module requirement (kylelemons/godebug) that is not in
// go.mod, and adding it would mean touching the module file for a test-only
// dependency.
func sseGaugeValue(t *testing.T, channel string) float64 {
	t.Helper()

	var m dto.Metric
	if err := metrics.SSEConnections.WithLabelValues(channel).Write(&m); err != nil {
		t.Fatalf("read SSE gauge %q: %v", channel, err)
	}
	return m.GetGauge().GetValue()
}

// waitSSEGauge polls the labelled gauge until it reaches want or times out.
func waitSSEGauge(t *testing.T, channel string, want float64) {
	t.Helper()

	deadline := time.Now().Add(sseGaugeTimeout)
	var got float64
	for {
		got = sseGaugeValue(t, channel)
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("SSE gauge %q = %v, want %v after %s", channel, got, want, sseGaugeTimeout)
		}
		time.Sleep(sseGaugePoll)
	}
}

// sseStream is an open SSE response: the POST has been answered and the body is
// being read incrementally.
type sseStream struct {
	t      *testing.T
	resp   *http.Response
	body   *bufio.Reader
	cancel context.CancelFunc
}

// startSSEStream performs the POST in a goroutine and waits for the handler's
// first flushed byte.
//
// Waiting for headers is the first synchronisation point: net/http only sends
// them once the handler flushes an event, so headers arriving means the handler
// is already inside its loop. Some handlers have nothing to say on connect
// (realtime only writes on a message or the 15s heartbeat), so kick is retried
// every 25ms until the response shows up — a retry loop rather than a fixed
// sleep, because the subscription is registered asynchronously inside the
// handler while the gauge is incremented before it.
func startSSEStream(t *testing.T, srv *httptest.Server, path, body string, kick func()) *sseStream {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		cancel()
		t.Fatalf("build SSE request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	type result struct {
		resp *http.Response
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, doErr := http.DefaultClient.Do(req)
		done <- result{resp: resp, err: doErr}
	}()

	timeout := time.NewTimer(sseStreamTimeout)
	defer timeout.Stop()
	kickTick := time.NewTicker(25 * time.Millisecond)
	defer kickTick.Stop()

	var res result
waitHeaders:
	for {
		select {
		case res = <-done:
			break waitHeaders
		case <-kickTick.C:
			if kick != nil {
				kick()
			}
		case <-timeout.C:
			cancel()
			t.Fatalf("timed out after %s waiting for SSE headers from %s", sseStreamTimeout, path)
		}
	}

	if res.err != nil {
		cancel()
		t.Fatalf("SSE request %s: %v", path, res.err)
	}
	if res.resp.StatusCode != http.StatusOK {
		_ = res.resp.Body.Close()
		cancel()
		t.Fatalf("SSE request %s: status = %d, want %d", path, res.resp.StatusCode, http.StatusOK)
	}

	stream := &sseStream{t: t, resp: res.resp, body: bufio.NewReader(res.resp.Body), cancel: cancel}
	t.Cleanup(stream.close)
	return stream
}

// nextEvent returns the payload of the next `data:` line, skipping ids, event
// names and blank separator lines.
func (s *sseStream) nextEvent() string {
	s.t.Helper()

	type line struct {
		text string
		err  error
	}
	lines := make(chan line, 1)
	go func() {
		for {
			text, err := s.body.ReadString('\n')
			if err != nil {
				lines <- line{err: err}
				return
			}
			text = strings.TrimRight(text, "\r\n")
			if payload, ok := strings.CutPrefix(text, "data: "); ok {
				lines <- line{text: payload}
				return
			}
		}
	}()

	select {
	case got := <-lines:
		if got.err != nil {
			s.t.Fatalf("read SSE event: %v", got.err)
		}
		return got.text
	case <-time.After(sseGaugeTimeout):
		s.t.Fatalf("timed out after %s waiting for an SSE event", sseGaugeTimeout)
		return ""
	}
}

// close disconnects the client, which is what makes the server-side
// request context fire and both handlers return through their deferred Dec.
func (s *sseStream) close() {
	s.cancel()
	_ = s.resp.Body.Close()
}

// stubNotificationService implements just enough of service.NotificationService
// for the stream: CountUnread supplies the initial `data: {"unread":N}` frame.
type stubNotificationService struct {
	unread int64
}

var _ service.NotificationService = (*stubNotificationService)(nil)

func (s *stubNotificationService) Create(context.Context, string, string, string, string, string) error {
	return nil
}

func (s *stubNotificationService) List(context.Context, string, int, int) ([]model.Notification, error) {
	return nil, nil
}

func (s *stubNotificationService) ListFiltered(context.Context, string, int, int, string, string) ([]model.Notification, int64, error) {
	return nil, 0, nil
}

func (s *stubNotificationService) CountByUser(context.Context, string) (int64, error) {
	return s.unread, nil
}

func (s *stubNotificationService) CountUnread(context.Context, string) (int64, error) {
	return s.unread, nil
}

func (s *stubNotificationService) MarkRead(context.Context, string, string) error {
	return nil
}

func (s *stubNotificationService) MarkAllRead(context.Context, string) error {
	return nil
}

func (s *stubNotificationService) Delete(context.Context, string, string) error {
	return nil
}

// TestSSEGaugeNotificationStream pins the notification stream's gauge
// lifecycle: 1 while the client is connected, 0 once it disconnects.
func TestSSEGaugeNotificationStream(t *testing.T) {
	kv := testnats.NewKVStore(t, "handler_sse_notification")
	const unread = int64(7)
	svc := &stubNotificationService{unread: unread}
	h := newNotificationHandler(svc, kv, mock.NewSettingService())
	srv := newSSETestApp(t, "/notifications/stream", h.Stream)

	waitSSEGauge(t, "notification", 0)
	waitSSEGauge(t, "realtime", 0) // other channel must not be touched

	// The handler writes its first frame unprompted, so no kick is needed.
	stream := startSSEStream(t, srv, "/notifications/stream", "{}", nil)
	waitSSEGauge(t, "notification", 1)

	var first struct {
		Unread int64 `json:"unread"`
	}
	if err := json.Unmarshal([]byte(stream.nextEvent()), &first); err != nil {
		t.Fatalf("first notification event is not JSON: %v", err)
	}
	if first.Unread != unread {
		t.Fatalf("first notification event unread = %d, want %d", first.Unread, unread)
	}

	// Reading a frame proves the loop is live; the gauge must still read 1.
	if got := sseGaugeValue(t, "notification"); got != 1 {
		t.Fatalf("SSE gauge %q = %v while streaming, want 1", "notification", got)
	}

	// A published message is forwarded verbatim, so the subscription (and thus
	// the connection whose gauge we are asserting on) is really established.
	if err := kv.Publish(context.Background(), "notif:"+sseTestUserID, []byte(`{"unread":9}`)); err != nil {
		t.Fatalf("publish notification event: %v", err)
	}
	if got := stream.nextEvent(); got != `{"unread":9}` {
		t.Fatalf("forwarded notification event = %q, want %q", got, `{"unread":9}`)
	}
	waitSSEGauge(t, "notification", 1)

	stream.close()
	waitSSEGauge(t, "notification", 0)
}

// TestSSEGaugeRealtimeStream does the same for the generic realtime stream.
func TestSSEGaugeRealtimeStream(t *testing.T) {
	kv := testnats.NewKVStore(t, "handler_sse_realtime")
	h := newRealtimeHandler(kv)
	srv := newSSETestApp(t, "/realtime/stream", h.Stream)

	waitSSEGauge(t, "realtime", 0)
	waitSSEGauge(t, "notification", 0) // other channel must not be touched

	channel := "notif:" + sseTestUserID
	stream := startSSEStream(t, srv, "/realtime/stream", `{"channels":["`+channel+`"]}`, func() {
		if err := kv.Publish(context.Background(), channel, []byte(`{"x":1}`)); err != nil {
			t.Logf("publish realtime kick: %v", err)
		}
	})
	waitSSEGauge(t, "realtime", 1)

	var event struct {
		Type    string          `json:"type"`
		Channel string          `json:"channel"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(stream.nextEvent()), &event); err != nil {
		t.Fatalf("first realtime event is not JSON: %v", err)
	}
	if event.Type != "message" {
		t.Fatalf("first realtime event type = %q, want %q", event.Type, "message")
	}
	if event.Channel != channel {
		t.Fatalf("first realtime event channel = %q, want %q", event.Channel, channel)
	}
	if string(event.Payload) != `{"x":1}` {
		t.Fatalf("first realtime event payload = %s, want %s", event.Payload, `{"x":1}`)
	}

	waitSSEGauge(t, "realtime", 1)

	stream.close()
	waitSSEGauge(t, "realtime", 0)
}
