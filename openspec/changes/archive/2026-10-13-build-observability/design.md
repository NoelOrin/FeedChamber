# Design

## 背景

可观察性三根支柱里，本仓库只有第一根的一半：

- **Metrics**：`internal/metrics/metrics.go:9` 独立 Registry + Go/Process collector + 4 个 HTTP 指标，`middleware/prometheus.go:13` 已带 path 归一化，`metrics/collector.go:23` 已导出 DB 池。骨架正确，但覆盖面只有 HTTP 与 DB。
- **Logging**：`internal/pkg/logger` 是一套完整的 logrus 结构化封装（接口 + `With` 链式 + `WithContext`），但 `Config` 在生产路径上无人构造。
- **Tracing**：`pkg/gateway/tracing.go:16-19` 空函数体。

关键约束（决定了下面的顺序设计）：`server/iris.go:240-247` 的三个 `app.Use(...)` 是**全局中间件**，先于任何 per-party 中间件执行；而 `Tracing` 挂在受保护 party 上（`gateway.go:45`）。所以「实现 `Tracing.Middleware` 然后指望 `Logger()` 输出 trace_id」是行不通的——Logger  completion 日志在 trace 建立之前就已排队。

另一个规模约束：仓库有 **156 处裸 `logrus.X()` 调用分布在 30 个文件**（含 `server/iris.go` 51 处、`service/notification.go` 12 处、`eventbus.go` 7 处）。逐文件改成注入式 logger 不属于本轮范围。

## 决策

### 1. 自研 W3C `traceparent`，不引入 OpenTelemetry SDK

实现一个约 150 行的 `internal/pkg/tracing` 包：`TraceID [16]byte` / `SpanID [8]byte`、`ParseTraceparent(string) (Context, bool)`、`(Context).Traceparent() string`、`New()`（`crypto/rand`）、context With/From。格式严格遵循 W3C Trace Context Level-1 的 `00-{32hex}-{16hex}-{2hexflags}`，采样位置位 `01`。

不引 OTel 的理由：(a) 仓库没有 collector/Tempo/Jaeger，只加 SDK 会产出无处可去的 span；(b) `deploy/` 目前连 Prometheus 配置文件都缺，先把自有平面跑通更实在；(c) 项目偏好「修接缝而非加层」（请求层已按此原则处理过）。**缝保留**：入站/出站头部就是 OTel 使用的同一个 `traceparent`，将来替换 SDK 时对外契约不变。

### 2. 包位置 `internal/pkg/tracing`

`pkg/gateway` 已经 import `feedchamber/internal/...`（`apikey.go:15-16`、`auth.go:8-9`），所以 gateway、`internal/middleware`、`internal/pkg/eventbus`、`internal/service` 都能单向依赖同一个包，不产生反向依赖。

### 3. 一个关联 id，不做双 id

`request_id` 字段更名为 `trace_id`，`X-Request-ID` 响应头的取值改为 trace id（头名不变以兼容客户端与 nginx 配置）。入站优先级：合法 `traceparent` > 合法 `X-Request-ID`（恰为 32 位十六进制时）> 新生成。

理由：`$request_id`（nginx 内置变量）恰好是 32 位十六进制，与 W3C trace id 同形同宽。用它作回退，nginx 访问日志与后端应用日志天然共享一个 id，无需引入第二套映射。

### 4. 拆成两层：全局 `middleware.Trace()` 建 trace，网关 `Tracing` 命名 span

```
app.Use(CORS) → app.Use(middleware.Trace()) → app.Use(middleware.Logger()) → app.Use(Prometheus)   // iris.go:240-247
                              ↑ 新增：确定 trace、ResetRequest 写入 ctx.Request().Context()、回写响应头
party.Use(gw.Tracing.Middleware)   // 保留原位：只补 route 级 span 名（用归一化路由模板）
```

`Tracing.Middleware` 不再是空壳，但它不承担「建立 trace」职责——那必须早于 Logger。这是本设计最容易被写错的一点，故在 tasks 里作为独立子任务与独立测试。

### 5. `LOG_LEVEL` 生效面：由 `SetGlobal` 同步 `logrus.StandardLogger()`

