# observability Specification

## Purpose
定义请求关联标识（W3C traceparent）、结构化日志、Prometheus 指标与监控栈编排的约定，使一次请求在网关、handler、service、repository 与后台消费链路中共享同一个关联 id，且指标标签始终保持有界。

## Requirements
### Requirement: 关联标识与请求追踪

网关 MUST 为每个进入的 HTTP 请求确定 trace 上下文：优先接受上游 `traceparent` 头（W3C Trace Context Level-1，仅 `00` 版本），其 trace id 合法时沿用并新建 span id；缺失或不合法时 MUST 生成 128-bit 随机 trace id 与 64-bit span id。trace 上下文 MUST 写入 `request.Context()`，响应 MUST 回写 `traceparent` 与 `X-Request-ID`（后者取值等于 trace id，头名保持不变以兼容既有客户端）。

`internal/gateway/tracing.go` 的 `Tracing` 早先是空 TODO 实现却每请求调用，现 MUST 只做一件事：路由匹配后把端点模板写进 `trace.Name`（即 span 命名）。trace 本体 MUST 由 app 级 `internal/middleware.Trace` 建立——它必须先于 `Logger`，否则请求日志无 trace 可取。`Tracing` 在流水线里的首位调用位置 MUST 保持（由 `Gateway.preflight` 与组合入口 `Handler` 调用；跨包位置引用一律指符号，不指行号）。

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

系统 MUST 在 `internal/metrics.Registry` 上注册下列业务指标族（名称带 `orinforum_` 前缀），埋点位置为各子系统唯一的判定/交接点，MUST NOT 在调用方重复计数：

| 指标 | 类型 | 标签 | 记录点 |
| --- | --- | --- | --- |
| `orinforum_eventbus_publish_total` | Counter | `event`,`result` | `eventbus.go:177` 发布成功/失败分支 |
| `orinforum_eventbus_dispatch_duration_seconds` | Histogram | `event` | `dispatch`（`eventbus.go:78`）逐 handler |
| `orinforum_eventbus_dispatch_errors_total` | Counter | `event` | 既有 panic 恢复处（`eventbus.go:84-88`） |
| `orinforum_eventbus_queue_depth` | Gauge | — | `enqueue`/消费循环（`eventbus.go:127`） |
| `orinforum_eventbus_dropped_total` | Counter | `event` | 缓冲满静默丢弃分支（`eventbus.go:131`） |
| `orinforum_kv_operation_duration_seconds` | Histogram | `op`,`outcome` | `kvstore/nats.go` 各方法出口（`:207`/`:223`/`:237`/`:260`/`:269`/`:285`/`:312`/`:376`） |
| `orinforum_kv_cas_exhausted_total` | Counter | — | `incrCAS` 重试耗尽（`nats.go:369`） |
| `orinforum_ratelimit_rejections_total` | Counter | `name` | `internal/pkg/ratelimit` 的 `Limiter.Check` 超限分支（唯一发出点；两类调用方共用） |
| `orinforum_ratelimit_failopen_total` | Counter | `name` | 同上的 `recordFailure`（KV SetNX/Incr 失败，两处出口合一） |
| `orinforum_permission_decisions_total` | Counter | `scope`,`result` | `service/permission.go:217-226`、`:255` |
| `orinforum_sse_connections` | Gauge | `channel` | `handler/notification.go:186`、`handler/realtime.go:29` 建连后递增、`defer` 递减 |
| `orinforum_login_failures_total` | Counter | `reason` | `service/auth.go:283`/`:294`/`:303`（`unknown_user`/`no_credential`/`bad_password`） |
| `orinforum_webhook_deliveries_total` | Counter | `outcome` | `service/webhook.go:327` `deliver` 出口；终态失败记 `failed_terminal`（`:295`） |
| `orinforum_webhook_delivery_duration_seconds` | Histogram | — | `service/webhook.go:327`（已有 `start`/`LatencyMs`） |
| `orinforum_webhook_pending_deliveries` | Gauge | — | `StartWorker` 每 tick（`webhook.go:261-272`） |

HTTP 与 DB 池既有指标（`metrics.go:18-55`、`collector.go:23`）保持不变。

#### Scenario: 登录失败区分原因

- WHEN 登录因用户不存在、凭证缺失、密码不匹配三种情况分别失败
- THEN `orinforum_login_failures_total` MUST 递增到三个不同的 `reason` 取值，MUST NOT 合并为单一计数

#### Scenario: 已有安全机制不重复埋点

- WHEN 失败登录同时触发 `SecurityManager.RecordFailedLogin` 的暴力破解阈值事件
- THEN 该事件 MUST 沿用现有 `model.SecurityEvent` 持久化，MUST NOT 为其新增独立指标族

