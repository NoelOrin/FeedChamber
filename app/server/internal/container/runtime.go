package container

import (
	"context"
	"fmt"

	"feedchamber/internal/plugin"
	"feedchamber/internal/plugin/pluginevent"
	"feedchamber/internal/service"

	"github.com/sirupsen/logrus"
)

// Runtime owns everything that exists only after the graph is built: event
// subscriptions and background workers. Create it with Container.Start and call
// Shutdown on exit — a Container that was never started spawns nothing, which is
// what makes it safe to construct inside tests.
type Runtime struct {
	c               *Container
	notifDispatcher *service.NotifDispatcher
	// workers is keyed by name so a shutdown that hangs is attributable in logs.
	workers map[string]context.CancelFunc
}

// Start subscribes to the event bus and launches the tickers and worker pools.
// ctx is the parent for every worker; cancelling it stops all of them at once.
func (c *Container) Start(ctx context.Context) (*Runtime, error) {
	r := &Runtime{c: c, workers: map[string]context.CancelFunc{}}

	// Seed the default badge set. The call is idempotent, but it writes, so a
	// failure has to reach the operator instead of being discarded in a
	// constructor the way `_ = SeedDefaults(ctx)` did.
	if err := c.BadgeSvc.SeedDefaults(ctx); err != nil {
		return nil, fmt.Errorf("seed default badges: %w", err)
	}

	r.notifDispatcher = service.SubscribeNotifications(c.EventBus, c.NotificationSvc)
	c.WebhookSvc.Subscribe(c.EventBus)

	r.workers["view_count_flusher"] = c.PostSvc.StartViewCountFlusher(ctx)
	r.workers["pin_expiry_checker"] = c.PostSvc.StartPinExpiryChecker(ctx)
	r.workers["webhook_delivery"] = c.WebhookSvc.StartWorker(ctx)
	r.workers["weekly_digest"] = c.DigestSvc.StartWeeklyTicker(ctx)
	r.workers["trust_recompute"] = c.TrustSvc.StartDailyRecomputeTicker(ctx)
	if err := c.GateSvc.StartSyncSubscriber(ctx); err != nil {
		return nil, fmt.Errorf("start gate sync subscriber: %w", err)
	}
	if err := c.TrustSvc.StartSyncSubscriber(ctx); err != nil {
		return nil, fmt.Errorf("start trust sync subscriber: %w", err)
	}
	r.workers["motion_resolver"] = c.MotionSvc.StartResolveTicker(ctx)
	// 插件 cron.daily 没有事件源（不在 hookEvents 里），必须由 ticker 直接触发。
	r.workers["plugin_cron_daily"] = c.PluginHost.StartDailyHookTicker(ctx)

	// 「发布后异步审核」（moderation_mode=async）消费发帖事件；external 模式
	// 在外部服务不可用时也走这条兜底路径。订阅本身不产生 goroutine，
	// 消费在 EventBus 自己的 worker 池里跑。
	c.ModerationSvc.SubscribeAsync(c.EventBus)

	// 插件事件桥接。漏了这一步不会编译报错，症状是插件显示「运行中」
	// 但钩子永不触发——所以这里必须显式接线，并有 e2e 验收兜底。
	c.PluginHost.SubscribeEvents(pluginevent.Broadcaster(c.EventBus))

	// 恢复重启前处于 running 的插件。crashed / stopped 不自动拉起：
	// 前者已经在退避里放弃，后者是管理员显式停的。
	if err := c.resumePlugins(ctx); err != nil {
		// 恢复失败不阻断启动：少起几个插件比整个站点起不来好。
		logrus.WithError(err).Warn("plugin: 恢复插件时出错")
	}

	return r, nil
}

// resumePlugins 拉起状态为 running 的插件。
func (c *Container) resumePlugins(ctx context.Context) error {
	items, err := c.PluginSvc.ListResumable(ctx)
	if err != nil {
		return fmt.Errorf("list resumable plugins: %w", err)
	}
	for _, in := range items {
		m, mErr := plugin.ParseManifest([]byte(in.Manifest))
		if mErr != nil {
			logrus.WithError(mErr).Warnf("plugin: %s 的 manifest 无法解析，跳过恢复", in.PluginID)
			continue
		}
		caps := plugin.CapabilitySetFromNames(in.Capabilities)
		if sErr := c.PluginHost.StartPlugin(ctx, in.PluginID, m, caps); sErr != nil {
			logrus.WithError(sErr).Warnf("plugin: 恢复 %s 失败", in.PluginID)
			continue
		}
	}
	return nil
}

// Shutdown drains buffered state, then stops the workers and the bus. Order
// matters: view counts live in KV and must reach SQL before the caller closes
// KV and the DB handle.
func (r *Runtime) Shutdown() {
	if r == nil {
		return
	}
	// 插件最先停：停一个插件会关掉它的 Conn，若等到 EventBus/KV 关完再停，
	// 插件在途的宿主 API 调用会撞上已关闭的 KV 与总线。
	if r.c.PluginHost != nil {
		r.c.PluginHost.StopAll()
	}

	r.c.PostSvc.FlushViewCounts(context.Background())

	for name, cancel := range r.workers {
		if cancel != nil {
			cancel()
		}
		delete(r.workers, name)
	}

	r.c.EventBus.Shutdown()
	if r.notifDispatcher != nil {
		r.notifDispatcher.Shutdown()
	}
	service.ShutdownNotificationEmailDispatcher(r.c.NotificationSvc)
}
