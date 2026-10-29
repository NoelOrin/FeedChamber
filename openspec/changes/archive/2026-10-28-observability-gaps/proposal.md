## Why

`2026-10-13-build-observability` 建立了指标与追踪的地基，但留下了明确的埋点空白，且这些空白正好落在「故障静默」的位置——出事时没有任何信号，只能事后翻日志。`docs/reviews/observability-findings-2026-10-13.md` 的 G3/G4/G5 记录了它们，本轮把它们做完。

三项埋点空白（G3）：

1. **NATS 连接状态无指标。** `kvstore/nats.go` 与 `eventbus/eventbus.go` 都以 `nats.MaxReconnects(-1)` 连接，即断连后无限重试、进程永不退出。这本身是正确设计，但代价是**网络分区完全不可见**：没有 `DisconnectErrHandler`/`ReconnectHandler`，断连只在某条 logrus 输出里偶然出现。而 2026-10-28 的改动把 NATS 从「进程内嵌、可选」变成「外部硬依赖、必填」，这条路径的故障代价从「降级」升到「会话缓存、限流、事件总线、权限同步全部失效」。
2. **SSE 写失败无区分。** 两个 stream handler 的写函数在 `fmt.Fprintf` 出错时只 `return`。连接 gauge 随之下降，但**这与客户端正常关标签页无法区分**——真正的问题（代理缓冲/截断流）被淹没在正常的连接数波动里。
3. **webhook 积压 gauge 恒等于 20。** `processPending` 用 `len(ListPendingDeliveries(ctx, 20))` 设值，而 20 是 worker 的批大小。积压 500 条时面板仍显示 20，且任何基于该值的告警都不可能触发。这是既有埋点的**实质缺陷**，不是空白。

两项对外边界问题（G4/G5）：

4. **`METRICS_PORT` 无鉴权且只靠部署纪律兜底。** compose 用 `expose` 而非 `ports` 是对的，但把 `expose` 改成 `ports`、或在 nginx 加一条指向 9091 的 location，都会让 DB 池与流量内部数公网可读，而**没有任何自动化检查会发现**。
5. **`X-Request-ID` 只透传到 `/api/v1/`。** 前端 location 与 `/swagger/` 没有该头，前端 SSR 也不读它，因此「前端报错」与「后端 trace」无法用同一个 id 对齐——关联链路在最常见的入口处断掉。

G3 余项（本轮一并做完）：

6. **GORM 慢查询无指标。** `gorm.Open` 用默认 logger，它把错误直接写标准库 stdout，**绕过 `LOG_LEVEL`/`LOG_FORMAT`**；慢查询既不计数也不进结构化日志。
7. **权限缓存规模不可见。** role/space/group 三个内存 map 没有任何规模指标，异常膨胀只能靠读内存发现。
8. **备份任务成败无指标。** 备份失败会留下「上一条仍然 ready」的列表，只有一行日志，直到真正需要恢复那天才暴露。

以及两项遗留（findings G2/G7）：

9. **`LoginFailureSpike` 是绝对阈值 50/5m。** 小实例永不触发、大实例持续触发，两种情况下告警都无意义。
10. **「不引 OpenTelemetry」这个不可逆取向没有 ADR**，change 归档后语境只剩提案文件，容易被后人当技术债推翻。

## What Changes

