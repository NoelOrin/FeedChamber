# observability Specification (delta)

## ADDED Requirements

### Requirement: 关联标识与请求追踪

网关 MUST 为每个进入的 HTTP 请求确定 trace 上下文：优先接受上游 `traceparent` 头（W3C Trace Context Level-1，仅 `00` 版本），其 trace id 合法时沿用并新建 span id；缺失或不合法时 MUST 生成 128-bit 随机 trace id 与 64-bit span id。trace 上下文 MUST 写入 `request.Context()`，响应 MUST 回写 `traceparent` 与 `X-Request-ID`（后者取值等于 trace id，头名保持不变以兼容既有客户端）。

`pkg/gateway/tracing.go` 的空 TODO 实现 MUST 被替换为上述真实实现，其在流水线首位的调用位置（`gateway.go:45`、`gateway.go:94`）保持不变。

#### Scenario: 无入站 traceparent

- WHEN 请求不携带 `traceparent` 头
- THEN 网关 MUST 生成新 trace id 与 span id，并在响应头回写 `traceparent: 00-<trace-id>-<span-id>-01`

#### Scenario: 沿用上游 trace

- WHEN 请求携带合法的 `traceparent`（版本 `00`、32 位十六进制 trace id、16 位十六进制 span id、非全零）
- THEN trace id MUST 沿用入站值、span id MUST 新生成，且入站 span id 记为该 span 的 parent

#### Scenario: 拒绝畸形 traceparent

- WHEN `traceparent` 存在但格式不合法（版本非 `00`、长度错误、含非十六进制字符、trace id 全零）
- THEN 系统 MUST 忽略该头并生成全新 trace，且 MUST NOT panic 或因此拒绝请求

#### Scenario: 追踪开关

- WHEN `TRACING_ENABLED=false`
- THEN 系统 MUST 仍产生用于日志关联的 trace id 并写入 context，仅停止解析入站 `traceparent` 与回写响应头

### Requirement: 追踪上下文经 NATS 传播

EventBus 发布 MUST 通过 `nc.PublishMsg`（nats.go v1.39.1 无 `MsgPublish`）在消息头携带 `traceparent`；消费侧 MUST 从 `msg.Header` 恢复 trace 上下文，并将带该上下文的 `context.Context` 放入 `Event.Ctx`，取代当前硬编码的 `context.Background()`（`internal/pkg/eventbus/eventbus.go:123`）。恢复时 MUST NOT 传递发布方的 ctx：`Publish` 现有代码在 `:164` 主动把 `Event.Ctx` 重置为 background，注释说明是为了避免请求结束后 ctx 被取消导致通知静默失败，因此只允许把 trace **值**放到新的 background ctx 上。头部缺失或不合法时消费侧 MUST 新建独立 trace 而非失败。

#### Scenario: 事件跨实例携带同一 trace

- WHEN 实例 A 在带 trace T 的请求内 `Publish(EventPostCreated)`，实例 B 经 NATS 收到
- THEN B 的订阅 handler MUST 拿到 trace id 为 T 的 `Event.Ctx`，其日志可与 A 的请求串联

#### Scenario: 旧版本消息无头部

- WHEN 收到的 NATS 消息不含 `traceparent` 头（来自未升级实例或外部发布方）
- THEN 消费 MUST 正常执行并生成新的 trace id，MUST NOT 丢弃消息

### Requirement: 日志输出可配置

`internal/config` MUST 读取 `LOG_LEVEL`（默认 `info`）、`LOG_FORMAT`（默认 `json`，可选 `text`）、`LOG_OUTPUT`（默认 `stderr`，可为文件路径），聚合为 logger 配置。`Start(env)` MUST 在 `cfg.Validate()` 之后、任何业务日志之前调用 `logger.SetGlobal(logger.New(cfg.Log))`，使 `logger.Config` 在生产路径上实际生效。

#### Scenario: 调整日志级别

