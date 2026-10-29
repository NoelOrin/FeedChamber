package logger

import (
	"context"
	"errors"
	"testing"

	"orinforum/internal/pkg/tracing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestWith_ReturnsNewInstance(t *testing.T) {
	base := New(Config{})
	enriched := base.With(F("key", "val"))
	assert.NotEqual(t, base, enriched)
}

func TestWithError(t *testing.T) {
	l := New(Config{})
	err := errors.New("something broke")
	_ = l.WithError(err) // 不 panic 即可，实际输出需集成验证
}

func TestWithComponent(t *testing.T) {
	l := New(Config{})
	child := l.WithComponent("auth")
	assert.NotEqual(t, l, child)
}

func TestWithContext_ExtractsTraceFields(t *testing.T) {
	tc := tracing.New()
	ctx := tracing.ContextWithTrace(context.Background(), tc)

	l := New(Config{}).WithContext(ctx)
	l.Info("test message") // 不 panic 即可；字段由 hooks 断言验证

	entry := l.(*logrusLogger).entry
	assert.Equal(t, tc.TraceIDHex(), entry.Data["trace_id"])
	assert.Equal(t, tc.SpanIDHex(), entry.Data["span_id"])
}

func TestWithContext_NoTraceInContextLeavesLoggerUnchanged(t *testing.T) {
	base := New(Config{})
	got := base.WithContext(context.Background()).(*logrusLogger)

	assert.NotContains(t, got.entry.Data, "trace_id")
}

func TestSetGlobalSyncsStandardLogrus(t *testing.T) {
	previous := logrus.StandardLogger().Level
	t.Cleanup(func() { logrus.SetLevel(previous) })

	SetGlobal(New(Config{Level: "debug", Format: "text"}))

	assert.Equal(t, logrus.DebugLevel, logrus.StandardLogger().Level,
		"约 156 处裸 logrus 调用应随 LOG_LEVEL 生效")
}

func TestNewDoesNotTouchStandardLogrus(t *testing.T) {
	previous := logrus.StandardLogger().Level
	t.Cleanup(func() { logrus.SetLevel(previous) })

	assert.NotPanics(t, func() { _ = New(Config{Level: "debug"}) })
	assert.Equal(t, previous, logrus.StandardLogger().Level,
		"New 必须保持无全局副作用，避免测试互相污染")
}

func TestFromContext_FallbackToDefault(t *testing.T) {
	ctx := context.Background()
	l := FromContext(ctx)
	assert.NotNil(t, l)
}

func TestFromContext_UsesInjected(t *testing.T) {
	custom := New(Config{}).With(F("service", "test"))
	ctx := NewContext(context.Background(), custom)
	l := FromContext(ctx)
	assert.Equal(t, custom, l)
}

func TestNewNop(t *testing.T) {
	l := NewNop()
	// 所有级别都不 panic，不产生输出
	l.Debug("debug")
	l.Info("info")
	l.Warn("warn")
	l.Error("error")
}

func TestNew_WithConfig(t *testing.T) {
	l := New(Config{Level: "debug", Format: "text"})
	l.Debug("should appear")
}

func TestF(t *testing.T) {
	f := F("count", 42)
	assert.Equal(t, "count", f.Key)
	assert.Equal(t, 42, f.Value)
}

func TestChain(t *testing.T) {
	l := New(Config{}).
		With(F("service", "forum")).
		WithComponent("post").
		With(F("post_id", "abc"))
	l.Info("post created")
}
