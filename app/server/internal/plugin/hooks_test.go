package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/testnats"

	"github.com/stretchr/testify/require"
)

// recorder 收集 fake caller 的调用，供断言使用。
type recorder struct {
	mu      sync.Mutex
	methods map[string][]string
	params  map[string][]any
	counts  map[string]int
}

func newRecorder() *recorder {
	return &recorder{
		methods: map[string][]string{},
		params:  map[string][]any{},
		counts:  map[string]int{},
	}
}

func (r *recorder) caller(id string) PluginCaller {
	return func(ctx context.Context, method string, params any) ([]byte, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.counts[id]++
		r.methods[id] = append(r.methods[id], method)
		r.params[id] = append(r.params[id], params)
		return []byte(`{}`), nil
	}
}

func (r *recorder) count(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[id]
}

func (r *recorder) methodOf(id string, i int) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.methods[id]) <= i {
		return ""
	}
	return r.methods[id][i]
}

// ---------------------------------------------------------------------------
// 基本分发语义
// ---------------------------------------------------------------------------

func TestDispatcherDispatchFansOutToAllSubscribers(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()
	for _, id := range []string{"a", "b"} {
		id := id
		d.RegisterPluginWithCaller(id, &Manifest{Name: id, Hooks: []string{HookPostCreated}}, rec.caller(id))
	}

	d.Dispatch(context.Background(), HookPostCreated, map[string]any{"post_id": "p1"})

	require.Eventually(t, func() bool {
		return rec.count("a") == 1 && rec.count("b") == 1
	}, 2*time.Second, 10*time.Millisecond, "两个插件都必须收到")

	// IPC 方法名是 hook.<钩子名>：插件侧按方法名分派（见 ipc_test 与 Task 6 的宿主 API）。
	require.Equal(t, "hook.post.created", rec.methodOf("a", 0))
	require.Equal(t, "hook.post.created", rec.methodOf("b", 0))
}

func TestDispatcherDispatchIgnoresNonSubscribers(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()
	d.RegisterPluginWithCaller("a", &Manifest{Name: "a", Hooks: []string{HookCommentCreated}}, rec.caller("a"))

	d.Dispatch(context.Background(), HookPostCreated, nil)

	time.Sleep(300 * time.Millisecond)
	require.Equal(t, 0, rec.count("a"), "未订阅该钩子的插件不得被调用")
}