- WHEN 以 `LOG_LEVEL=warn` 启动
- THEN 请求完成这类 info 级日志 MUST NOT 出现在输出中
- WHEN 以默认 `LOG_FORMAT=json` 之外的 `LOG_FORMAT=text` 启动
- THEN 输出 MUST 为 logrus 文本格式而非 JSON

> 实现期发现：仓库非测试代码中 debug 级调用点为 0，因此「`LOG_LEVEL=debug` 时出现 debug 行」在当前代码下无可观测对象，验收改用上面的 warn 反向判据。

#### Scenario: 级别取值非法

- WHEN `LOG_LEVEL` 为无法解析的字符串
- THEN 系统 MUST 回退到 `info` 并继续启动，MUST NOT 中断

### Requirement: 请求日志字段

请求完成日志 MUST 输出：`trace_id`、`span_id`、`method`、`path`、`status`、`latency_ms`、`user_id`、`ip`。原 `request_id` 字段 MUST 更名为 `trace_id`，且全链路仅保留这一个关联 id，MUST NOT 同时输出两个不同取值的 id。

#### Scenario: Service 层日志可关联

- WHEN Service 或 Repository 层用 `logger.FromContext(ctx)`（ctx 为请求 context）输出一条日志
- THEN 该日志 MUST 携带与该请求相同的 `trace_id`

### Requirement: 业务指标覆盖

系统 MUST 在 `internal/metrics.Registry` 上注册下列业务指标族（名称带 `feedchamber_` 前缀），埋点位置为各子系统唯一的判定/交接点，MUST NOT 在调用方重复计数：

| 指标 | 类型 | 标签 | 记录点 |
| --- | --- | --- | --- |
| `feedchamber_eventbus_publish_total` | Counter | `event`,`result` | `eventbus.go:177` 发布成功/失败分支 |
| `feedchamber_eventbus_dispatch_duration_seconds` | Histogram | `event` | `dispatch`（`eventbus.go:78`）逐 handler |
| `feedchamber_eventbus_dispatch_errors_total` | Counter | `event` | 既有 panic 恢复处（`eventbus.go:84-88`） |
| `feedchamber_eventbus_queue_depth` | Gauge | — | `enqueue`/消费循环（`eventbus.go:127`） |
| `feedchamber_eventbus_dropped_total` | Counter | `event` | 缓冲满静默丢弃分支（`eventbus.go:131`） |
| `feedchamber_kv_operation_duration_seconds` | Histogram | `op`,`outcome` | `kvstore/nats.go` 各方法出口（`:207`/`:223`/`:237`/`:260`/`:269`/`:285`/`:312`/`:376`） |
| `feedchamber_kv_cas_exhausted_total` | Counter | — | `incrCAS` 重试耗尽（`nats.go:369`） |
| `feedchamber_ratelimit_rejections_total` | Counter | `name` | `pkg/gateway/ratelimit.go:93` |
| `feedchamber_ratelimit_failopen_total` | Counter | `name` | `pkg/gateway/ratelimit.go:83,89` |
| `feedchamber_permission_decisions_total` | Counter | `scope`,`result` | `service/permission.go:217-226`、`:255` |
| `feedchamber_sse_connections` | Gauge | `channel` | `handler/notification.go:186`、`handler/realtime.go:29` 建连后递增、`defer` 递减 |
| `feedchamber_login_failures_total` | Counter | `reason` | `service/auth.go:283`/`:294`/`:303`（`unknown_user`/`no_credential`/`bad_password`） |
| `feedchamber_webhook_deliveries_total` | Counter | `outcome` | `service/webhook.go:327` `deliver` 出口；终态失败记 `failed_terminal`（`:295`） |
| `feedchamber_webhook_delivery_duration_seconds` | Histogram | — | `service/webhook.go:327`（已有 `start`/`LatencyMs`） |
| `feedchamber_webhook_pending_deliveries` | Gauge | — | `StartWorker` 每 tick（`webhook.go:261-272`） |