#### Scenario: handler panic 计入指标且不扩散

- WHEN 某事件的订阅 handler 发生 panic
- THEN `orinforum_eventbus_dispatch_errors_total{event}` MUST +1，恢复逻辑与其它 handler 的执行 MUST NOT 受影响

### Requirement: 指标暴露面与访问边界

`/metrics` MUST 从 `/api/v1` 业务路由移除（现为 `internal/router/router.go:119` 的 admin 门控路由），改由独立监听地址 `METRICS_PORT`（默认 9091）暴露，MUST NOT 出现在 nginx 对外 upstream 中。`METRICS_ENABLED=false` 时该 listener 与 Prometheus 中间件 MUST NOT 注册。

#### Scenario: 内部抓取可用

- WHEN Prometheus 访问 `http://backend:9091/metrics`
- THEN MUST 无鉴权返回 Prometheus 文本格式，且包含上述全部指标族

#### Scenario: 对外不可达

- WHEN 客户端经 nginx 请求 `/api/v1/metrics` 或任何 `/metrics` 路径
- THEN MUST 返回 404，MUST NOT 泄露 DB 连接池与内部指标

### Requirement: 告警规则最低集

`deploy/prometheus/rules/orinforum.yml` MUST 至少定义 8 条告警：5xx 错误率过高、P99 请求延迟过高、DB 连接池饱和（`in_use` 逼近 `max_open` 或 `wait_count` 增长）、KV 操作延迟异常、EventBus 出现丢弃、webhook 终态失败、登录失败突增、`/api/v1/health` 探测失败。

#### Scenario: 丢弃事件触发告警

- WHEN `orinforum_eventbus_dropped_total` 在 5 分钟内增长大于 0
- THEN MUST 产生一条可定位到事件类型的告警

### Requirement: 指标基数护栏

Prometheus 标签取值 MUST 有界：允许 `method`、归一化 `path`（沿用 `middleware/prometheus.go:39-56` 的 ID 折叠）、`status`、事件类型常量、KV 操作名、判定结果、有界角色名。trace id、span id、用户 id、原始 URL 段、自由文本与错误消息 MUST NOT 作为标签。

#### Scenario: 追踪 id 不进入指标标签

- WHEN 请求携带任意 trace id
- THEN 该 id MUST 只出现在日志与响应头中，MUST NOT 出现在任何指标的标签里

### Requirement: 限流只有一个引擎

HTTP 侧的限流 MUST 全部经 `internal/pkg/ratelimit` 的 `Limiter.Check`：滑动窗口算法（SetNX 起窗 + Incr 计数 + 比较）、计数器键形状、指标发出与 fail-open 日志 MUST 只有这一处实现。`internal/gateway` 的全局每 IP 上限与 `internal/middleware` 的每端点预算 MUST 只作为该引擎的两个调用方存在，差别仅限"策略从哪里来"（`rate_limit_enabled` 设置开关 vs 路由注册时声明）。

策略名 MUST 共用一个标签值空间并在 `orinforum_ratelimit_*{name}` 中可读：全局策略为 `global`，其余为端点名（`login`、`register`、`post_create` …）。新增策略 MUST NOT 另开家族或另开标签。

计数器键形状是契约的一部分：`rl:<ip>:<policy>`（端点）与 `rl:ratelimit:<ip>`（全局）由 `EndpointKey`/`GlobalKey` 单点产生并有测试钉住——改形状等于清零正在计数的窗口，使上线瞬间所有客户端获得新配额。

#### Scenario: 两类策略共用失败语义

- WHEN KV 后端读失败
- THEN 两个调用方 MUST 都放行并各记一次 `orinforum_ratelimit_failopen_total{name}`，MUST NOT 出现"一个 fail-open、另一个 fail-close"的分歧

#### Scenario: 端点预算不占用全局配额

- WHEN 某端点预算（如 `login` 10/分钟）被打满
- THEN 全局每 IP 计数器 MUST NOT 因此多计，二者的键与配额相互独立

### Requirement: NATS 连接状态可观测

KV Store 与 EventBus 各自持有一条 NATS 连接，两者都以无限重试策略连接（`nats.MaxReconnects(-1)`），断连后进程**不会退出**。因此连接状态 MUST 由指标表达，MUST NOT 只依赖日志：

| 指标 | 类型 | 标签 | 语义 |
| --- | --- | --- | --- |
| `orinforum_nats_connected` | Gauge | `client` | 1 = 连接可用，0 = 已断开 |
| `orinforum_nats_disconnects_total` | Counter | `client` | 从「可用」转为「断开」的次数 |
| `orinforum_nats_reconnects_total` | Counter | `client` | 重连成功的次数 |

