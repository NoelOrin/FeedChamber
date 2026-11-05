package plugin

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Host 是插件子系统对外的唯一入口：托管子进程生命周期，并把钩子分发
// 委托给内嵌的 dispatcher。
//
// 分工刻意分两层：
//   - dispatcher（hooks.go）只负责「谁订阅了哪个钩子、怎么并发分发」；
//   - Host（本文件）负责「插件进程起停、能力授予、代际轮换」。
//
// 这样 dispatcher 可以在测试里塞假 caller，不必拉起真进程。
type Host struct {
	*dispatcher

	opts HostOptions

	mu     sync.Mutex
	supers map[string]*Supervisor
	// manifests 记住每个在跑插件的清单，供 Invoke/Dispatch 与状态查询使用。
	manifests map[string]*Manifest
}

// HostOptions 是 Host 的装配输入。
type HostOptions struct {
	// PluginRoot 是插件代码根目录（如 ./plugins）。每个插件在
	// <PluginRoot>/<pluginID>/ 下。
	PluginRoot string
	// NodePath 是 node 可执行文件路径，空则用 PATH 里的 node。
	NodePath string
	// MaxRestarts 是单插件连续崩溃的最大重启次数，0 用 supervisor 默认值。
	MaxRestarts int
	// OnStatus 在插件状态变化时回调，供调用方落库。
	//
	// 它在 supervisor 的 goroutine 上同步执行（见 SupervisorOptions.OnCrash），
	// 因此实现方若要做 IO 应当自己开 goroutine，否则会拖慢重启节奏。
	OnStatus func(pluginID, status, lastError string)
	// APIDepsFor 为某个插件构造宿主 API 依赖。
	//
	// 每次启动都会调用一次（包括崩溃重启后），因此实现方可以在这里
	// 按插件 ID 做 KV 前缀、审计归属等定制。
	APIDepsFor func(pluginID string, caps CapabilitySet) APIDeps
}

// NewHostWithOptions 构造 Host。它不拉起任何进程，也不订阅事件——
// 那是 StartPlugin / SubscribeEvents 的事。这与 container.New 的
// 「只构造不启动」约定一致。
func NewHostWithOptions(opts HostOptions) *Host {
	return &Host{
		dispatcher: newDispatcher(),
		opts:       opts,
		supers:     make(map[string]*Supervisor),
		manifests:  make(map[string]*Manifest),
	}
}

// HasAPIDeps 报告宿主是否已接上真实 service 适配。
//
// 这不是给运行时用的，而是给 composition root 的装配断言用：
// StartPlugin 里 `if APIDepsFor != nil` 一旦漏接线，9 个 api.* 方法全是
// 空壳，症状是「插件显示 running 但一调宿主 API 就 not wired」，且不会
// 让任何现有测试变红。
func (h *Host) HasAPIDeps() bool {
	return h.opts.APIDepsFor != nil
}

// StartPlugin 拉起一个插件并注册它的钩子。
//
// 重复启动同一插件返回错误而不是静默覆盖：静默覆盖会让旧进程变成
// 无人回收的孤儿，而它的 Conn 还挂在 dispatcher 里。
func (h *Host) StartPlugin(ctx context.Context, pluginID string, m *Manifest, caps CapabilitySet) error {
	if m == nil {
		return fmt.Errorf("plugin: %s 的 manifest 为空", pluginID)
	}
	if pluginID == "" {
		return fmt.Errorf("plugin: 插件 ID 不能为空")
	}

	h.mu.Lock()
	if _, exists := h.supers[pluginID]; exists {
		h.mu.Unlock()
		return fmt.Errorf("plugin: %s 已在运行", pluginID)
	}
	// 占位，防止并发 StartPlugin 同一 ID 时两个 goroutine 都通过上面的检查。
	h.supers[pluginID] = nil
	h.mu.Unlock()

	fail := func(err error) error {
		h.mu.Lock()
		delete(h.supers, pluginID)
		h.mu.Unlock()
		h.reportStatus(pluginID, "crashed", err.Error())
		return err
	}

	sup := NewSupervisor(SupervisorOptions{
		NodePath:    h.opts.NodePath,
		PluginDir:   filepath.Join(h.opts.PluginRoot, pluginID),
		Entry:       m.Entry,
		MaxRestarts: h.opts.MaxRestarts,
		OnCrash: func(reason string) {
			h.reportStatus(pluginID, "crashed", reason)
		},
	})

	conn, err := sup.Start(ctx)
	if err != nil {
		return fail(fmt.Errorf("plugin: 启动 %s 失败: %w", pluginID, err))
	}

	if h.opts.APIDepsFor != nil {
		RegisterAPIs(conn, pluginID, caps, h.opts.APIDepsFor(pluginID, caps))
	}

	h.mu.Lock()
	h.supers[pluginID] = sup
	h.manifests[pluginID] = m
	h.mu.Unlock()

	// 注册钩子。dispatcher 每次分发时都会重新取 sup.Conn()，
	// 所以这里传入 sup 而不是某一代 conn——崩溃重启后仍然可用。
	h.RegisterPlugin(pluginID, m, sup)

	h.reportStatus(pluginID, "running", "")
	logrus.Infof("plugin: %s 已启动（钩子 %v，能力 %d 项）", pluginID, m.Hooks, len(caps))
	return nil
}