HTTP 与 DB 池既有指标（`metrics.go:18-55`、`collector.go:23`）保持不变。

#### Scenario: 登录失败区分原因

- WHEN 登录因用户不存在、凭证缺失、密码不匹配三种情况分别失败
- THEN `feedchamber_login_failures_total` MUST 递增到三个不同的 `reason` 取值，MUST NOT 合并为单一计数

#### Scenario: 已有安全机制不重复埋点

- WHEN 失败登录同时触发 `SecurityManager.RecordFailedLogin` 的暴力破解阈值事件
- THEN 该事件 MUST 沿用现有 `model.SecurityEvent` 持久化，MUST NOT 为其新增独立指标族

#### Scenario: handler panic 计入指标且不扩散

- WHEN 某事件的订阅 handler 发生 panic
- THEN `feedchamber_eventbus_dispatch_errors_total{event}` MUST +1，恢复逻辑与其它 handler 的执行 MUST NOT 受影响

### Requirement: 指标暴露面与访问边界

`/metrics` MUST 从 `/api/v1` 业务路由移除（现为 `internal/router/router.go:119` 的 admin 门控路由），改由独立监听地址 `METRICS_PORT`（默认 9091）暴露，MUST NOT 出现在 nginx 对外 upstream 中。`METRICS_ENABLED=false` 时该 listener 与 Prometheus 中间件 MUST NOT 注册。

#### Scenario: 内部抓取可用

- WHEN Prometheus 访问 `http://backend:9091/metrics`
- THEN MUST 无鉴权返回 Prometheus 文本格式，且包含上述全部指标族

#### Scenario: 对外不可达

- WHEN 客户端经 nginx 请求 `/api/v1/metrics` 或任何 `/metrics` 路径
- THEN MUST 返回 404，MUST NOT 泄露 DB 连接池与内部指标

### Requirement: 监控栈可部署

`deploy/docker-compose.cluster.yml` 引用的每个挂载文件 MUST 在仓库中存在。系统 MUST 新增 `deploy/prometheus.yml`（抓取 backend 的 `METRICS_PORT`，并通过 `rule_files` 引入告警文件）、`deploy/prometheus/rules/feedchamber.yml` 与 `deploy/grafana/provisioning/`（数据源 + dashboard 自动装配）。Grafana 管理员口令 MUST 由环境变量注入，MUST NOT 硬编码。

#### Scenario: 配置校验通过

- WHEN 在 `app/server/deploy/` 执行 `docker compose -f docker-compose.cluster.yml config`
- THEN MUST 无「挂载文件不存在」错误（该校验不需要 Docker daemon 在跑）

### Requirement: 告警规则最低集

`deploy/prometheus/rules/feedchamber.yml` MUST 至少定义 8 条告警：5xx 错误率过高、P99 请求延迟过高、DB 连接池饱和（`in_use` 逼近 `max_open` 或 `wait_count` 增长）、KV 操作延迟异常、EventBus 出现丢弃、webhook 终态失败、登录失败突增、`/api/v1/health` 探测失败。

#### Scenario: 丢弃事件触发告警

- WHEN `feedchamber_eventbus_dropped_total` 在 5 分钟内增长大于 0
- THEN MUST 产生一条可定位到事件类型的告警

### Requirement: 指标基数护栏

Prometheus 标签取值 MUST 有界：允许 `method`、归一化 `path`（沿用 `middleware/prometheus.go:39-56` 的 ID 折叠）、`status`、事件类型常量、KV 操作名、判定结果、有界角色名。trace id、span id、用户 id、原始 URL 段、自由文本与错误消息 MUST NOT 作为标签。

#### Scenario: 追踪 id 不进入指标标签

- WHEN 请求携带任意 trace id
- THEN 该 id MUST 只出现在日志与响应头中，MUST NOT 出现在任何指标的标签里