func TestDispatcherDispatchFailureDoesNotBlockOthers(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()

	d.RegisterPluginWithCaller("bad", &Manifest{Name: "bad", Hooks: []string{HookPostCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			return nil, errors.New("boom")
		})
	d.RegisterPluginWithCaller("good", &Manifest{Name: "good", Hooks: []string{HookPostCreated}}, rec.caller("good"))

	d.Dispatch(context.Background(), HookPostCreated, nil)

	require.Eventually(t, func() bool {
		return rec.count("good") == 1
	}, 2*time.Second, 10*time.Millisecond, "一个插件失败不得影响其他插件")
}

// 同一钩子在 manifest 里重复声明（插件作者手误）时只能调用一次：
// 分发按插件为粒度，不是按声明条目为粒度。
func TestDispatcherDispatchDedupesDuplicateHookDeclaration(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()
	d.RegisterPluginWithCaller("dup", &Manifest{
		Name:  "dup",
		Hooks: []string{HookPostCreated, HookPostCreated},
	}, rec.caller("dup"))

	d.Dispatch(context.Background(), HookPostCreated, nil)

	require.Eventually(t, func() bool { return rec.count("dup") == 1 }, 2*time.Second, 10*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	require.Equal(t, 1, rec.count("dup"), "重复声明不得导致重复调用")
}

func TestDispatcherUnregisterStopsDispatch(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()
	d.RegisterPluginWithCaller("a", &Manifest{Name: "a", Hooks: []string{HookPostCreated}}, rec.caller("a"))

	d.Dispatch(context.Background(), HookPostCreated, nil)
	require.Eventually(t, func() bool { return rec.count("a") == 1 }, 2*time.Second, 10*time.Millisecond)

	d.UnregisterPlugin("a")
	d.Dispatch(context.Background(), HookPostCreated, nil)

	time.Sleep(150 * time.Millisecond)
	require.Equal(t, 1, rec.count("a"), "摘除后的插件不得再收到钩子")
}

// 分发路径上的 panic 会掐断整条事件流，因此对畸形注册要能容忍：
// nil manifest（没有声明任何钩子）与 nil ctx 都不该 panic。
func TestDispatcherToleratesNilManifestAndNilContext(t *testing.T) {
	d := newDispatcher()
	d.RegisterPluginWithCaller("broken", nil, func(ctx context.Context, method string, params any) ([]byte, error) {
		return []byte(`{}`), nil
	})

	require.NotPanics(t, func() {
		d.Dispatch(context.Background(), HookPostCreated, nil)
		//nolint:staticcheck // 故意传 nil ctx：分发必须自己兜底，不能 panic
		d.Dispatch(nil, HookPostCreated, nil)
	})
}

// ---------------------------------------------------------------------------
// 「不得阻塞业务动作」与「每次分发必须有超时」
// ---------------------------------------------------------------------------

// Dispatch 必须在插件完成之前返回。钩子是事后通知，业务动作已经返回；
// 让事件总线的回调（或任何直接调用方）等插件，会用一个卡死的插件
// 拖住后续所有同类型事件的投递。
func TestDispatcherDispatchDoesNotBlockOnSlowPlugin(t *testing.T) {
	d := newDispatcher()
	d.timeout = 300 * time.Millisecond

	release := make(chan struct{})
	// 释放插件并让 Dispatch 一定返回：否则「同步分发」这个 mutation 会让
	// 测试永久挂住（挂住 ≠ 变红，CI 里只表现为超时）。1s 后无条件放行，
	// 把「阻塞」变成「elapsed 断言失败」这种可读的红。
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	time.AfterFunc(time.Second, unblock)

	d.RegisterPluginWithCaller("slow", &Manifest{Name: "slow", Hooks: []string{HookPostCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			<-release
			return []byte(`{}`), nil
		})

	begin := time.Now()
	d.Dispatch(context.Background(), HookPostCreated, nil)
	elapsed := time.Since(begin)
	unblock()

	require.Less(t, elapsed, 100*time.Millisecond,
		"Dispatch MUST NOT 等待插件完成（实测 %s）", elapsed)
}

// 每次钩子调用都必须带上限：插件不返回时，调用要在上限处被取消，
// 而不是永久占住一个 goroutine。
func TestDispatcherDispatchAppliesPerCallTimeout(t *testing.T) {
	d := newDispatcher()
	d.timeout = 60 * time.Millisecond

	type outcome struct {
		deadline time.Duration
		err      error
	}
	got := make(chan outcome, 1)
	d.RegisterPluginWithCaller("stuck", &Manifest{Name: "stuck", Hooks: []string{HookPostCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			dl, ok := ctx.Deadline()
			if !ok {
				got <- outcome{deadline: -1}
				return nil, errors.New("钩子调用没有 deadline")
			}
			// 必须在 ctx.Done() 之前取剩余时间：到期后再取会是负数。
			remaining := time.Until(dl)
			<-ctx.Done()
			got <- outcome{deadline: remaining, err: ctx.Err()}
			return nil, ctx.Err()
		})

	d.Dispatch(context.Background(), HookPostCreated, nil)

	select {
	case o := <-got:
		require.GreaterOrEqual(t, o.deadline, time.Duration(0), "deadline 不该已经过去")
		require.LessOrEqual(t, o.deadline, 60*time.Millisecond, "deadline 不得超过配置的钩子上限")
		require.ErrorIs(t, o.err, context.DeadlineExceeded, "超时必须取消这次调用")
	case <-time.After(2 * time.Second):
		t.Fatal("钩子调用没有被超时取消，插件卡死会永久占住 goroutine")
	}
}

// 钩子不得继承调用方的取消：请求结束时取消 ctx 是常态，
// 若继承取消，业务动作完成之后钩子会被静默掐断（spec：钩子在动作完成后触发）。
func TestDispatcherDispatchSurvivesDeadCallerContext(t *testing.T) {
	d := newDispatcher()
	d.timeout = 2 * time.Second

	seen := make(chan error, 1)
	d.RegisterPluginWithCaller("a", &Manifest{Name: "a", Hooks: []string{HookPostCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			seen <- ctx.Err()
			return []byte(`{}`), nil
		})

	dead, cancel := context.WithCancel(context.Background())
	cancel() // 模拟「请求已结束」
	d.Dispatch(dead, HookPostCreated, nil)

	select {
	case err := <-seen:
		require.NoError(t, err, "钩子调用不得继承调用方已取消的 context")
	case <-time.After(2 * time.Second):
		t.Fatal("插件没有被调用：钩子继承了调用方的取消")
	}
}

// ---------------------------------------------------------------------------
// 倒排索引：事件 → 钩子
// ---------------------------------------------------------------------------

// 同一事件下挂多个钩子时只能订阅一次，但两个钩子都要分发。
func TestInvertHookEventsGroupsHooksPerEvent(t *testing.T) {
	idx := invertHookEvents(map[string]string{
		HookPostCreated:    "post.created",
		HookCommentCreated: "content.created",
		HookUserFollowed:   "content.created", // 人为制造「一事件多钩子」
	})

	require.Equal(t, []string{HookCommentCreated, HookUserFollowed}, idx["content.created"],
		"同一事件下的钩子必须聚在一起且顺序稳定")
	require.Equal(t, []string{HookPostCreated}, idx["post.created"])
	require.Len(t, idx, 2, "两个钩子绑同一事件只产生一个条目")
}

// fakeBroadcaster 记录被订阅的事件，并能手工触发 handler。
type fakeBroadcaster struct {
	mu       sync.Mutex
	events   []string
	handlers map[string]func(string, map[string]any)
}

func newFakeBroadcaster() *fakeBroadcaster {
	return &fakeBroadcaster{handlers: map[string]func(string, map[string]any){}}
}

func (f *fakeBroadcaster) SubscribeBroadcast(eventType string, handler func(string, map[string]any)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, eventType)
	f.handlers[eventType] = handler
}

func (f *fakeBroadcaster) subscribed() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int, len(f.events))
	for _, e := range f.events {
		out[e]++
	}
	return out
}

func (f *fakeBroadcaster) fire(t *testing.T, eventType string, payload map[string]any) {
	t.Helper()
	f.mu.Lock()
	h := f.handlers[eventType]
	f.mu.Unlock()
	require.NotNil(t, h, "事件 %s 没有被订阅", eventType)
	h(eventType, payload)
}

// fireIfSubscribed 只在事件真被订阅时投递，模拟「双发」中未被订阅的那条：
// 真实 eventbus 对没人订阅的事件就是什么都不做。
func (f *fakeBroadcaster) fireIfSubscribed(t *testing.T, eventType string, payload map[string]any) {
	t.Helper()
	f.mu.Lock()
	h := f.handlers[eventType]
	f.mu.Unlock()
	if h == nil {
		return
	}
	h(eventType, payload)
}

// SubscribeEvents 必须按 HookEvents() 的映射订阅，每个事件恰好一次；
// cron.daily 由 ticker 驱动，不进事件总线。
func TestDispatcherSubscribeEventsSubscribesEventDrivenHooksOnly(t *testing.T) {
	d := newDispatcher()
	fb := newFakeBroadcaster()
	d.SubscribeEvents(fb)

	got := fb.subscribed()
	want := map[string]int{}
	for _, event := range HookEvents() {
		want[event]++
	}
	require.Equal(t, want, got, "订阅集合必须恰好等于 HookEvents() 的事件集合，每事件一次")

	require.Equal(t, 1, got[eventbus.EventContentCreated], "comment.created 的真实事件叫 content.created")
	require.NotContains(t, got, "cron.daily", "cron.daily 由 ticker 驱动，不得订阅事件")
	require.Len(t, got, 7, "事件驱动钩子恰好 7 个，每个事件一条订阅")
	require.NotContains(t, got, eventbus.EventUserCreated,
		"不得订阅 user.created：auth 双发会让 user.registered 钩子投递两次")
}

// 事件名与钩子名分叉的路径必须真的送达：content.created → comment.created。
func TestDispatcherDeliversContentCreatedToCommentHook(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()
	d.RegisterPluginWithCaller("c", &Manifest{Name: "c", Hooks: []string{HookCommentCreated}}, rec.caller("c"))

	fb := newFakeBroadcaster()
	d.SubscribeEvents(fb)
	fb.fire(t, eventbus.EventContentCreated, map[string]any{"content_id": "c1"})

	require.Eventually(t, func() bool { return rec.count("c") == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, "hook.comment.created", rec.methodOf("c", 0))
}

// ---------------------------------------------------------------------------
// 端到端：auth.go 的 user.created + user.registered 双发
// ---------------------------------------------------------------------------

// 事件名与钩子名分叉之外的另一条路径：auth.go 注册成功时同时发布
// user.created 与 user.registered（EventUserRegistered 是历史别名）。
// 插件声明的 user.registered 钩子对一次注册只能收到一次调用。
//
// 这里用 fakeBroadcaster 而不是真 eventbus：本测试要钉的是**订阅了哪些事件**
// 这个决策，而 fakeBroadcaster 按事件名分发的语义与 eventbus 的广播完全一致
// （真 eventbus 的适配层由 internal/plugin/pluginevent 的测试覆盖）。
func TestDispatcherUserRegisteredFiresOnceOnDoublePublish(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()
	d.RegisterPluginWithCaller("reg", &Manifest{Name: "reg", Hooks: []string{HookUserRegistered}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			rec.mu.Lock()
			rec.counts["reg"]++
			rec.methods["reg"] = append(rec.methods["reg"], method)
			rec.params["reg"] = append(rec.params["reg"], params)
			rec.mu.Unlock()
			return []byte(`{}`), nil
		})

	fb := newFakeBroadcaster()
	d.SubscribeEvents(fb)

	// 与 internal/service/auth.go 的注册成功路径一致：两条事件都发。
	fb.fireIfSubscribed(t, eventbus.EventUserCreated, map[string]any{"user_id": "u1"})
	fb.fireIfSubscribed(t, eventbus.EventUserRegistered, map[string]any{"user_id": "u1"})

	require.Eventually(t, func() bool { return rec.count("reg") == 1 }, 2*time.Second, 10*time.Millisecond,
		"一次注册必须恰好投递一次 user.registered")

	// 再等一会儿确认没有第二次投递（双发若两条都命中，第二条会在这段时间到达）。
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, 1, rec.count("reg"), "user.created 不得再触发一次 user.registered")
	require.Equal(t, "hook.user.registered", rec.methodOf("reg", 0))

	rec.mu.Lock()
	payload, _ := rec.params["reg"][0].(map[string]any)
	rec.mu.Unlock()
	require.Equal(t, "u1", payload["user_id"], "payload 必须原样透传到插件")

	require.NotContains(t, fb.subscribed(), eventbus.EventUserCreated,
		"不得订阅 user.created：否则双发会导致同一插件被调用两次")
	require.Equal(t, 1, fb.subscribed()[eventbus.EventUserRegistered])
}

// ---------------------------------------------------------------------------
// 端到端：真 eventbus（含真实 NATS 广播路径）
// ---------------------------------------------------------------------------

// testBusAdapter 与 internal/plugin/pluginevent.Broadcaster 同形。
//
// 这里重写一遍而不是 import 那个子包：子包反过来 import 本包，在包内测试里
// 会构成 import cycle（package plugin 的测试文件导入 pluginevent，后者再导入
// plugin）。子包自身的适配语义由 pluginevent 的测试覆盖，这里只借用它证明
// 「真实的 eventbus 广播路径能把事件送到钩子」。
type testBusAdapter struct {
	bus interface {
		SubscribeBroadcast(string, func(e eventbusEventForTest))
	}
}

// 上面那种抽象不可用：eventbus.Handler 是具体签名，直接用真类型。
type eventbusEventForTest = struct{}

// realBusAdapter 把 *eventbus.EventBus 适配成 Broadcaster（含 trace ctx 版本）。
type realBusAdapter struct{ bus *eventbus.EventBus }

func (a realBusAdapter) SubscribeBroadcast(eventType string, handler func(string, map[string]any)) {
	a.bus.SubscribeBroadcast(eventType, func(e eventbus.Event) { handler(e.Type, e.Payload) })
}

func (a realBusAdapter) SubscribeBroadcastContext(eventType string, handler func(context.Context, string, map[string]any)) {
	a.bus.SubscribeBroadcast(eventType, func(e eventbus.Event) { handler(e.Ctx, e.Type, e.Payload) })
}

// 真 eventbus + 真 NATS：user.created 与 user.registered 双发时，
// 插件的 user.registered 钩子只被调用一次，且 payload 原样到达。
func TestDispatcherUserRegisteredFiresOnceOverRealEventbus(t *testing.T) {
	bus := testnats.NewEventBus(t)

	var calls atomic.Int64
	gotPayload := make(chan map[string]any, 4)

	d := newDispatcher()
	d.RegisterPluginWithCaller("reg", &Manifest{Name: "reg", Hooks: []string{HookUserRegistered}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			calls.Add(1)
			if p, ok := params.(map[string]any); ok {
				select {
				case gotPayload <- p:
				default:
				}
			}
			return []byte(`{}`), nil
		})

	d.SubscribeEvents(realBusAdapter{bus: bus})

	// auth.go 注册成功路径：两条事件都发布。
	bus.Publish(eventbus.Event{Type: eventbus.EventUserCreated, Payload: map[string]any{"user_id": "u1"}})
	bus.Publish(eventbus.Event{Type: eventbus.EventUserRegistered, Payload: map[string]any{"user_id": "u1"}})

	select {
	case p := <-gotPayload:
		require.Equal(t, "u1", p["user_id"], "payload 必须穿过 eventbus 与适配层原样到达")
	case <-time.After(5 * time.Second):
		t.Fatal("user.registered 钩子没有被触发")
	}

	// 双发若两条都命中，第二次调用会在这段时间内到达。
	time.Sleep(400 * time.Millisecond)
	require.Equal(t, int64(1), calls.Load(), "一次注册必须恰好投递一次 user.registered")
}

// ---------------------------------------------------------------------------
// 并发安全
// ---------------------------------------------------------------------------

// 分发与注册并发进行时不得数据竞争（-race 下才有意义），
// 且分发必须对插件表取快照后在锁外调用。
func TestDispatcherConcurrentDispatchAndRegistration(t *testing.T) {
	d := newDispatcher()
	var called atomic.Int64
	caller := func(ctx context.Context, method string, params any) ([]byte, error) {
		called.Add(1)
		return []byte(`{}`), nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			d.Dispatch(context.Background(), HookPostCreated, nil)
		}()
		go func() {
			defer wg.Done()
			d.RegisterPluginWithCaller("p", &Manifest{Name: "p", Hooks: []string{HookPostCreated}}, caller)
			d.UnregisterPlugin("p")
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 真实 IPC 往返：分发经 Conn.Call 打到对端插件
// ---------------------------------------------------------------------------

// 端到端：dispatcher 用 supervisorCaller 包住的真实 Conn，方法名与 payload
// 都要落到插件的 handler 上，返回值不计（钩子不消费结果）。
func TestDispatcherReachesRealPluginConn(t *testing.T) {
	serverConn, pluginConn := newPipePairForTest(t)

	var mu sync.Mutex
	var seenMethod string
	var seenParams map[string]any
	pluginConn.Handle("hook.post.created", func(ctx context.Context, params json.RawMessage) (any, error) {
		var in map[string]any
		require.NoError(t, json.Unmarshal(params, &in))
		mu.Lock()
		seenMethod = "hook.post.created"
		seenParams = in
		mu.Unlock()
		return map[string]any{"ok": true}, nil
	})

	caller := func(ctx context.Context, method string, params any) ([]byte, error) {
		return serverConn.Call(ctx, method, params)
	}

	d := newDispatcher()
	d.RegisterPluginWithCaller("real", &Manifest{Name: "real", Hooks: []string{HookPostCreated}}, caller)
	d.Dispatch(context.Background(), HookPostCreated, map[string]any{"post_id": "p7"})

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return seenMethod != ""
	}, 2*time.Second, 10*time.Millisecond, "钩子必须真的走完一次 IPC 往返")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, "p7", seenParams["post_id"])
}

// supervisorCaller 在「没有连接」时必须立刻返回错误而不是 panic 或等超时。
func TestSupervisorCallerWithoutRunningSupervisor(t *testing.T) {
	call := supervisorCaller(nil)
	_, err := call(context.Background(), "hook.post.created", nil)
	require.ErrorIs(t, err, errPluginNotRunning)

	// 未 Start 的 supervisor：Conn() 返回 nil，同样要立刻报「未运行」。
	call = supervisorCaller(NewSupervisor(SupervisorOptions{PluginDir: t.TempDir()}))
	_, err = call(context.Background(), "hook.post.created", nil)
	require.ErrorIs(t, err, errPluginNotRunning)
}

// newPipePairForTest 返回一对已接好的 Conn（与 ipc_test.go 的外部测试版本同构，
// 这里需要包内版本以便与 dispatcher 一起测）。
func newPipePairForTest(t *testing.T) (*Conn, *Conn) {
	t.Helper()
	c1, c2 := net.Pipe()
	a := NewConn(c1, c1)
	b := NewConn(c2, c2)
	require.NoError(t, a.Start())
	require.NoError(t, b.Start())
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	return a, b
}

// supervisorCaller 必须**每次**重新取当前代连接，而不是缓存注册时的那个。
//
// 插件崩溃重启后新一代有自己的 Conn，旧的是 ErrClosed；若把 Conn 缓存在
// register 时，之后每一次钩子都会拿到 ErrClosed，而状态面板显示「运行中」——
// Task 4 用 TestSupervisorConnReturnsCurrentGeneration 钉住了 supervisor 侧，
// 这里钉住消费侧：分发器必须走 Conn() 而不是捕获的指针。
func TestSupervisorCallerFollowsConnectionGeneration(t *testing.T) {
	sup := NewSupervisor(SupervisorOptions{PluginDir: t.TempDir()})

	// 第一代：应答 ok:1
	gen1Server, gen1Plugin := newPipePairForTest(t)
	gen1Plugin.Handle("hook.post.created", func(ctx context.Context, params json.RawMessage) (any, error) {
		return map[string]any{"gen": 1}, nil
	})
	sup.mu.Lock()
	sup.conn = gen1Server
	sup.mu.Unlock()

	call := supervisorCaller(sup)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, err := call(ctx, "hook.post.created", nil)
	require.NoError(t, err)
	require.Contains(t, string(raw), "1")

	// 换代：旧连接关闭，换成第二代。
	_ = gen1Server.Close()
	gen2Server, gen2Plugin := newPipePairForTest(t)
	gen2Plugin.Handle("hook.post.created", func(ctx context.Context, params json.RawMessage) (any, error) {
		return map[string]any{"gen": 2}, nil
	})
	sup.mu.Lock()
	sup.conn = gen2Server
	sup.mu.Unlock()

	// 同一个 caller（注册时创建的那个）必须走新连接；缓存旧指针的话这里拿到 ErrClosed。
	raw, err = call(ctx, "hook.post.created", nil)
	require.NoError(t, err, "换代后必须使用新连接，而不是缓存的第一代")
	require.Contains(t, string(raw), "2")
}

// 在 supervisor 的当前代连接已被关闭时（崩溃但尚未重启），
// 调用必须尽快以 ErrClosed 失败，而不是空等到 timeout。
func TestSupervisorCallerReportsClosedConnection(t *testing.T) {
	sup := NewSupervisor(SupervisorOptions{PluginDir: t.TempDir()})
	serverConn, peer := newPipePairForTest(t)
	_ = peer.Close()
	_ = serverConn.Close()
	sup.mu.Lock()
	sup.conn = serverConn
	sup.mu.Unlock()

	call := supervisorCaller(sup)
	start := time.Now()
	_, err := call(context.Background(), "hook.post.created", nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrClosed) || !errors.Is(err, context.DeadlineExceeded),
		"已关闭的连接必须立刻失败（实际 %v）", err)
	require.Less(t, time.Since(start), time.Second, "不得空等")
}

// 分发 goroutine 里的 panic 必须被收住：它在独立 goroutine 里，逃出去就是
// 整个宿主进程崩溃——那等于「一个插件失败拖垮站点」。
func TestDispatcherDispatchRecoversPanicInCaller(t *testing.T) {
	rec := newRecorder()
	d := newDispatcher()
	d.RegisterPluginWithCaller("boom", &Manifest{Name: "boom", Hooks: []string{HookPostCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			panic("插件 caller 炸了")
		})
	d.RegisterPluginWithCaller("ok", &Manifest{Name: "ok", Hooks: []string{HookPostCreated}}, rec.caller("ok"))

	require.NotPanics(t, func() {
		d.Dispatch(context.Background(), HookPostCreated, nil)
	})

	// panic 的插件不影响另一个插件。
	require.Eventually(t, func() bool { return rec.count("ok") == 1 }, 2*time.Second, 10*time.Millisecond)
}
