// Package pluginevent 把 eventbus 的广播事件适配成 plugin.Broadcaster。
//
// 它独立成包，是为了保住 `internal/plugin` 的分层约束：
// eventbus → internal/metrics → internal/db → gorm，而 `internal/plugin`
// MUST NOT 持有 *gorm.DB、MUST NOT 引用 internal/repository。直接把
// eventbus 引进 plugin 包，会让 plugin 的**生产**依赖图里凭空多出 10 个
// gorm 包（基线是 0），分层约束就只剩一句文档承诺。
//
// 接缝留在 plugin（Broadcaster / TraceBroadcaster），真实事件总线在这里接线，
// 由 composition root 决定用哪个——这正是「只消费注入的接口」的意思。
package pluginevent

import (
	"context"

	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/plugin"
)

// Broadcaster 把 *eventbus.EventBus 适配成 plugin.Broadcaster。
//
// 返回的类型同时实现 plugin.TraceBroadcaster：eventbus 的广播 handler 拿得到
// 带 traceparent 的 Event.Ctx，用 ctx 版本能让插件钩子的日志继续带 trace_id。
// 只实现二元签名会静默丢掉 trace。
func Broadcaster(bus *eventbus.EventBus) plugin.Broadcaster {
	return adapter{bus: bus}
}

type adapter struct{ bus *eventbus.EventBus }

func (a adapter) SubscribeBroadcast(eventType string, handler func(string, map[string]any)) {
	if a.bus == nil || handler == nil {
		return
	}
	a.bus.SubscribeBroadcast(eventType, func(e eventbus.Event) {
		handler(e.Type, e.Payload)
	})
}

func (a adapter) SubscribeBroadcastContext(eventType string, handler func(context.Context, string, map[string]any)) {
	if a.bus == nil || handler == nil {
		return
	}
	a.bus.SubscribeBroadcast(eventType, func(e eventbus.Event) {
		handler(e.Ctx, e.Type, e.Payload)
	})
}

// 编译期确认：适配器同时满足两个接缝。
var (
	_ plugin.Broadcaster      = adapter{}
	_ plugin.TraceBroadcaster = adapter{}
)
