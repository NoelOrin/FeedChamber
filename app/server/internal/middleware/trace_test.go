package middleware

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"orinforum/internal/pkg/logger"
	"orinforum/internal/pkg/tracing"

	"github.com/kataras/iris/v12"
)

// captureLog redirects both the logger package and the standard logrus into a
// temp file so assertions can read real emitted fields.
func captureLog(t *testing.T) (path string, restore func()) {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "orin-log-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	path = f.Name()
	_ = f.Close()

	lg := logger.New(logger.Config{Level: "debug", Format: "json", Output: path})
	previous := logger.Global()
	logger.SetGlobal(lg)
	return path, func() { logger.SetGlobal(previous) }
}

func readLogLines(t *testing.T, path string) []map[string]any {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log %s: %v", path, err)
	}
	defer f.Close()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(text), &entry); err != nil {
			t.Fatalf("log line is not json: %q (%v)", text, err)
		}
		lines = append(lines, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan log: %v", err)
	}
	return lines
}

func newTestApp() *iris.Application {
	app := iris.New()
	app.Use(Trace(true))
	app.Use(Logger())
	app.Get("/ping", func(ctx iris.Context) {
		// Stands in for a Service/Repository layer call: it only has a
		// context.Context, and must still log correlated with the request.
		logger.FromContext(ctx.Request().Context()).Info("inner layer")
		ctx.StatusCode(http.StatusOK)
		_, _ = ctx.WriteString("pong")
	})
	return app
}

// TestTraceEstablishedBeforeLogger is the regression test for the ordering trap:
// app-level middleware runs before per-party ones, so if Trace were placed after
// Logger the completion line would lose its trace id.
func TestTraceEstablishedBeforeLogger(t *testing.T) {
	path, restore := captureLog(t)
	defer restore()

	app := newTestApp()
	if err := app.Build(); err != nil {
		t.Fatalf("app.Build: %v", err)
	}
	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	headerID := resp.Header.Get("X-Request-ID")
	if len(headerID) != 32 {
		t.Fatalf("X-Request-ID = %q, want 32 hex chars", headerID)
	}

	lines := readLogLines(t, path)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d", len(lines))
	}

	var completion, inner map[string]any
	for _, l := range lines {
		switch l["msg"] {
		case "request completed":
			completion = l
		case "inner layer":
			inner = l
		}
	}
	if completion == nil {
		t.Fatal(`no "request completed" log line`)
	}
	if inner == nil {
		t.Fatal(`no "inner layer" log line`)
	}

	// The whole point of the change: correlation survives Handler -> Service.
	if got := completion["trace_id"]; got != headerID {
		t.Errorf("completion trace_id = %v, want %s", got, headerID)
	}
	if got := inner["trace_id"]; got != headerID {
		t.Errorf("service-layer trace_id = %v, want %s", got, headerID)
	}
	if completion["span_id"] != inner["span_id"] {
		t.Errorf("span mismatch: completion=%v inner=%v", completion["span_id"], inner["span_id"])
	}
	for _, key := range []string{"method", "path", "status", "latency_ms", "ip"} {
		if _, ok := completion[key]; !ok {
			t.Errorf("completion log missing field %q", key)
		}
	}
	// The dead request_id field must be gone: one id per request.
	if _, ok := completion["request_id"]; ok {
		t.Error("completion log still carries request_id; it must be trace_id only")
	}
}

func TestTraceAdoptsUpstreamRequestID(t *testing.T) {
	path, restore := captureLog(t)
	defer restore()

	app := newTestApp()
	if err := app.Build(); err != nil {
		t.Fatalf("app.Build: %v", err)
	}
	srv := httptest.NewServer(app)
	defer srv.Close()

	inbound := "0123456789abcdef0123456789abcdef"
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/ping", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Request-ID", inbound)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Request-ID"); got != inbound {
		t.Errorf("X-Request-ID = %q, want echoed %q", got, inbound)
	}
	tp := resp.Header.Get("traceparent")
	if !strings.HasPrefix(tp, "00-"+inbound+"-") {
		t.Errorf("traceparent = %q, want prefix 00-%s-", tp, inbound)
	}
	for _, l := range readLogLines(t, path) {
		if l["msg"] == "request completed" && l["trace_id"] != inbound {
			t.Errorf("completion trace_id = %v, want %s", l["trace_id"], inbound)
		}
	}
}

func TestTraceIgnoresInboundWhenDisabled(t *testing.T) {
	app := iris.New()
	app.Use(Trace(false))
	app.Get("/ping", func(ctx iris.Context) {
		tc, ok := RequestTrace(ctx)
		if !ok {
			t.Error("RequestTrace must still yield a trace when tracing is disabled")
		}
		ctx.JSON(iris.Map{"trace_id": tc.TraceIDHex()})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("app.Build: %v", err)
	}
	srv := httptest.NewServer(app)
	defer srv.Close()

	inbound := tracing.New()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/ping", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("traceparent", inbound.Traceparent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("traceparent"); got != "" {
		t.Errorf("traceparent = %q, want absent when tracing disabled", got)
	}
	if got := resp.Header.Get("X-Request-ID"); len(got) != 32 {
		t.Errorf("X-Request-ID = %q, want a generated 32-hex id", got)
	}
	if resp.Header.Get("X-Request-ID") == inbound.TraceIDHex() {
		t.Error("inbound traceparent must not be adopted when tracing disabled")
	}
}