`client` 取值 MUST 限于静态集合 `kv` / `eventbus`（符合「指标基数护栏」）。三族 MUST 在 `init()` 内以 `connected=1` seed，因为构造这两条连接的前提就是 `nats.Connect` 成功——若 seed 为 0，健康实例会立即触发断连告警。

两处 `nats.Connect` MUST 注册 `DisconnectErrHandler` 与 `ReconnectHandler`：前者置 0 并累加 `disconnects_total`，后者置 1 并累加 `reconnects_total`。断连计数 MUST NOT 在重连时回退或抵消——「断连数 ≫ 重连数」是链路抖动的唯一可判据。

#### Scenario: 网络分区可见

- WHEN 后端与 NATS 之间的网络中断且持续超过重连等待
- THEN `orinforum_nats_connected{client}` MUST 在 2 分钟内变为 0，并触发 `NATSDisconnected` 告警
- AND 该 client 的 `orinforum_nats_disconnects_total` MUST 恰好 +1

#### Scenario: 重连恢复

- WHEN NATS 恢复可达
- THEN `orinforum_nats_connected{client}` MUST 回到 1，`orinforum_nats_reconnects_total` +1
- AND `orinforum_nats_disconnects_total` MUST 保持不变（不得被抵消）

### Requirement: SSE 写失败与正常断开可区分

两个 SSE handler（`notification.Stream`、`realtime.Stream`）在向客户端写事件失败时 MUST 递增 `orinforum_sse_write_failures_total{channel}`（`channel` ∈ `notification` / `realtime`），并在写失败分支终止该流。

该指标 MUST 与 `orinforum_sse_connections` 分开：连接 gauge 的下降同时包含「客户端关标签页」这一正常事件，因而无法承担「投递失败」的语义。只增加连接 gauge 的精度而不新增失败计数，不能满足本要求。

#### Scenario: 写失败被计数

- WHEN 底层 `ResponseWriter` 的写操作返回错误
- THEN 对应 channel 的 `orinforum_sse_write_failures_total` MUST +1
- AND 连接 gauge MUST 按既有 `defer` 语义正常回落，两个数字 MUST NOT 相互推导

#### Scenario: 静默实例不误报

- WHEN 实例启动后从未发生写失败
- THEN 两个 channel 的失败序列 MUST 已存在且值为 0（由 `init()` seed）

### Requirement: 积压 gauge 反映真实深度

`orinforum_webhook_pending_deliveries` MUST 取真实的待投递总数（`COUNT(*)`），MUST NOT 取 worker 单批查询结果的长度。

worker 每 tick 用固定批大小取待投递行（当前 20），该长度是**吞吐参数**，与积压深度无关。用批大小充当积压会使 gauge 在深度超过批大小时饱和，使基于它的告警结构性不可触发。

计数与批查询 MUST 使用同一谓词（`status='pending' AND attempts<5`），并由测试钉住二者选中同一批行。`NextRetryAt` 在未来的行 MUST 计入积压——它们是尚未投递的工作，排除掉会让重试风暴看起来像空队列。

#### Scenario: 积压超过批大小

- WHEN 待投递行数为 35、worker 批大小为 20
- THEN `orinforum_webhook_pending_deliveries` MUST 为 35，MUST NOT 为 20
- AND `ListPendingDeliveries` MUST 仍只返回 20 行

### Requirement: 指标端口暴露面由自动化检查强制

`METRICS_PORT`（默认 9091）无鉴权，其安全性完全依赖两点：compose 只 `expose` 不 `ports`，且 nginx 不代理该端口。两者都是一行改动即可破坏，且编译器与测试都不会发现。

因此 MUST 存在一个自动化检查脚本，断言：

1. 任何 `deploy/docker-compose*.yml` 中，**没有任何服务**把 9091 发布到宿主机（`ports:` 短语法 `9091:9091` / `127.0.0.1:9091:9091`，或长语法 `target: 9091` / `published: 9091` 均算违规）；
2. `deploy/nginx/nginx.conf` 不出现 `9091`。

该检查 MUST 在 CI 中执行，且 MUST 排在镜像构建之前——让违规在数秒内被拒绝，而不是等 40 分钟构建完成后才发现。

#### Scenario: 有人把 expose 改成 ports

- WHEN 某 compose 档的 backend 出现 `ports: ["9091:9091"]`
- THEN 检查脚本 MUST 以非零退出码失败并指出文件与服务名
- AND CI MUST 在镜像构建前失败

#### Scenario: 正常的 expose 通过

- WHEN backend 仅声明 `expose: ["8080", "9091"]`
- THEN 检查脚本 MUST 以 0 退出

