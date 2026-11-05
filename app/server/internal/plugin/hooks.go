package plugin

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PluginCaller 是宿主对一个插件发起调用的最小能力。
//
// 抽成函数类型而不是接口：测试里塞一个闭包就够了，不必造完整 Conn。
// 也正因为它是个函数值，supervisedCaller 可以在**每次分发时**重新取当前代
// 连接（见 supervisorCaller），而不是把某个可能已经死掉的 *Conn 钉在注册表里。
type PluginCaller func(ctx context.Context, method string, params any) ([]byte, error)

// hookTimeout 是单次钩子调用的默认上限。
//
// 钩子是异步的（在业务动作完成之后触发），所以超时不会影响用户请求；
// 但插件卡死会一直占着 goroutine，仍需兜底。dispatcher.timeout 可覆盖它
// （测试要靠这个把 5s 压到毫秒级）。
const hookTimeout = 5 * time.Second

// errPluginNotRunning 表示插件当前没有可用连接（尚未启动或已停止）。
//
// 与 ErrClosed 区分：ErrClosed 是「这一代连接结束了，下一代会再来」，
// 这个是「压根没有代」。两者都只记日志，不重试——钩子是事后通知。
var errPluginNotRunning = errors.New("plugin 未运行")

// dispatcher 是插件注册表与钩子分发器。
//
// 它只做两件事：记住「哪个插件声明了哪些钩子」，以及把一次钩子调用
// 扇出到所有订阅者。它不管理生命周期、不碰数据库、不持有 repository——
// 那些是 Task 7 的 Host 的事（Host 内嵌本类型）。
//
// 名字刻意不叫 Host：Task 7 另有一个对外的 Host（NewHostWithOptions）。
type dispatcher struct {
	// timeout 是单次钩子调用的上限，newDispatcher 里设为 hookTimeout。
	// 测试直接改这个字段来缩短超时，不必等 5 秒。
	timeout time.Duration

	mu      sync.RWMutex
	plugins map[string]*registeredPlugin
}

type registeredPlugin struct {
	manifest *Manifest
	call     PluginCaller
}

func newDispatcher() *dispatcher {
	return &dispatcher{
		timeout: hookTimeout,
		plugins: make(map[string]*registeredPlugin),
	}
}

// RegisterPlugin 登记一个已启动的插件。
//
// 传 Supervisor 而不是 *Conn 是刻意的：Supervisor.Conn() 返回**当前这一代**
// 的连接，崩溃重启后新一代连接才拿得到。若在这里钉死 Start 返回的 *Conn，
// 插件崩溃重启之后每次钩子都会拿到 ErrClosed，而状态面板显示「运行中」——
// 这是最难排查的一类故障（见 AGENTS.md「Start 只返回第一代 Conn」）。
func (d *dispatcher) RegisterPlugin(id string, m *Manifest, sup *Supervisor) {
	d.RegisterPluginWithCaller(id, m, supervisorCaller(sup))
}

// RegisterPluginWithCaller 是 RegisterPlugin 的接缝版本：测试塞闭包，
// 也供已经自行管理连接生命周期的调用方使用。
func (d *dispatcher) RegisterPluginWithCaller(id string, m *Manifest, call PluginCaller) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.plugins[id] = &registeredPlugin{manifest: m, call: call}
}

// UnregisterPlugin 摘除插件。之后不再收到任何钩子。
func (d *dispatcher) UnregisterPlugin(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.plugins, id)
}

// supervisorCaller 把 Supervisor 适配成 PluginCaller，并且**每次调用都重新取当前代**。
func supervisorCaller(sup *Supervisor) PluginCaller {
	return func(ctx context.Context, method string, params any) ([]byte, error) {
		if sup == nil {
			return nil, errPluginNotRunning
		}
		conn := sup.Conn()
		if conn == nil {
			return nil, errPluginNotRunning
		}
		return conn.Call(ctx, method, params)
	}
}

// Dispatch 把一次钩子事件分发给所有订阅它的插件。
//
// 语义（每一条都有测试钉住）：
//   - **异步**：本方法只启动调用就返回，不等插件完成。调用方通常是事件总线的
//     回调 goroutine，让它等插件会用一个卡死的插件拖住同类型事件的后续投递。
//   - **独立超时**：每次调用各带一个 timeout，插件不返回时到点取消。
//   - **不继承取消**：调用方的 ctx 只用来传递 trace 等值，取消/截止时间被剥离
//     （context.WithoutCancel）。业务请求结束后才触发的钩子不应该被请求取消掐断。
//   - **失败隔离**：任一插件失败只记日志，不影响其他插件，也不影响已完成的业务动作。
//
// 调用方 MUST NOT 在 Dispatch 之后修改 payload：同一个 map 会交给所有插件的
// 序列化路径，改它就是数据竞争。
func (d *dispatcher) Dispatch(ctx context.Context, hook string, payload map[string]any) {
	d.dispatchHooks(ctx, []string{hook}, payload)
}

