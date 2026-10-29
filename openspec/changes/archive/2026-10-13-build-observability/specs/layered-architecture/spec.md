# layered-architecture Specification (delta)

## ADDED Requirements

### Requirement: 关联上下文沿依赖链向下传递

trace 上下文 MUST 在网关/中间件层建立一次，并沿依赖链单向向下传递：中间件把 trace 写入 `request.Context()`，Handler MUST 以 `ctx.Request().Context()` 作为下游 ctx 来源，Service 与 Repository MUST 保持 ctx 为首参并继续向下游传递，不得中途替换。

请求路径上 MUST NOT 出现 `context.Background()` / `context.TODO()`；确需脱离请求生命周期的后台任务（flush、健康检查探活、ticker/worker）MUST 使用显式命名的独立 context，并自行作为新 trace 的起点。当前豁免例：`internal/handler/handler.go:85`（计数刷写）、`:116` 与 `:144`（健康检查超时）——其中健康检查仍 MUST NOT 借用请求 ctx。

#### Scenario: Handler 转发的 ctx 携带 trace

- WHEN Handler 调用 `svc.Method(ctx, ...)`
- THEN 该 ctx 来源 MUST 是 `ctx.Request().Context()`，使 Service 层 `logger.WithContext(ctx)` 能取到 `trace_id`

#### Scenario: Service 不自造 context

- WHEN Service 方法需要向下传递超时
- THEN MUST 基于入参 ctx 派生（`context.WithTimeout(ctx, ...)`），MUST NOT 以 `context.Background()` 新建而丢失 trace

#### Scenario: 后台 worker 自成根

- WHEN `webhook.StartWorker`（`internal/service/webhook.go:259`）或每日信任晋升 ticker 执行一轮任务
- THEN MUST 为该轮创建新的 trace 根，并使其日志与任何 HTTP 请求解耦

#### Scenario: EventBus 消费侧不以 Background ctx 起始

- WHEN NATS 消息进入订阅 handler
- THEN `Event.Ctx` MUST 由消息头恢复的 trace 构造，MUST NOT 保留当前的 `context.Background()` 写法（`internal/pkg/eventbus/eventbus.go:123`）