// StopPlugin 停止插件并摘除钩子。未在运行的插件视为已停止，返回 nil
// （调用方在关停路径上不该因为「本来就没起」而报错）。
func (h *Host) StopPlugin(pluginID string) error {
	h.mu.Lock()
	sup := h.supers[pluginID]
	delete(h.supers, pluginID)
	delete(h.manifests, pluginID)
	h.mu.Unlock()

	// 先摘钩子再停进程：反过来的话，进程已死但钩子还在，分发会撞上死 Conn。
	h.UnregisterPlugin(pluginID)

	if sup != nil {
		if err := sup.Stop(); err != nil {
			return fmt.Errorf("plugin: 停止 %s: %w", pluginID, err)
		}
	}
	h.reportStatus(pluginID, "stopped", "")
	return nil
}

// StopAll 停止全部插件，供宿主关停时调用。
//
// 逐个停而不是并发停：Stop 要等 supervisor 的 watch goroutine 退出，
// 并发停会把关停路径变成一场竞态，而顺序停最多多花几十毫秒。
func (h *Host) StopAll() {
	h.mu.Lock()
	ids := make([]string, 0, len(h.supers))
	for id := range h.supers {
		ids = append(ids, id)
	}
	h.mu.Unlock()

	for _, id := range ids {
		if err := h.StopPlugin(id); err != nil {
			logrus.WithError(err).Warnf("plugin: 关停 %s 时出错", id)
		}
	}
}

// Running 报告某插件是否在运行。
func (h *Host) Running(pluginID string) bool {
	h.mu.Lock()
	sup := h.supers[pluginID]
	h.mu.Unlock()
	return sup != nil && sup.Running()
}

// Invoke 直接调用某个插件的一个方法（不走钩子订阅）。供后台「测试连接」用。
func (h *Host) Invoke(ctx context.Context, pluginID, method string, params any) ([]byte, error) {
	h.mu.Lock()
	sup := h.supers[pluginID]
	h.mu.Unlock()
	if sup == nil {
		return nil, fmt.Errorf("plugin: %s 未在运行", pluginID)
	}
	conn := sup.Conn()
	if conn == nil {
		return nil, fmt.Errorf("plugin: %s 没有可用连接", pluginID)
	}
	return conn.Call(ctx, method, params)
}

// reportStatus 把状态变化转给调用方。OnStatus 未接线时静默跳过。
func (h *Host) reportStatus(pluginID, status, lastError string) {
	if h.opts.OnStatus == nil {
		return
	}
	h.opts.OnStatus(pluginID, status, lastError)
}

// ErrPluginNotFound 供上层区分「插件不存在」与「插件启动失败」。
var ErrPluginNotFound = errors.New("plugin not found")

// StartDailyHookTicker 启动每日钩子触发，返回取消函数。
//
// `cron.daily` 没有事件源（它不在 hookEvents 里），因此必须由 ticker 直接
// DispatchHook。不给它接 ticker 的话，插件声明了该钩子却永远收不到调用——
// 而且症状是静默的（AGENTS.md 里明确记着这条，见「钩子名 ≠ 事件名」）。
//
// 首次触发延迟 2 分钟：与 TrustService 的每日重算保持同一节奏，避免
// 启动瞬间所有定时任务一起压上来。
func (h *Host) StartDailyHookTicker(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		timer := time.NewTimer(2 * time.Minute)
		ticker := time.NewTicker(24 * time.Hour)
		defer timer.Stop()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				h.dispatchDaily()
			case <-ticker.C:
				h.dispatchDaily()
			}
		}
	}()
	return cancel
}

func (h *Host) dispatchDaily() {
	// 用 Background 而不是 ticker 的 ctx：钩子不该因为宿主正在关停而被掐断，
	// 关停顺序由 StopAll 负责。
	h.Dispatch(context.Background(), HookCronDaily, map[string]any{
		"trigger": "daily",
	})
}
