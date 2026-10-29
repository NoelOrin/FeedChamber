// Package tracing implements request-level trace correlation using the W3C
// Trace Context header format, without depending on a tracing backend.
package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"strings"
)

const (
	TraceIDLen = 16
	SpanIDLen  = 8

	traceparentVersion = "00"
	flagSampled        = byte(0x01)
)

// TraceID is a 128-bit trace identifier. All-zero is reserved as invalid.
type TraceID [TraceIDLen]byte

// SpanID is a 64-bit span identifier. All-zero is reserved as invalid.
type SpanID [SpanIDLen]byte

// Context is the trace state carried along a request.
type Context struct {
	TraceID      TraceID
	SpanID       SpanID
	ParentSpanID SpanID
	Flags        byte

	// Name is the matched route template (e.g. "/api/v1/posts/{id}"), filled in
	// by the gateway once routing has resolved it. It is log-only: never a
	// Prometheus label, since route templates are already bounded but the
	// middleware path label does the same job.
	Name string
}

type ctxKey struct{}

// New returns a Context with a fresh random trace id and span id.
func New() Context {
	return Context{TraceID: newTraceID(), SpanID: newSpanID(), Flags: flagSampled}
}

// FromTraceID adopts an externally supplied trace id (for example nginx's
// $request_id, which has the same 32-lowercase-hex shape) and mints a new span
// under it. Invalid ids yield ok=false so callers can fall back to New().
func FromTraceID(id TraceID) (Context, bool) {
	if id == (TraceID{}) {
		return Context{}, false
	}
	return Context{TraceID: id, SpanID: newSpanID(), Flags: flagSampled}, true
}

// Child keeps the trace id, mints a new span id and records the previous one
// as parent.
func (c Context) Child() Context {
	return Context{
		TraceID:      c.TraceID,
		SpanID:       newSpanID(),
		ParentSpanID: c.SpanID,
		Flags:        c.Flags,
	}
}

// Valid reports whether both identifiers are present and non-zero.
func (c Context) Valid() bool {
	return c.TraceID != (TraceID{}) && c.SpanID != (SpanID{})
}

// TraceIDHex returns the 32-character lowercase hex form of the trace id, which
// is also what this project writes to the X-Request-ID header.
func (c Context) TraceIDHex() string { return hex.EncodeToString(c.TraceID[:]) }

// SpanIDHex returns the 16-character lowercase hex form of the span id.
func (c Context) SpanIDHex() string { return hex.EncodeToString(c.SpanID[:]) }

// Traceparent renders the value for the traceparent header.
func (c Context) Traceparent() string {
	return traceparentVersion + "-" + c.TraceIDHex() + "-" + c.SpanIDHex() + "-" + hex.EncodeToString([]byte{c.Flags})
}

// ParseTraceparent parses "00-<32hex>-<16hex>-<2hex>".
//
// Only version 00 is accepted and hex must be lowercase, as required by W3C
// Trace Context (encoding/hex would otherwise also accept uppercase). Anything
// else reports false so callers generate a new trace instead of propagating a
// value they cannot trust.
func ParseTraceparent(value string) (Context, bool) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 4 || parts[0] != traceparentVersion {
		return Context{}, false
	}
	traceID, ok := parseTraceID(parts[1])
	if !ok {
		return Context{}, false
	}
	spanID, ok := parseSpanID(parts[2])
	if !ok {
		return Context{}, false
	}
	flags, ok := parseFlags(parts[3])
	if !ok {
		return Context{}, false
	}
	return Context{TraceID: traceID, SpanID: spanID, Flags: flags}, true
}

// ParseTraceIDHex accepts a bare 32-character lowercase hex trace id. It backs
// the X-Request-ID fallback: nginx's built-in $request_id already has this
// exact shape, so the proxy access log and the app log can share one id.
func ParseTraceIDHex(value string) (TraceID, bool) {
	return parseTraceID(strings.TrimSpace(value))
}

// ContextWithTrace stores tc in ctx for downstream layers.
func ContextWithTrace(ctx context.Context, tc Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, tc)
}

// TraceFromContext reads the trace back; ok is false when none was set.
// A nil ctx is possible on fire-and-forget paths (Event.Ctx is optional).
func TraceFromContext(ctx context.Context) (Context, bool) {
	if ctx == nil {
		return Context{}, false
	}
	tc, ok := ctx.Value(ctxKey{}).(Context)
	return tc, ok && tc.Valid()
}

func parseTraceID(s string) (TraceID, bool) {
	var id TraceID
	if len(s) != TraceIDLen*2 || !isLowerHex(s) {
		return id, false
	}
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != TraceIDLen {
		return id, false
	}
	copy(id[:], raw)
	if id == (TraceID{}) {
		return id, false
	}
	return id, true
}

func parseSpanID(s string) (SpanID, bool) {
	var id SpanID
	if len(s) != SpanIDLen*2 || !isLowerHex(s) {
		return id, false
	}
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != SpanIDLen {
		return id, false
	}
	copy(id[:], raw)
	if id == (SpanID{}) {
		return id, false
	}
	return id, true
}

func parseFlags(s string) (byte, bool) {
	if len(s) != 2 || !isLowerHex(s) {
		return 0, false
	}
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != 1 {
		return 0, false
	}
	return raw[0], true
}

func isLowerHex(s string) bool {
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

func newTraceID() TraceID {
	var id TraceID
	for {
		if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
			panic("tracing: crypto/rand unavailable: " + err.Error())
		}
		if id != (TraceID{}) {
			return id
		}
	}
}

func newSpanID() SpanID {
	var id SpanID
	for {
		if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
			panic("tracing: crypto/rand unavailable: " + err.Error())
		}
		if id != (SpanID{}) {
			return id
		}
	}
}
