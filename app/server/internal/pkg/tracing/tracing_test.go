package tracing

import (
	"context"
	"strings"
	"testing"
)

func TestNewProducesValidTrace(t *testing.T) {
	a, b := New(), New()

	if !a.Valid() || !b.Valid() {
		t.Fatal("New() must produce non-zero ids")
	}
	if a.TraceID == b.TraceID {
		t.Fatalf("New() returned duplicate trace id %s", a.TraceIDHex())
	}
	if len(a.TraceIDHex()) != 32 || len(a.SpanIDHex()) != 16 {
		t.Fatal("hex form must be 32 / 16 chars")
	}
	if strings.ToLower(a.TraceIDHex()) != a.TraceIDHex() {
		t.Fatal("trace id hex must be lowercase")
	}
}

func TestTraceparentRoundTrip(t *testing.T) {
	want := New()

	got, ok := ParseTraceparent(want.Traceparent())
	if !ok {
		t.Fatalf("ParseTraceparent(%q) = false, want true", want.Traceparent())
	}
	if got.TraceID != want.TraceID || got.SpanID != want.SpanID || got.Flags != want.Flags {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestParseTraceparentRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"empty":             "",
		"wrong version":     "01-" + hex32("1") + "-" + hex16("2") + "-01",
		"uppercase version": "FF-" + hex32("1") + "-" + hex16("2") + "-01",
		"uppercase trace":   "00-" + strings.ToUpper(hex32("a")) + "-" + hex16("2") + "-01",
		"short trace":       "00-" + hex32("1")[:30] + "-" + hex16("2") + "-01",
		"long trace":        "00-" + hex32("1") + "ab-" + hex16("2") + "-01",
		"short span":        "00-" + hex32("1") + "-" + hex16("2")[:14] + "-01",
		"all zero trace":    "00-" + strings.Repeat("0", 32) + "-" + hex16("2") + "-01",
		"all zero span":     "00-" + hex32("1") + "-" + strings.Repeat("0", 16) + "-01",
		"non hex":           "00-" + strings.Repeat("g", 32) + "-" + hex16("2") + "-01",
		"too few fields":    "00-" + hex32("1") + "-" + hex16("2"),
		"too many fields":   "00-" + hex32("1") + "-" + hex16("2") + "-01-extra",
		"bad flags":         "00-" + hex32("1") + "-" + hex16("2") + "-zz",
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if got, ok := ParseTraceparent(value); ok {
				t.Fatalf("ParseTraceparent(%q) accepted bad input: %+v", value, got)
			}
		})
	}
}

func TestParseTraceparentAcceptsFlagValues(t *testing.T) {
	for _, flags := range []string{"00", "01", "ff"} {
		value := traceparentVersion + "-" + hex32("a") + "-" + hex16("b") + "-" + flags
		got, ok := ParseTraceparent(value)
		if !ok {
			t.Fatalf("ParseTraceparent(%q) = false, want true", value)
		}
		if got.Flags != parseHexByte(flags) {
			t.Fatalf("Flags = %02x, want %s", got.Flags, flags)
		}
	}
}

func TestChildKeepsTraceAndRenamesSpan(t *testing.T) {
	parent := New()
	child := parent.Child()

	if child.TraceID != parent.TraceID {
		t.Fatal("Child must keep the trace id")
	}
	if child.SpanID == parent.SpanID {
		t.Fatal("Child must mint a new span id")
	}
	if child.ParentSpanID != parent.SpanID {
		t.Fatal("Child must record the previous span id as parent")
	}
}

func TestContextRoundTrip(t *testing.T) {
	want := New()
	ctx := ContextWithTrace(context.Background(), want)

	got, ok := TraceFromContext(ctx)
	if !ok {
		t.Fatal("TraceFromContext() = false, want true")
	}
	if got.TraceID != want.TraceID || got.SpanID != want.SpanID {
		t.Fatalf("context mismatch: got %+v want %+v", got, want)
	}
}

func TestTraceFromContextEmpty(t *testing.T) {
	if _, ok := TraceFromContext(context.Background()); ok {
		t.Fatal("TraceFromContext(context.Background()) = true, want false")
	}
	if _, ok := TraceFromContext(ContextWithTrace(context.Background(), Context{})); ok {
		t.Fatal("zero Context must not be treated as a trace")
	}
}

func TestParseTraceIDHex(t *testing.T) {
	id, ok := ParseTraceIDHex(hex32("a"))
	if !ok || id != mustTrace(t, hex32("a")) {
		t.Fatalf("ParseTraceIDHex valid case failed: %+v %v", id, ok)
	}
	if _, ok := ParseTraceIDHex(strings.ToUpper(hex32("a"))); ok {
		t.Fatal("uppercase hex must be rejected")
	}
	if _, ok := ParseTraceIDHex(strings.Repeat("0", 32)); ok {
		t.Fatal("all-zero must be rejected")
	}
	if _, ok := ParseTraceIDHex("36f7f0b6b1a4b9e8d7c6f5a4b3c2d1"); ok {
		t.Fatal("31 chars must be rejected")
	}
}

func mustTrace(t *testing.T, hex32 string) TraceID {
	t.Helper()
	id, ok := ParseTraceIDHex(hex32)
	if !ok {
		t.Fatalf("ParseTraceIDHex(%q) failed", hex32)
	}
	return id
}

// hex32 / hex16 build deterministic padded hex fields for test inputs.
func hex32(seed string) string { return pad(strings.Repeat(seed, 2), 32) }
func hex16(seed string) string { return pad(strings.Repeat(seed, 2), 16) }

func pad(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s + strings.Repeat("0", n-len(s))
}

func parseHexByte(s string) byte {
	var b byte
	for _, c := range s {
		b <<= 4
		switch {
		case c >= '0' && c <= '9':
			b |= byte(c - '0')
		case c >= 'a' && c <= 'f':
			b |= byte(c-'a') + 10
		}
	}
	return b
}
