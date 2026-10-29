package container

import (
	"context"
	"fmt"

	"feedchamber/internal/service"
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

	// 「发布后异步审核」（moderation_mode=async）消费发帖事件；external 模式
	// 在外部服务不可用时也走这条兜底路径。订阅本身不产生 goroutine，
	// 消费在 EventBus 自己的 worker 池里跑。
	c.ModerationSvc.SubscribeAsync(c.EventBus)

	return r, nil
}

// Shutdown drains buffered state, then stops the workers and the bus. Order
// matters: view counts live in KV and must reach SQL before the caller closes
// KV and the DB handle.
func (r *Runtime) Shutdown() {
	if r == nil {
		return
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
