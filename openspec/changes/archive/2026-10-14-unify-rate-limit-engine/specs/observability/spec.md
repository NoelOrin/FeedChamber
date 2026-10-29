# observability Specification (delta)

## MODIFIED Requirements

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
| `feedchamber_ratelimit_rejections_total` | Counter | `name` | `internal/pkg/ratelimit` 的 `Limiter.Check` 超限分支（唯一发出点；两类调用方共用） |
| `feedchamber_ratelimit_failopen_total` | Counter | `name` | 同上的 `recordFailure`（KV SetNX/Incr 失败，两处出口合一） |
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

## ADDED Requirements

### Requirement: 限流只有一个引擎

HTTP 侧的限流 MUST 全部经 `internal/pkg/ratelimit` 的 `Limiter.Check`：滑动窗口算法（SetNX 起窗 + Incr 计数 + 比较）、计数器键形状、指标发出与 fail-open 日志 MUST 只有这一处实现。`pkg/gateway` 的全局每 IP 上限与 `internal/middleware` 的每端点预算 MUST 只作为该引擎的两个调用方存在，差别仅限"策略从哪里来"（`rate_limit_enabled` 设置开关 vs 路由注册时声明）。

策略名 MUST 共用一个标签值空间并在 `feedchamber_ratelimit_*{name}` 中可读：全局策略为 `global`，其余为端点名（`login`、`register`、`post_create` …）。新增策略 MUST NOT 另开家族或另开标签。

计数器键形状是契约的一部分：`rl:<ip>:<policy>`（端点）与 `rl:ratelimit:<ip>`（全局）由 `EndpointKey`/`GlobalKey` 单点产生并有测试钉住——改形状等于清零正在计数的窗口，使上线瞬间所有客户端获得新配额。

#### Scenario: 两类策略共用失败语义

- WHEN KV 后端读失败
- THEN 两个调用方 MUST 都放行并各记一次 `feedchamber_ratelimit_failopen_total{name}`，MUST NOT 出现"一个 fail-open、另一个 fail-close"的分歧

#### Scenario: 端点预算不占用全局配额

- WHEN 某端点预算（如 `login` 10/分钟）被打满
- THEN 全局每 IP 计数器 MUST NOT 因此多计，二者的键与配额相互独立