`logger.SetGlobal(l)` 在替换全局实例之外，MUST 调用 `logrus.SetLevel/SetFormatter/SetOutput`，使 156 处存量裸 `logrus.X()` 一并受 `LOG_LEVEL`/`LOG_FORMAT`/`LOG_OUTPUT` 管辖。`New(cfg)` MUST NOT 有全局副作用，`NewNop()` 更 MUST NOT 触碰标准 logger（保持测试隔离）。

这是对全局单例的显式副作用，写在函数注释里，并由 `logger_test.go` 两条测试分别锁住「SetGlobal 会同步」与「New 不会同步」。**实现期修正**：原计划把副作用放在 `New`，实测会让每个 `New(Config{})` 测试调用污染标准 logger，故移到 `SetGlobal`——启动路径只有一次调用，效果相同。

### 6. ctx 注入用 `ctx.ResetRequest(...WithContext(...))`

`internal/handler/*` 一律已经用 `ctx.Request().Context()`（`auth.go:88`、`content.go:132`、`notification.go:202` 等），Service 一律 ctx 首参（`service/auth.go:276`、`service/post.go:93`）。因此**只要** trace 进了 request context，下层无需任何改动即可通过 `logger.FromContext(ctx).WithContext(ctx)` 拿到——本项零业务改动。

### 7. 指标暴露面：独立 listener，删除 `/api/v1/metrics`

现 `router/router.go:119` 把 `/metrics` 门控在 `PermAdminMetrics` 之后，Prometheus 匿名抓取拿不到数据（除非配 token）。选择独立端口而非抓取 token：

- `server/iris.go` 在 `METRICS_ENABLED` 为真时另起 `http.Server` 监听 `:{METRICS_PORT}`（默认 9091），handler 复用现 `handler.MetricsServeHTTP`（`handler/metrics.go:11`）；
- 删除 `router.go:119` 的业务路由；
- compose 中 backend 用 `expose: ["9091"]` 而非 `ports`，Prometheus 走容器网络抓取，nginx 不代理该端口。

安全面因此从「admin 鉴权的公网路径」变成「无鉴权的内部端口」，是常规做法，但 MUST 在文档明确不得公网暴露。Grafana 成为唯一查看面。

### 8. EventBus 不改 `Handler func(event Event)` 签名

分发错误统计复用既有 panic 恢复块（`eventbus.go:84-88`），耗时用 `dispatch`（`:78`）的直方图。改成 `func(Event) error` 会牵动 14 个 Publish 调用点与全部订阅端，属另一件事。

发布侧改 `nc.PublishMsg`（`:177`；nats.go v1.39.1 没有 `MsgPublish`）为唯一必要侵入点；消费侧 `handleNATSMessage`（`:114`）是天然头部提取点，顺带替换 `:123` 的 `Ctx: context.Background()`。注意 `Publish` 在 `:164` 会主动把 `Event.Ctx` 重置为 background（防请求取消导致通知静默失败），因此只把 trace 值搬到新 background ctx 上，不搬运 ctx 本身。

### 9. KV 埋点收成一个 helper

`kvstore/nats.go` 10 个方法各自出口统一调 `observe(op string, start time.Time, err error)`，避免 10 份重复的 `time.Now()` 与 outcome 判定。`incrCAS`（`:335`）的重试耗尽（`:369`，上限 50 次）单记一个计数器。

### 10. 权限指标只带 `scope` 与 `result`

`HasPermission`（`permission.go:217-226`）与 `HasPermissionInSpace`（`:255`）各一处计数。角色名虽有界（约 6 个），但 `role` 维度会 6 倍膨胀序列且暂无排障需求，先不加——按「不做投机性设计」处理，加标签是可逆的后续动作。

### 11. SSE 用 gauge 且各自 defer 递减

两个 Stream handler 无连接注册表（`notification.go:186-254`、`realtime.go:29-132`），各自在 Flusher 校验通过后 `Inc()`、`defer Dec()`；`channel` 标签取值固定 `notification` / `realtime`。

### 12. nginx：`map` 决定 `X-Request-ID`，日志格式收录该字段

`deploy/nginx/nginx.conf` 现仅设 `Host/X-Real-IP/X-Forwarded-For`（`:7-12`）。新增：

```nginx
map $http_x_request_id $req_id {
    default   $http_x_request_id;   # 客户端/上游已带则沿用
    ""        $request_id;          # 否则用 nginx 内置 32hex
}
server { log_format main '... rid=$http_x_request_id ...';
         location /api/ { proxy_set_header X-Request-ID $req_id; } }
```

