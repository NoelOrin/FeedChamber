package logger

import (
	"context"
	"io"
	"os"

	"orinforum/internal/pkg/tracing"

	"github.com/sirupsen/logrus"
)

// ── Logger 能力接口 ─────────────────────────────────────

// Logger 结构化日志接口。
// 所有返回 Logger 的方法均返回新实例（不可变），支持链式调用。
type Logger interface {
	// ── 字段构建 ──

	// With 追加结构化字段，返回新 Logger。
	With(fields ...Field) Logger

	// WithError 将 error 作为 "error" 字段追加。
	WithError(err error) Logger

	// WithComponent 标记组件名，追加 "component" 字段。
	WithComponent(name string) Logger

	// ── 日志输出 ──

	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field) // 输出后 os.Exit(1)

	// ── context 集成 ──

	// WithContext 从 context 提取 trace 字段（trace_id, span_id），
	// 返回关联了这些字段的新 Logger。
	WithContext(ctx context.Context) Logger
}

// ── Config ───────────────────────────────────────────────

// Config 日志配置。
type Config struct {
	Level  string // debug | info | warn | error，默认 info
	Format string // json | text，默认 json（生产推荐）
	Output string // stdout | stderr | 文件路径，默认 stderr
}

// ── Field ────────────────────────────────────────────────

// Field 结构化日志字段。
type Field struct {
	Key   string
	Value any
}

// F 快捷构造 Field。
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// ── context 集成 ─────────────────────────────────────────

type ctxKey struct{}

// NewContext 将 Logger 存入 context。
func NewContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext 从 context 取 Logger，未找到则返回全局默认 logger。
// 由 Trace/Logger 中间件写入，Service 与 Repository 层用
// `logger.FromContext(ctx)` 即可拿到带 trace_id 的请求级 logger。
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}
	return std
}

// ── 全局默认 ─────────────────────────────────────────────

var std Logger

func init() {
	std = New(Config{})
}

// SetGlobal 设置全局默认 logger（启动时调用），并同步配置 logrus 的标准 logger，
// 使仓库内约 156 处直接调用 logrus.Info/Warn/... 的存量代码一并受
// LOG_LEVEL / LOG_FORMAT / LOG_OUTPUT 管辖。New 本身不改全局状态，
// 因此测试可安全构造独立实例。
func SetGlobal(l Logger) {
	std = l
	if lr, ok := l.(*logrusLogger); ok {
		base := lr.entry.Logger
		logrus.SetLevel(base.Level)
		logrus.SetFormatter(base.Formatter)
		logrus.SetOutput(base.Out)
	}
}

// Global 返回全局默认 logger。
func Global() Logger { return std }

// ── 工厂函数 ─────────────────────────────────────────────

// New 根据配置创建 Logger。零值 Config 使用 json 格式 + info 级别 + stderr。
func New(cfg Config) Logger {
	l := logrus.New()

	// 输出
	switch cfg.Output {
	case "stdout":
		l.SetOutput(os.Stdout)
	case "", "stderr":
		l.SetOutput(os.Stderr)
	default:
		f, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			l.SetOutput(os.Stderr)
			l.Warnf("open log file %s failed: %v, fallback stderr", cfg.Output, err)
		} else {
			l.SetOutput(f)
		}
	}

	// 格式
	switch cfg.Format {
	case "text":
		l.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000",
		})
	default: // json
		l.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000",
		})
	}

	// 级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	l.SetLevel(level)

	return &logrusLogger{entry: logrus.NewEntry(l)}
}

// NewNop 返回空操作 Logger（测试用）。不触碰标准 logger，保证测试之间不互相污染。
func NewNop() Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return &logrusLogger{entry: logrus.NewEntry(l)}
}

// ── logrus 实现 ──────────────────────────────────────────

type logrusLogger struct {
	entry *logrus.Entry
}

func (l *logrusLogger) With(fields ...Field) Logger {
	logrusFields := make(logrus.Fields, len(fields))
	for _, f := range fields {
		logrusFields[f.Key] = f.Value
	}
	return &logrusLogger{entry: l.entry.WithFields(logrusFields)}
}

func (l *logrusLogger) WithError(err error) Logger {
	return &logrusLogger{entry: l.entry.WithError(err)}
}

func (l *logrusLogger) WithComponent(name string) Logger {
	return &logrusLogger{entry: l.entry.WithField("component", name)}
}

func (l *logrusLogger) WithContext(ctx context.Context) Logger {
	tc, ok := tracing.TraceFromContext(ctx)
	if !ok {
		return l
	}
	return l.With(F("trace_id", tc.TraceIDHex()), F("span_id", tc.SpanIDHex()))
}

func (l *logrusLogger) Debug(msg string, fields ...Field) { l.log(logrus.DebugLevel, msg, fields) }
func (l *logrusLogger) Info(msg string, fields ...Field)  { l.log(logrus.InfoLevel, msg, fields) }
func (l *logrusLogger) Warn(msg string, fields ...Field)  { l.log(logrus.WarnLevel, msg, fields) }
func (l *logrusLogger) Error(msg string, fields ...Field) { l.log(logrus.ErrorLevel, msg, fields) }
func (l *logrusLogger) Fatal(msg string, fields ...Field) { l.log(logrus.FatalLevel, msg, fields) }

func (l *logrusLogger) log(level logrus.Level, msg string, fields []Field) {
	if len(fields) == 0 {
		l.entry.Log(level, msg)
		return
	}
	logrusFields := make(logrus.Fields, len(fields))
	for _, f := range fields {
		logrusFields[f.Key] = f.Value
	}
	l.entry.WithFields(logrusFields).Log(level, msg)
}
