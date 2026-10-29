# Metrics Prometheus 指标

Prometheus 指标埋点模块，独立于 Iris 框架的指标注册表。

## 文件

| 文件 | 说明 |
|------|------|
| metrics.go | 自定义 `prometheus.Registry` + Go/Process collector + 4 个 HTTP 指标向量 |
| collector.go | `dbPoolCollector` — DB 连接池指标收集器（max_open/open/in_use/idle/wait_count/wait_duration） |
| http.go | `Serve(addr)` — 在 `METRICS_PORT` 上起内部指标监听（同步 bind，端口冲突即时上报；随 app 关闭而 Shutdown） |
| eventbus.go | 发布/分发耗时/handler panic/队列深度/丢弃 |
| readcache.go | `ReadCacheHits` / `ReadCacheMisses`（`feedchamber_readcache_{hits,misses}_total{name}`）；`name` 由 `readcache` 调用方给固定枚举 |
| kv.go | `KVOperationDuration{op,outcome}`（outcome ∈ ok/miss/error）+ `KVCasExhaustedTotal`；埋点经 `observe()` 单点 helper |
| gateway.go | 限流拒绝与 fail-open `{name}`；唯一埋点调用方是 `internal/pkg/ratelimit.Check`，`name` 为端点名或常量 `global` |
| permission.go | 权限判定 `{scope,result}`；`ObservePermission()` |
| sse.go | `feedchamber_sse_connections{channel}` gauge + `feedchamber_sse_write_failures_total{channel}`；两族在 `init()` seed 两个静态 channel |
| auth.go | `feedchamber_login_failures_total{reason}`（三值常量） |
| nats.go | `feedchamber_nats_connected{client}` gauge + `_disconnects_total` / `_reconnects_total`；`client` 静态两值 `kv` / `eventbus` |
| webhook.go | 投递结果/耗时/积压（积压取真实 `COUNT(*)`，非批大小） |
| observability_test.go | 指标族存在性与标签名契约 |
| nats_sse_test.go | NATS 连接状态机与 SSE 写失败语义 |
| metrics_test.go | 指标注册测试 |

## 设计

- 使用独立 Registry（非全局默认），通过 `promhttp.HandlerFor` 暴露
- **暴露面**：只在 `METRICS_PORT`（默认 9091）的独立 listener 上提供 `/metrics`，该端口无鉴权，MUST 仅容器网络内可达、MUST NOT 由 nginx 代理。`/api/v1/metrics` 业务路由已移除（原为 admin 门控，Prometheus 匿名抓取拿不到数据）
- `handler/metrics.go` 已删除；handler 层不再承担指标暴露
- DB Pool 指标通过 `RegisterDBPoolCollector(store)` 在启动时注册（SQL 后端）
- RequestDuration 按 method/path/status 分桶，path 经 `middleware/prometheus.go` 的 ID 折叠归一化
- **基数护栏**：标签取值必须有界。trace id / span id / 用户 id / 原始路径段 / 错误文本一律禁止作为标签
- 标签集合固定的族（permission/sse/login/webhook/http）在 `init()` 里 seed 零值，首次抓取即有 family
- 标签只在运行时已知的族（eventbus 事件名、kv op、ratelimit name）由**生产侧**在接线点 seed：`eventbus.New` 遍历 `AllEventTypes()` 调 `SeedEventbusEvent`、`kvstore.NewNATSStore` 调 `SeedKVOperation`、`middleware.RateLimit`（每路由注册）与 `gateway.NewRateLimiter` 调 `SeedRatelimitPolicy`。这样 `MetricsFamilyMissing` 的 `absent()` 才是真探针（族缺失＝接线没跑），健康但安静的实例不会误报；依赖方向仍是 eventbus/kvstore/gateway → metrics，不形成反向依赖
- histogram 在 TSDB 里只以 `<name>_bucket/_sum/_count` 存在，`absent()` 必须探 `_count`：`absent(<histogram 基础名>)` 恒为真，会让告警永久触发
- **NATS 三族的 seed 值是 1 不是 0**：`NATSConnected` 的序列存在本身意味着 `nats.Connect` 成功过，seed 成 0 会让健康实例立刻触发 `NATSDisconnected`。这与其它族的「零值 seed」是刻意相反的，改动前先想清楚
- **SSE 写失败与连接 gauge 不可合并**：连接 gauge 下降包含「客户端正常关标签页」，无法承担投递失败的语义。两个指标分开，任何一方都不能由另一方推导
- **积压类 gauge 不得取批查询长度**：`WebhookPendingDeliveries` 曾用 `len(ListPendingDeliveries(ctx, 20))`，而 20 是 worker 批大小，导致 gauge 结构性饱和在 20、基于它的告警永不触发。积压一律走独立的 `COUNT(*)`
- **暴露面有 CI 护栏**：`scripts/check-metrics-exposure.sh` 断言 compose 不发布 9091、nginx 不引用 9091，在 `.github/workflows/docker-build.yml` 的 `metrics-exposure` job 里跑。改 `ports`/nginx 前先跑它
