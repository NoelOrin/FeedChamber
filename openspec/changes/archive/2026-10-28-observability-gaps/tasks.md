# Tasks

> 提案：[proposal.md](./proposal.md)；规范 delta：[specs/observability/spec.md](./specs/observability/spec.md)。
> 验证 Go 需 `export PATH="/usr/local/go/bin:$PATH"` 或登录 shell。

## 1. NATS 连接指标

- [x] 1.1 `internal/metrics/nats.go`（新）：`NATSConnected{client}` gauge、`NATSDisconnectsTotal{client}` / `NATSReconnectsTotal{client}` counter；`client` 常量 `NATSClientKV` / `NATSClientEventBus`
- [x] 1.2 `init()` 内对两个静态 client 调 `SeedNATSClient`，且 seed 时 `connected=1` —— 构造连接的前提就是 `nats.Connect` 成功，seed 成 0 会让健康实例立即触发断连告警
- [x] 1.3 `internal/pkg/kvstore/nats.go`：`nats.Connect` 增 `DisconnectErrHandler`（置 0 + `disconnects_total`+1 + warn 日志）与 `ReconnectHandler`（置 1 + `reconnects_total`+1 + info 日志）
- [x] 1.4 `internal/pkg/eventbus/eventbus.go`：同上，client 为 `eventbus`
- [x] 1.5 依赖方向保持 生产方 → metrics（kvstore/eventbus import metrics，metrics 不反向 import）

## 2. SSE 写失败指标

- [x] 2.1 `internal/metrics/sse.go`：新增 `SSEWriteFailuresTotal{channel}` + `ObserveSSEWriteFailure()`；channel 常量 `SSEChannelNotification` / `SSEChannelRealtime`；`init()` 同时 seed 两个 channel 的两个族
- [x] 2.2 `internal/handler/notification.go`：`writeSSE` 写失败分支调 `ObserveSSEWriteFailure`；gauge 的 label 改用常量
- [x] 2.3 `internal/handler/realtime.go`：`writeEnvelope` 同上；gauge 的 label 改用常量

## 3. webhook 积压 gauge 修正（既有缺陷）

- [x] 3.1 `internal/repository/webhook.go`：接口新增 `CountPendingDeliveries(ctx) (int64, error)`
- [x] 3.2 `internal/repository/webhook_sql.go`：`COUNT(*)` 实现，谓词与 `ListPendingDeliveries` 一致（`status='pending' AND attempts<5`）；`NextRetryAt` 在未来的行**计入**
- [x] 3.3 `internal/service/webhook.go`：`processPending` 先 `CountPendingDeliveries` 设 gauge，再取批处理；删除 `Set(float64(len(items)))`
- [x] 3.4 `internal/metrics/webhook.go`：Help 文案改为「true backlog」，并注释记录原缺陷（gauge 被批大小钉在 20）

## 4. 指标暴露面 CI 护栏

- [x] 4.1 `scripts/check-metrics-exposure.sh`（新）：awk 状态机按服务解析 `ports:` 块（避免把 postgres 的 ports 误判给 backend），识别短语法 `9091:9091` / `127.0.0.1:9091:9091` 与长语法 `target/published: 9091`；另查 nginx 不出现 `9091`
- [x] 4.2 `.github/workflows/docker-build.yml`：新增 `metrics-exposure` job（`timeout-minutes: 5`），排在 `server` / `web` 之前
- [x] 4.3 实测三种违规形状均被捕获：`ports: ["9091:9091"]`、`ports: ["127.0.0.1:9091:9091"]`、长语法 `target: 9091`；合规的 `expose` 通过

## 5. X-Request-ID 端到端

- [x] 5.1 `deploy/nginx/nginx.conf`：`/swagger/` location 补 `proxy_set_header X-Request-ID $req_id`；前端 location `/` 同样补
- [x] 5.2 `deploy/nginx/nginx.conf`：`/health`（nginx 本地应答）`add_header X-Request-ID $req_id always`
- [x] 5.3 `packages/api-client/src/adapter.ts`：`AuthAdapter` 新增 `getSsrRequestId()`，`defaultAdapter` 返回 null
- [x] 5.4 `packages/api-client/src/adapters/web/index.ts`：`WebSsrSources` 新增 `readRequestId()`，`createWebAdapter` 接线
- [x] 5.5 `packages/api-client/src/client.ts`：SSR 分支把 `getSsrRequestId()` 的结果写入 `X-Request-ID`
- [x] 5.6 `app/web/lib/api/adapter.ts`：`readRequestId: async () => (await headers()).get('x-request-id')`
- [x] 5.7 dashboard 适配器继承 `defaultAdapter`，自动获得默认实现，无需改动（已确认）