- **NATS 连接指标**（`orinforum_nats_connected` / `_disconnects_total` / `_reconnects_total`，标签 `client` ∈ `kv`/`eventbus`）。两处 `nats.Connect` 都挂 `DisconnectErrHandler` / `ReconnectHandler`；gauge 在连接成功时置 1，断连置 0，重连再置 1。client 标签是**静态两值**，符合既有基数护栏。
- **SSE 写失败指标**（`orinforum_sse_write_failures_total{channel}`）。两个 handler 的写函数在写失败时计数；与连接 gauge **分开**，因为后者的下降包含正常关连接。
- **webhook 积压改真值**：`WebhookRepository` 新增 `CountPendingDeliveries`，`processPending` 每 tick 先 `COUNT(*)` 设 gauge，再取批处理。计数与列表使用同一谓词（`status='pending' AND attempts<5`），并由测试钉住二者选同一批行。
- **CI 护栏**（`scripts/check-metrics-exposure.sh`）：断言任何 compose 档的 backend 都不发布 9091，且 nginx 不引用 9091。新增一个 `metrics-exposure` CI job，跑在 40 分钟镜像构建之前。
- **`X-Request-ID` 端到端**：nginx 的 `/swagger/`、`/`（前端）location 补透传；`/health`（nginx 本地应答）回写该头。`api-client` 的 `AuthAdapter` 新增 `getSsrRequestId()`，SSR 请求把它作为 `X-Request-ID` 转发给后端，使后端沿用同一个 trace id。
- **DB 慢查询**：`internal/db/gorm_logger.go` 自定义 GORM logger，每语句计 `orinforum_db_queries_total`，超 200ms 另计 `orinforum_db_slow_queries_total` 并写结构化 warn（SQL 只进日志、不进标签）。因 `metrics` 已 import `db`，经 `db.SetQueryObserver` 钩子注入，避免 import cycle。
- **权限缓存规模**：`orinforum_permission_cache_entries{cache}`，在 `LoadCache` 持写锁期间设置（重建完成态，非中间态）。
- **备份任务**：`orinforum_backup_jobs_total{kind,outcome}` + `orinforum_backup_job_duration_seconds{kind}`，outcome 由 `err` 推导。
- **告警阈值改相对基线**：`LoginFailureSpike` 从「>50/5m」改为「> 7 天均值的 5 倍，下限 10」。
- **ADR**：新增 `docs/adr/no-otel-sdk-yet.md`，记录决策、理由、保留的兼容缝与明确代价。
- **告警与面板**：告警 13 → **18** 条（`NATSDisconnected`、`WebhookBacklogGrowing`、`SSEWriteFailures`、`DBSlowQueries`、`BackupJobFailed`），`MetricsFamilyMissing` 补探 `orinforum_nats_connected`；Grafana 9 → **13** 面板。
- **文档修正**（findings D2/D3/D4）：`AGENTS.md` 与 `app/server/AGENTS.md` 仍写「Cobra 分发 dev/prod 子命令」，实际只有持久标志 `-e/--env`。D1（operations spec）此前已修正。

## Capabilities

### Modified Capabilities

- `observability`：新增「NATS 连接状态可观测」「SSE 写失败可区分」「积压 gauge 反映真实深度」「指标端口暴露面由 CI 强制」四条 requirement；「请求日志字段」requirement 补充端到端关联约束。

## Impact

- **后端**：`internal/metrics/nats.go`（新）、`internal/metrics/sse.go`、`internal/metrics/webhook.go`、`internal/pkg/kvstore/nats.go`、`internal/pkg/eventbus/eventbus.go`、`internal/handler/{notification,realtime}.go`、`internal/repository/webhook{,_sql}.go`、`internal/service/webhook.go`。
- **前端**：`packages/api-client/src/{adapter,client}.ts`、`packages/api-client/src/adapters/web/index.ts`、`app/web/lib/api/adapter.ts`。`AuthAdapter` 新增方法，但 `defaultAdapter` 提供默认实现，dashboard 适配器继承它，无需改动。
- **部署**：`deploy/nginx/nginx.conf`（两处 location + health 回写）、`deploy/prometheus/rules/orinforum.yml`、`deploy/grafana/dashboards/orinforum.json`。
- **CI**：`.github/workflows/docker-build.yml` 新增 job；`scripts/check-metrics-exposure.sh`（新）。
- **新增指标文件**：`internal/metrics/{nats,db,permission_cache,backup}.go`、`internal/db/gorm_logger.go`、`docs/adr/no-otel-sdk-yet.md`。
- **非破坏性**：不新增/删除任何 HTTP 端点；`orinforum_webhook_pending_deliveries` 名称不变，仅语义从「批大小」改为「真实积压」——历史数据在改动点是断点，看板需以改动时间分段解读。
- **不在本轮范围**：NATS server 自身指标（`/varz` / JetStream 存储水位）——需在容器网络内多一个 scrape target；G2 的其余阈值（5xx/P99/DB 池）按真实流量回归校准——那些需要真实分位数，本轮只改了能用相对基线表达的那条。