// dispatchHooks 把一次事件扇出到若干钩子对应的插件。
//
// 事件桥接用这个入口：一个事件可能绑多个钩子（见 HookEvents 的注释），
// 每个钩子是一条独立的 IPC 方法，因此要分别投递。
func (d *dispatcher) dispatchHooks(ctx context.Context, hooks []string, payload map[string]any) {
	targets := d.targets(hooks)
	if len(targets) == 0 {
		return
	}

	// 一次 WithoutCancel 就够：派生出的调用 ctx 只在 timeout 上不同。
	base := detachContext(ctx)
	for _, t := range targets {
		go func(t dispatchTarget) {
			// panic 必须在这里收住：它在独立 goroutine 里，逃出去就是整个宿主进程
			// 崩溃——等于「一个插件失败拖垮站点」，直接违反失败隔离。
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("plugin: %s 处理钩子 %s 时 panic: %v", t.id, t.hook, r)
				}
			}()

			callCtx, cancel := context.WithTimeout(base, d.callTimeout())
			defer cancel()

			if _, err := t.call(callCtx, "hook."+t.hook, payload); err != nil {
				// 钩子是事后通知：失败只记日志，绝不重试、绝不回滚业务动作。
				// ErrClosed 意味着这一代子进程结束了（supervisor 会重启并换代），
				// 下一次事件自然用新连接，这里不必也不该做任何补救。
				entry := logrus.WithError(err).WithFields(logrus.Fields{
					"plugin": t.id,
					"hook":   t.hook,
				})
				if errors.Is(err, ErrClosed) || errors.Is(err, errPluginNotRunning) {
					entry.Debug("plugin: 钩子调用时连接已结束（等待换代）")
					return
				}
				entry.Warn("plugin: 钩子处理失败")
			}
		}(t)
	}
}

// dispatchTarget 是一次扇出中的一个投递目标（一个插件上的一个钩子）。
type dispatchTarget struct {
	id   string
	hook string
	call PluginCaller
}

// targets 返回订阅了给定钩子之一的插件调用入口。
//
// 在锁内**复制**出快照，调用发生在锁外：插件处理钩子时可能反过来调宿主 API
// （进而登记/摘除插件），持锁调用会自死锁。
func (d *dispatcher) targets(hooks []string) []dispatchTarget {
	if len(hooks) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(hooks))
	for _, h := range hooks {
		want[h] = struct{}{}
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var out []dispatchTarget
	for id, p := range d.plugins {
		if p == nil || p.manifest == nil || p.call == nil {
			// 畸形注册（nil manifest）：没有钩子可分发，跳过而不是 panic。
			// 若在这里 panic，事件总线的回调会掐断整条订阅。
			continue
		}
		seen := make(map[string]struct{}, len(p.manifest.Hooks))
		for _, h := range p.manifest.Hooks {
			if _, dup := seen[h]; dup {
				continue // manifest 里重复声明同一钩子只投一次
			}
			seen[h] = struct{}{}
			if _, ok := want[h]; !ok {
				continue
			}
			out = append(out, dispatchTarget{id: id, hook: h, call: p.call})
		}
	}
	return out
}

// callTimeout 取本次分发的超时上限，未设置时回落到 hookTimeout。
func (d *dispatcher) callTimeout() time.Duration {
	if d.timeout <= 0 {
		return hookTimeout
	}
	return d.timeout
}

// detachContext 剥离 ctx 的取消与截止时间，只保留其中携带的值（trace 等）。
//
// 钩子在业务动作**完成之后**触发。业务请求返回时取消 ctx 是常态，
// 若钩子继承这次取消，插件会在写完日志之前被静默掐断——spec 明确要求
// 钩子失败不得影响业务动作，反过来业务动作的收尾也不该让钩子消失。
func detachContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

// invertHookEvents 把「钩子 → 事件」的映射倒排成「事件 → 钩子」。
//
// 事件桥接必须用倒排表，不能遍历 KnownHooks() 做恒等映射：
//   - comment.created 的真实事件叫 content.created，恒等映射会让插件静默收不到；
//   - cron.daily 没有事件源（ticker 驱动），不该进事件订阅；
//   - 一个事件将来可能绑多个钩子，倒排天然支持，且**每个事件只订阅一次**。
//
// 每个事件下的钩子名按字典序排列，订阅/日志的顺序因此是确定的。
func invertHookEvents(m map[string]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for hook, event := range m {
		out[event] = append(out[event], hook)
	}
	for event := range out {
		sort.Strings(out[event])
	}
	return out
}

// Broadcaster 是事件总线的接缝，避免分发器依赖 eventbus 的具体类型。
type Broadcaster interface {
	SubscribeBroadcast(eventType string, handler func(eventType string, payload map[string]any))
}

// TraceBroadcaster 是 Broadcaster 的可选增强接缝：额外把事件的 ctx 透传出来。
//
// eventbus 的广播 handler 拿到的是带 traceparent 的 context，而 Broadcaster
// 的二元签名会丢掉它。SubscribeEvents 若发现订阅方实现了本接口，就用 ctx 版本，
// 于是插件钩子的日志仍能带 trace_id。
type TraceBroadcaster interface {
	SubscribeBroadcastContext(eventType string, handler func(ctx context.Context, eventType string, payload map[string]any))
}

// SubscribeEvents 把事件总线的广播事件桥接到钩子分发。
//
// 必须走 HookEvents() 的**倒排**映射，不能遍历 KnownHooks() 做恒等映射：
// 后者会让 comment.created 永远收不到（真实事件叫 content.created），
// 而 cron.daily 会被订阅一个不存在的事件。
func (d *dispatcher) SubscribeEvents(b Broadcaster) {
	if b == nil {
		return
	}
	tb, hasTrace := b.(TraceBroadcaster)

	for event, hooks := range invertHookEvents(HookEvents()) {
		event, hooks := event, hooks
		if hasTrace {
			tb.SubscribeBroadcastContext(event, func(ctx context.Context, _ string, payload map[string]any) {
				d.dispatchHooks(ctx, hooks, payload)
			})
			continue
		}
		b.SubscribeBroadcast(event, func(_ string, payload map[string]any) {
			// 常量接缝没有 ctx：trace 只能丢在这里。
			d.dispatchHooks(context.Background(), hooks, payload)
		})
	}
}