## 6. 告警与面板

- [x] 6.1 `deploy/prometheus/rules/orinforum.yml`：新增 `NATSDisconnected`（`connected==0` for 2m，severity page）、`WebhookBacklogGrowing`（`>100` for 15m）、`SSEWriteFailures`（10m 内 >5）
- [x] 6.2 `MetricsFamilyMissing` 补探 `orinforum_nats_connected`；注释更新
- [x] 6.3 三条新告警归入 `orinforum-business` 组（初稿误放 `metric-gaps`，已修正）
- [x] 6.4 `deploy/grafana/dashboards/orinforum.json`：新增面板 10「NATS 连接状态 / 断连与重连」、面板 11「SSE 写失败 / Webhook 积压」
- [x] 6.5 告警总数 13 → **16**，面板 9 → **11**

## 7. 测试

- [x] 7.1 `internal/metrics/nats_sse_test.go`（新）：NATS 两个 client seed 后 connected=1；断连→0 且 disconnects+1；重连→1 且 reconnects+1 且 **disconnects 不回退**（抖动判据）
- [x] 7.2 同上：两个 SSE channel 的 gauge 与失败计数族都已 seed；写失败只增计数、**不动连接 gauge**
- [x] 7.3 `internal/repository/webhook_test.go`（新）：35 条积压时 `CountPendingDeliveries`=35 而批查询仍 20（直接钉住原 bug）；计数与列表选同一批行（谓词一致性）；未来 `NextRetryAt` 计入、`attempts>=5` 排除
- [x] 7.4 `internal/metrics/observability_test.go`：`TestSeededFamiliesExist` 补 4 个新族；`TestLabelNamesAreBounded` 补 5 条标签契约（nats 三族 `{client}`、sse 失败 `{channel}`、webhook 积压 `{}`）
- [x] 7.5 测试断言方式沿用既有先例（`Registry.Gather()` 读 `dto.Metric`），未引入 `prometheus/testutil`，go.mod 零改动

## 8. 验证

- [x] 8.1 `go build ./...` / `go vet ./...` / `go test ./...` 退出码均为 **0**（19 包 ok，零 FAIL）
- [x] 8.2 `tsc -b packages/api-client/tsconfig.build.json` 与 `tsc --noEmit -p app/web/tsconfig.json` 退出码均为 0
- [x] 8.3 `bash scripts/check-metrics-exposure.sh` 在真实仓库上退出 0（两个 compose 档 + nginx 全部 OK）
- [x] 8.4 `deploy/prometheus/rules/orinforum.yml` YAML 解析通过，16 条告警分组正确
- [x] 8.5 `deploy/grafana/dashboards/orinforum.json` JSON 解析通过，11 面板
- [x] 8.6 `.github/workflows/docker-build.yml` YAML 解析通过，job 顺序为 `metrics-exposure` → `server` → `web` → `deploy`

## 9. 文档与规范同步

- [x] 9.1 `internal/metrics/AGENTS.md`：文件表补 `nats.go`，说明新族与 seed 策略、webhook 积压语义修正
- [x] 9.2 `openspec/specs/observability/spec.md`：本轮经 change 归档后合并（当前以 delta 形式存在）
- [x] 9.3 `docs/reviews/observability-findings-2026-10-13.md`：G3（NATS/SSE/webhook 三项）、G4、G5 标注已处理

## 10. G3 余项：GORM 慢查询

- [x] 10.1 `internal/db/gorm_logger.go`（新）：自定义 `slowQueryLogger` 实现 `logger.Interface`；`Trace` 对每语句调 `queryObserver`，超 200ms 另写带 `latency_ms`/`rows`/`sql` 的 warn 日志
- [x] 10.2 **import cycle 处置**：`metrics/collector.go` 已 import `db`，`db` 反向 import `metrics` 会成环（实测 `go build` 报 `import cycle not allowed`）。改为 `db` 暴露 `SetQueryObserver(fn func(slow bool))` 钩子，由 `metrics/db.go` 的 `init()` 注入——依赖方向仍是 `metrics → db`
- [x] 10.3 `internal/metrics/db.go`（新）：`orinforum_db_queries_total` + `orinforum_db_slow_queries_total`，**均无标签**（SQL 是无界文本，违反基数护栏；语句只进日志）
- [x] 10.4 `internal/db/sql.go`：`gorm.Open(dialector, &gorm.Config{Logger: newSlowQueryLogger()})`