### 13. `deploy/prometheus.yml` 与告警规则

抓取 `backend:9091`，`rule_files: [/etc/prometheus/rules/*.yml]`，规则文件落 `deploy/prometheus/rules/feedchamber.yml`，8 条告警至少覆盖：`High5xxErrorRate`（`RequestsTotal{status=~"5.."}` 比率）、`HighP99Latency`（`histogram_quantile` over `RequestDuration`）、`DBPoolSaturation`（`in_use` / `max_open` 或 `wait_count` 增率）、`SlowKVOperations`、`EventBusDrops`、`WebhookTerminalFailures`、`LoginFailureSpike`、`HealthEndpointDown`（`probe_success{job="feedchamber"}` 或 `/api/v1/health` up）。

### 14. Grafana provisioning 与口令

`deploy/grafana/provisioning/datasources/prometheus.yml`（指向 `http://prometheus:9090`）+ `dashboards/dashboards.yml`（读 `deploy/grafana/dashboards/`），挂载到 `/etc/grafana/provisioning`；口令写 `GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_ADMIN_PASSWORD:?set GRAFANA_ADMIN_PASSWORD}`（缺失即 compose 报错，杜绝 `admin` 明文）。首张 dashboard 覆盖 RED、延迟分位、DB 池、EventBus、webhook 积压。

## 备选方案

- **OpenTelemetry Go SDK + OTLP**：契约更标准，但要求 collector/Tempo 才有落点，`deploy/` 连 prometheus.yml 都缺，先把自有平面跑通。放弃，保留 `traceparent` 兼容缝。
- **保留 `/api/v1/metrics` + 抓取专用 token**：省一个端口，但要新增 token 校验分支并让指标继续暴露在公网路由上。放弃。
- **`request_id` 与 `trace_id` 并存**：日志里两个 id、采集端要做 join，收益只有「历史字段不更名」。放弃。
- **逐文件替换 156 处裸 logrus 为注入式 logger**：改动面 30 个文件、与在途迁移冲突。放弃，改用决策 5 的单点副作用。
- **EventBus `Handler` 返回 error 以便统计失败**：14 个 Publish 点 + 全部订阅端连锁。放弃（决策 8）。

## 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| trace 建在 Logger 之后，日志永远缺 trace_id（决策 4 的顺序坑） | 单列子任务；测试断言「请求完成日志含 trace_id」，而非仅测 middleware 存在 |
| 客户端伪造 `traceparent` | id 非机密、不影响鉴权；trace id 绝不作为指标标签（基数护栏入 spec），畸形值一律丢弃重生成 |
| 无鉴权的 9091 端口成为新暴露面 | compose 用 `expose` 不用 `ports`；nginx 不代理；AGENTS 与 compose 注释双处标注「内部端口」 |
| `ResetRequest` 改动请求对象，可能干扰已依赖 `ctx.Request()` 的中间件 | Trace 置于 CORS 之后、Logger 之前；对 `/stream`（SSE 跳过 RequestTimeout，`iris.go`）与 `PathBodyLimit` 的包装顺序做回归测试 |
| `logrus.StandardLogger()` 全局副作用污染测试 | `NewNop()` 明确不触碰标准 logger；测试统一用 `NewNop` |
| KV/EventBus 每次操作多一次 histogram Observe 的开销 | 单 helper、复用已有 `start`（`webhook.go:328` 已有 `LatencyMs`，不重复计时）；直方图桶沿用 `DefBuckets` |
| 日志字段更名影响外部采集 | 全仓 grep 无日志解析消费者（`e2e-test.js` 只锁 DOM/URL）；作为行为变更写进发布说明 |
| 与在途 `2026-10-13-remove-mongodb-backend` 的 `operations` delta 冲突 | 本 change 的 delta 基于其文本（`DB_TYPE` 无 mongo），proposal 已声明应用顺序；只 ADD `layered-architecture` 要求，不改其已被占用的小节 |
| 本地 Docker daemon 未运行，无法端到端验证 compose | 用 `docker compose -f docker-compose.cluster.yml config` 做静态校验（不需 daemon），并加 grep 断言挂载文件存在 |
