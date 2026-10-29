# observability Specification (delta)

## ADDED Requirements

### Requirement: NATS 连接状态可观测

KV Store 与 EventBus 各自持有一条 NATS 连接，两者都以无限重试策略连接（`nats.MaxReconnects(-1)`），断连后进程**不会退出**。因此连接状态 MUST 由指标表达，MUST NOT 只依赖日志：

| 指标 | 类型 | 标签 | 语义 |
| --- | --- | --- | --- |
| `feedchamber_nats_connected` | Gauge | `client` | 1 = 连接可用，0 = 已断开 |
| `feedchamber_nats_disconnects_total` | Counter | `client` | 从「可用」转为「断开」的次数 |
| `feedchamber_nats_reconnects_total` | Counter | `client` | 重连成功的次数 |

`client` 取值 MUST 限于静态集合 `kv` / `eventbus`（符合「指标基数护栏」）。三族 MUST 在 `init()` 内以 `connected=1` seed，因为构造这两条连接的前提就是 `nats.Connect` 成功——若 seed 为 0，健康实例会立即触发断连告警。

两处 `nats.Connect` MUST 注册 `DisconnectErrHandler` 与 `ReconnectHandler`：前者置 0 并累加 `disconnects_total`，后者置 1 并累加 `reconnects_total`。断连计数 MUST NOT 在重连时回退或抵消——「断连数 ≫ 重连数」是链路抖动的唯一可判据。

#### Scenario: 网络分区可见

- WHEN 后端与 NATS 之间的网络中断且持续超过重连等待
- THEN `feedchamber_nats_connected{client}` MUST 在 2 分钟内变为 0，并触发 `NATSDisconnected` 告警
- AND 该 client 的 `feedchamber_nats_disconnects_total` MUST 恰好 +1

#### Scenario: 重连恢复

- WHEN NATS 恢复可达
- THEN `feedchamber_nats_connected{client}` MUST 回到 1，`feedchamber_nats_reconnects_total` +1
- AND `feedchamber_nats_disconnects_total` MUST 保持不变（不得被抵消）

### Requirement: SSE 写失败与正常断开可区分

两个 SSE handler（`notification.Stream`、`realtime.Stream`）在向客户端写事件失败时 MUST 递增 `feedchamber_sse_write_failures_total{channel}`（`channel` ∈ `notification` / `realtime`），并在写失败分支终止该流。

该指标 MUST 与 `feedchamber_sse_connections` 分开：连接 gauge 的下降同时包含「客户端关标签页」这一正常事件，因而无法承担「投递失败」的语义。只增加连接 gauge 的精度而不新增失败计数，不能满足本要求。

#### Scenario: 写失败被计数

- WHEN 底层 `ResponseWriter` 的写操作返回错误
- THEN 对应 channel 的 `feedchamber_sse_write_failures_total` MUST +1
- AND 连接 gauge MUST 按既有 `defer` 语义正常回落，两个数字 MUST NOT 相互推导

#### Scenario: 静默实例不误报

- WHEN 实例启动后从未发生写失败
- THEN 两个 channel 的失败序列 MUST 已存在且值为 0（由 `init()` seed）

### Requirement: 积压 gauge 反映真实深度

`feedchamber_webhook_pending_deliveries` MUST 取真实的待投递总数（`COUNT(*)`），MUST NOT 取 worker 单批查询结果的长度。

worker 每 tick 用固定批大小取待投递行（当前 20），该长度是**吞吐参数**，与积压深度无关。用批大小充当积压会使 gauge 在深度超过批大小时饱和，使基于它的告警结构性不可触发。

计数与批查询 MUST 使用同一谓词（`status='pending' AND attempts<5`），并由测试钉住二者选中同一批行。`NextRetryAt` 在未来的行 MUST 计入积压——它们是尚未投递的工作，排除掉会让重试风暴看起来像空队列。

#### Scenario: 积压超过批大小

- WHEN 待投递行数为 35、worker 批大小为 20
- THEN `feedchamber_webhook_pending_deliveries` MUST 为 35，MUST NOT 为 20
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

- 对每个执行的语句递增 `feedchamber_db_queries_total`；
- 对超过阈值（200ms）的语句额外递增 `feedchamber_db_slow_queries_total` 并写一条带 `latency_ms`/`rows`/`sql` 的 warn 日志；
- MUST NOT 把 SQL 文本放进指标标签（无界文本，违反基数护栏）。

`metrics` 已 import `db`（连接池 collector），故该 logger MUST 经 `db.SetQueryObserver` 钩子注入，MUST NOT 让 `db` 反向 import `metrics`（会形成 import cycle）。

备份任务 MUST 以 `feedchamber_backup_jobs_total{kind,outcome}` 与 `feedchamber_backup_job_duration_seconds{kind}` 记录，`kind` ∈ `create`/`restore`/`cleanup`。outcome MUST 由 `err` 推导，MUST NOT 由调用方传入——否则「忘记把失败报成失败」无法被发现。

#### Scenario: 慢查询被计数但不进标签

- WHEN 一条查询耗时超过阈值
- THEN `feedchamber_db_slow_queries_total` MUST +1，且 `feedchamber_db_queries_total` 同样 +1
- AND 该 SQL 文本 MUST 只出现在日志中

#### Scenario: 备份失败可告警

- WHEN `Create` / `Restore` / `Cleanup` 返回 error
- THEN 对应 kind 的 `outcome="failed"` MUST +1，并触发 `BackupJobFailed`

### Requirement: 权限缓存规模可观测

三个权限缓存（role / space / group）MUST 以 `feedchamber_permission_cache_entries{cache}` 报告条目数，`cache` ∈ `role`/`space`/`group`。取值 MUST 在缓存重建完成时设置（`LoadCache` 持有写锁期间），MUST NOT 在部分填充的中间态设置。

该 gauge 是**当前规模**，MUST NOT 作为高水位：缓存收缩时 gauge MUST 随之下降。

#### Scenario: 缓存重建后反映真实规模

- WHEN `LoadCache` 重建 role 缓存为 3 条
- THEN `feedchamber_permission_cache_entries{cache="role"}` MUST 为 3
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