## 11. G3 余项：权限缓存规模 + 备份任务

- [x] 11.1 `internal/metrics/permission_cache.go`（新）：`orinforum_permission_cache_entries{cache}`，`cache` ∈ `role`/`space`/`group`，`init()` seed 三值
- [x] 11.2 `internal/service/permission.go`：`LoadCache` 在**持有写锁期间**上报三个缓存的 `len()`（重建完成态，非部分填充态）
- [x] 11.3 `internal/metrics/backup.go`（新）：`orinforum_backup_jobs_total{kind,outcome}` + `orinforum_backup_job_duration_seconds{kind}`；`init()` seed 全部 kind×outcome 组合
- [x] 11.4 `internal/service/backup.go`：`Create`/`Restore`/`Cleanup` 三个入口用具名返回值 + `defer observeBackup(...)`；outcome 由 `err` 推导（不由调用方传入，否则「忘了报失败」无法被发现）

## 12. G2 告警阈值改相对基线

- [x] 12.1 `LoginFailureSpike` 从 `> 50/5m` 改为「> 该实例 7 天均值（按 5 分钟切片）× 5，且下限 10」。理由：绝对阈值在小实例永不触发、在大实例持续触发
- [x] 12.2 新增 `DBSlowQueries`（慢查询占比 > 5%，10m）、`BackupJobFailed`（1h 内出现 `outcome="failed"`）
- [x] 12.3 告警总数 13 → **18**

## 13. G7 no-OTel ADR

- [x] 13.1 `docs/adr/no-otel-sdk-yet.md`（新）：决策、五条理由、**保留的兼容缝**（W3C traceparent 双向兼容，将来接入只需把 SDK 接到解析结果上）、**明确承认的三项代价**（无 span 树/无采样/无 OTLP）
- [x] 13.2 `docs/ARCHITECTURE.md` 的 ADR 索引补链接

## 14. 文档修正（findings D2/D3/D4）

- [x] 14.1 `AGENTS.md` 项目结构行：`cmd/root.go` 改为「持久标志 `-e/--env`（子命令只有 backup / import-discourse）」
- [x] 14.2 `AGENTS.md` 的 CLI 小节：删「支持 dev 和 prod 子命令」，写明 `go run main.go dev` 会报 unknown command
- [x] 14.3 `app/server/AGENTS.md` 文件表同一行同步
- [x] 14.4 D1（`openspec/specs/operations/spec.md`）复核：正文已正确，`:51` 的 scenario 本就是「位置参数形式被拒绝」的断言，**无需改动**

## 15. 追加验证

- [x] 15.1 `go build` / `go vet` / `go test ./...` 退出码全 0（19 包）
- [x] 15.2 `internal/metrics/gaps_test.go`（新）：DB 计数经钩子接线、权限缓存 gauge seed 与可收缩、备份 kind×outcome 全覆盖
- [x] 15.3 `observability_test.go` 的族存在性与标签契约补入 5 个新族
- [x] 15.4 指标族 20 → **29**；告警 13 → **18**；面板 9 → **13**
- [x] 15.5 告警 YAML 与 dashboard JSON 解析通过

## 16. 明确未做（保留在 findings 中）

- [x] 16.1 NATS server 自身指标（`/varz` / JetStream 存储水位）：`deploy/docker-compose.yml` 新增 `nats-exporter`（`natsio/prometheus-nats-exporter:0.15.0`，`-varz -jsz all -connz` 抓 `nats:8222`），`deploy/prometheus.yml` 增 `job_name: nats`；`rules/orinforum.yml` 新增 `orinforum-nats` 组 5 条告警（exporter down / 慢消费者 / JetStream 存储 >85% / 消费者积压 / 连接数异常）。顺带修掉既有 `BackupJobFailed` 的 `\"` 转义错误（此前整份规则文件 promtool 解析失败）。`promtool check rules` → 23 rules SUCCESS
- [x] 16.2 5xx / P99 / DB 池阈值按真实流量回归：真实分位数无法在本地合成（用相对基线替代只会得到"永远不响"或"一直响"的假规则），因此改为交付校准工具 `scripts/calibrate-alert-thresholds.sh`——在**有真实流量的环境**里一次性打印 5xx 占比、P50/P90/P99/P99.9、DB 池使用率与等待次数、NATS 慢消费者/存储水位/连接数，每项都附「怎么读」的判据，结论直接写回 `rules/orinforum.yml`。脚本只读不写，连不上 Prometheus 时 fail fast。已用真实 Prometheus 实例验证全部 PromQL 表达式被接受（status=success）