### Requirement: 端到端关联 id 覆盖前端与协议面

`X-Request-ID` 的透传 MUST 覆盖 nginx 的所有上游 location，不只 `/api/v1/`：`/swagger/` 与前端 location（`/`）MUST 同样 `proxy_set_header X-Request-ID $req_id`。

前端 SSR MUST 读取入站 `X-Request-ID` 并在其发往后端的请求上转发该头，使后端沿用它作为 trace id。仅由 nginx 转发而前端不读，则 SSR 渲染与它触发的 API 调用仍会各自生成不同的 id，关联在最常见入口处断裂。

nginx 本地应答的 `/health`（不经过上游）MUST 在响应头回写 `X-Request-ID`，使调用方的重试可与 nginx access log 对齐。

#### Scenario: 一次 SSR 渲染只有一个 id

- WHEN 浏览器请求一个 SSR 页面，nginx 已赋值 `X-Request-ID`
- THEN 该次渲染触发的后端请求 MUST 携带同一个 `X-Request-ID`
- AND 后端完成日志中的 `trace_id` MUST 等于 nginx access log 的 `rid`

#### Scenario: 浏览器直连 API 不受影响

- WHEN 浏览器直接调用 `/api/v1/*`（无入站 id）
- THEN 后端 MUST 按既有语义生成新 trace id，MUST NOT 因缺头而失败

### Requirement: 数据库慢查询与备份任务可观测

GORM 的默认 logger 把错误直接写标准库 stdout，绕过 `LOG_LEVEL`/`LOG_FORMAT`。因此 MUST 使用自定义 GORM logger，它：

- 对每个执行的语句递增 `orinforum_db_queries_total`；
- 对超过阈值（200ms）的语句额外递增 `orinforum_db_slow_queries_total` 并写一条带 `latency_ms`/`rows`/`sql` 的 warn 日志；
- MUST NOT 把 SQL 文本放进指标标签（无界文本，违反基数护栏）。

`metrics` 已 import `db`（连接池 collector），故该 logger MUST 经 `db.SetQueryObserver` 钩子注入，MUST NOT 让 `db` 反向 import `metrics`（会形成 import cycle）。

备份任务 MUST 以 `orinforum_backup_jobs_total{kind,outcome}` 与 `orinforum_backup_job_duration_seconds{kind}` 记录，`kind` ∈ `create`/`restore`/`cleanup`。outcome MUST 由 `err` 推导，MUST NOT 由调用方传入——否则「忘记把失败报成失败」无法被发现。

#### Scenario: 慢查询被计数但不进标签

- WHEN 一条查询耗时超过阈值
- THEN `orinforum_db_slow_queries_total` MUST +1，且 `orinforum_db_queries_total` 同样 +1
- AND 该 SQL 文本 MUST 只出现在日志中

#### Scenario: 备份失败可告警

- WHEN `Create` / `Restore` / `Cleanup` 返回 error
- THEN 对应 kind 的 `outcome="failed"` MUST +1，并触发 `BackupJobFailed`

### Requirement: 权限缓存规模可观测

三个权限缓存（role / space / group）MUST 以 `orinforum_permission_cache_entries{cache}` 报告条目数，`cache` ∈ `role`/`space`/`group`。取值 MUST 在缓存重建完成时设置（`LoadCache` 持有写锁期间），MUST NOT 在部分填充的中间态设置。

该 gauge 是**当前规模**，MUST NOT 作为高水位：缓存收缩时 gauge MUST 随之下降。

#### Scenario: 缓存重建后反映真实规模

- WHEN `LoadCache` 重建 role 缓存为 3 条
- THEN `orinforum_permission_cache_entries{cache="role"}` MUST 为 3
- AND 再次重建为 7 条时 MUST 变为 7（不累积）

### Requirement: 告警阈值不得只用绝对常量

登录失败类告警 MUST 以实例自身的历史基线为参照，MUST NOT 只用绝对阈值。绝对阈值在小实例上永不触发、在大实例上持续触发，两种情况下告警都失去意义。

`LoginFailureSpike` MUST 以「最近 5 分钟失败数 > 7 天均值（按 5 分钟切片）的 5 倍」判定，并 MUST 设有下限（10），以免基线接近零时因几次拼写错误触发。

#### Scenario: 小实例不误报

- WHEN 实例 7 天均值极低（例如每 5 分钟 0.2 次）
- THEN 5 分钟内出现 3 次失败 MUST NOT 触发告警（低于下限 10）

#### Scenario: 大实例相对突增仍触发

- WHEN 实例基线为每 5 分钟 20 次，某 5 分钟出现 120 次
- THEN MUST 触发告警（超过 5 倍基线）
