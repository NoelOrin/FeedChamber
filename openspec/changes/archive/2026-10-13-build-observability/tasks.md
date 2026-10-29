# Tasks

> 前置：`2026-10-13-remove-mongodb-backend` 已归档（本 change 的 `operations` delta 以其文本为基础）。
> 验证 Go 需 `export PATH="/usr/local/go/bin:$PATH"` 或登录 shell（非登录 shell 的 PATH 不含 `/usr/local/go/bin`）。

## 1. 日志地基接通

- [x] 1.1 `internal/config/config.go`：新增 `Log logger.Config`（直接复用 logger 的 Config，不造平行结构）、`MetricsPort`、`TracingEnabled`；`Validate()` 补 `METRICS_PORT` 范围校验
- [x] 1.2 `internal/config/config_test.go`：`Config` 字面量用例仍编译（默认值与非法级别由 1.3/§9 覆盖）
- [x] 1.3 `internal/pkg/logger/logger.go`：**偏离原计划** —— 标准 logrus 的同步放在 `SetGlobal()` 而非 `New()`，`New` 保持无全局副作用以便测试隔离；`NewNop` 同样不触碰标准 logger；`logger_test.go` 两条测试锁住这组行为
- [x] 1.4 文件输出分支失败回退 stderr（沿用原实现；冒烟确认不中断启动）
- [x] 1.5 `server/iris.go`：`cfg.Validate()` 之后、`db.New` 之前 `logger.SetGlobal(logger.New(cfg.Log))`
- [x] 1.6（新增）死码清理：`ContextWithRequestID` / `ContextWithUserUUID` / `GetFromValues` / `ValuesGetter` 全无生产调用者（唯一在用的是 `middleware.go` 的 `logger.Global()`），删除；`WithContext` 改从 `tracing` 取字段。`NewContext`/`FromContext` 保留为唯一关联通路

## 2. trace 内核包

- [x] 2.1 `internal/pkg/tracing/tracing.go`：`Context{TraceID,SpanID,ParentSpanID,Flags,Name}`、`New`/`Child`/`FromTraceID`、`ParseTraceparent`、`ParseTraceIDHex`、`Traceparent()`、context With/From（含 nil ctx 守卫）
- [x] 2.2 `tracing_test.go`：往返 + 12 种畸形输入（版本/长度/大写/全零/非十六进制/字段数）+ Child 语义 + context 往返，全包通过

## 3. 全局 Trace 中间件（顺序是硬约束）

- [x] 3.1 `internal/middleware/trace.go`：`Trace(enabled)` + `RequestTrace(ctx)`；优先级 `traceparent` > `X-Request-ID`(32hex) > 新生成；`ResetRequest` 写入 `request.Context()`；回写 `X-Request-ID`，启用时另回写 `traceparent`
- [x] 3.2 `internal/middleware/middleware.go`：`Logger()` 去掉自造 uuid 与 `request_id` 字段，改读 trace 输出 `trace_id`/`span_id`，并把请求级 logger 放进 context；`route` 在 `ctx.Next()` 之后补齐；`uuid` import 随之移除
- [x] 3.3 `server/iris.go`：`app.Use(middleware.Trace(cfg.TracingEnabled))` 插在 `CORS()` 与 `Logger()` 之间
- [x] 3.4 见 1.6（通路收敛到 `logger.NewContext`/`FromContext`，未保留 request-id 别名）

## 4. 网关 Tracing 实化与 nginx

- [x] 4.1 `pkg/gateway/tracing.go`：空 TODO 替换为真实实现——路由匹配后把 `ctx.GetCurrentRoute().Path()`（模板，非具体路径）写进 `trace.Name`；沿用同一 span id，不调 `ctx.Next()`（与 `gateway.Handler()` 的组合方式一致）
- [x] 4.2 `deploy/nginx/nginx.conf`：`map $http_x_request_id $req_id` + `proxy_set_header X-Request-ID $req_id`；`log_format orinforum_main` 带 `rid=`（名称必须避开 http 上下文里已存在的 `combined`）
- [x] 4.3 `WithRemoteAddrHeader` 未改动；伪造 trace id 的风险按 design 风险表接受（id 非机密、不进标签）

## 5. trace 经 NATS 传播

- [x] 5.1 `eventbus.go`：`nc.Publish` → **`nc.PublishMsg`**（nats.go v1.39.1 无 `MsgPublish`），消息头写 `traceparent`；发布前先从 `event.Ctx` 取 trace，再按原语义重置为 background
- [x] 5.2 `handleNATSMessage`：`consumerContext(msg.Header)` 恢复 trace，替换 `Ctx: context.Background()`；无有效头部时**新生成**（初稿漏了，spec 场景要求之）
- [x] 5.3 **原任务前提有误**：本 EventBus 无「同进程 dispatch 绕过 NATS」快路径（`enqueue` 仅被 `handleNATSMessage` 调用），发布一律走网络，该条不适用
- [x] 5.4 `eventbus/trace_test.go`：头部往返、无头部回退、被取消的发布方 ctx 不泄漏给消费端、handler panic 计数且不阻断其他 handler

## 6. 指标暴露面迁移

- [x] 6.1 `internal/metrics/http.go`（新）：`Serve(addr)` 同步 `net.Listen` 再异步服务，端口冲突由调用方 warn 而非 crash；`server/iris.go` 在 `METRICS_ENABLED` 为真时启动
- [x] 6.2 `internal/router/router.go`：删除 `protected.Get("/metrics", ...)`；`model.PermAdminMetrics` 保留（`pkg/gateway/apikey.go:40` 白名单仍引用）
- [x] 6.3 启动日志文案改为实际内部地址
- [x] 6.4 `app.Shutdown` 后 `metricsSrv.Shutdown(ctx)` 复用同一 10s ctx
- [x] 6.5（连带）`internal/handler/metrics.go` 整体删除：6.2 之后 `MetricsServeHTTP` 与 `h.Metrics` 无任何引用

## 7. 业务指标埋点

- [x] 7.1 `internal/metrics/eventbus.go` + 埋点 `dispatch`(耗时/panic)、`enqueue`(深度/丢弃)、`worker`(深度回落)、`Publish`(result ∈ ok/encode_error/nats_error/closed)
- [x] 7.2 `internal/metrics/kv.go` + `kvstore/observe.go`（新 helper）：具名返回值 + `defer observe(op, &err)` 覆盖 8 个方法；`outcome` 增 **`miss`** 档（`ErrKeyNotFound` 是缓存正常语义，混入 error 会让告警失效）；`incrCAS` 收敛为 op="incr" 咽喉点；`KVCasExhaustedTotal` 接重试耗尽分支
- [x] 7.3 `internal/metrics/gateway.go` + `pkg/gateway/ratelimit.go`（rejections/failopen；3 处 `fmt.Printf/Println` 换结构化 logger）与 `internal/middleware/ratelimit.go`（废弃孪生实现同样埋点，label 用 endpoint 名，含 IP 的 key 不进标签）
- [x] 7.4 `internal/metrics/permission.go` + `service/permission.go`：两个方法改具名返回值 + defer 计数，仅 `scope`/`result` 两维
- [x] 7.5 `internal/metrics/sse.go` + 两个 Stream handler：Flusher 校验通过后 Inc，`defer` Dec
- [x] 7.6 `internal/metrics/auth.go` + `service/auth.go`：三个失败分支分别 Inc（`unknown_user`/`no_credential`/`bad_password`），计数放在 `secMgr != nil` 守卫之外；暴力破解沿用 `SecurityEvent` 存储不加指标
- [x] 7.7 `internal/metrics/webhook.go` + `service/webhook.go`：`deliver` 只观察耗时；结果分类放在 `processPending`（`delivered`/`retry_scheduled`/`failed_terminal`）避免双计数；积压 gauge 在 `ListPendingDeliveries` 后设值（该函数即 ticker 体）
- [x] 7.8 注册与 seed：标签静态已知的族 seed 零值（permission/sse/login/webhook/http）；**偏离**——eventbus 事件名、kv op、ratelimit name 无法在 metrics 包静态枚举（会形成 metrics→eventbus 反向依赖），改由 `MetricsFamilyMissing`(`absent()`) 告警兜底

## 8. Prometheus / 告警 / Grafana / compose

- [x] 8.1 `deploy/prometheus.yml`（新）：抓 `backend:9091`；`rule_files: /etc/prometheus/rules/*.yml`；`orinforum-health` 经 blackbox 探 `/api/v1/health`
- [x] 8.2 `deploy/prometheus/rules/orinforum.yml`（新）：**13 条**告警（超出最低 8 条）——5xx 比率、P99、指标端口 up、health 探测、DB 池饱和、KV P99、CAS 耗尽、EventBus 丢弃、handler panic、webhook 终态失败、登录失败突增、限流 fail-open、指标族缺失
- [x] 8.3 `deploy/grafana/provisioning/{datasources,dashboards}/*.yml` + `deploy/grafana/dashboards/orinforum.json`（9 面板）
- [x] 8.4 `docker-compose.cluster.yml`：prometheus 增规则挂载 + `depends_on: backend`；grafana 挂 provisioning/dashboards、口令改 `${GRAFANA_ADMIN_PASSWORD:?...}`；backend 补 `LOG_*`/`METRICS_PORT`/`TRACING_ENABLED`，`expose: ["8080","9091"]` 且无 `ports`
- [x] 8.5 确认 `basic/lite/full/docker-compose.yml` 未被要求引入监控栈（不动；`METRICS_ENABLED=false` 时 9091 无监听）
- [x] 8.6（新增）`deploy/blackbox/blackbox.yml`：`http_2xx` 模块。`/health` 返回 JSON 而非 Prometheus 文本，不引 prober 则「健康检查失败」告警无判定依据
- [x] 8.7（新增，既存缺陷）cluster compose 的 NATS 锚点把 `depends_on: [nats-2, nats-3]` 一并复制给 nats-2/nats-3，形成 `dependency cycle detected: nats-2 -> nats-2`；锚点移入顶层 `x-nats` 扩展字段修复。**不修则验收命令 9.9 无法执行**

## 9. 测试与验证

- [x] 9.1 `internal/middleware/trace_test.go`：完成日志与服务层日志共享同一 `trace_id`（等于响应 `X-Request-ID`）、沿用入站 `X-Request-ID` 并据此构造 `traceparent`、`enabled=false` 时不采纳入站值且不回写 `traceparent`（仍生成 trace 供日志）
- [x] 9.2 `pkg/gateway/tracing_test.go`：span 名等于路由模板且不含具体 id；无上游 trace 时不阻断链
- [x] 9.3 eventbus：见 5.4
- [x] 9.4 `internal/metrics/observability_test.go`：seeded 族抓取前即存在、runtime 标签族观察后出现、标签名契约（把基数护栏钉成测试）
- [x] 9.5 SSE gauge 的 handler 级测试（2026-10-18 补齐）：`internal/handler/sse_metrics_test.go`——两个 Stream 各一条用例，断言「建连后 gauge=1、断开后回 0」，并顺带钉住首帧内容（notification `{"unread":N}`、realtime 信封 `type/channel/payload`）以证明流确实进入循环而非只在建连处计数。脚手架要点：Iris app + `httptest.NewServer`、中间件注入 `pkg.CtxUserID`、真实 `testnats.NewKVStore`、以「响应头到达」为同步点（net/http 首次 Flush 后才发头）、gauge 断言用带超时轮询（2s/10ms）并在用例开头先断言基线 0。**偏离**：未用 `prometheus/testutil`（会引入 go.mod 未 require 的 `kylelemons/godebug`），改用已在依赖里的 `client_model/go` `dto.Metric` 读 gauge，go.mod/go.sum 零改动
- [x] 9.6 `login_failures_total` 三 reason 覆盖（2026-10-18 补齐）：`internal/service/auth_login_failures_test.go`——unknown_user / no_credential / bad_password 三个分支各自「本 reason 增量恰为 1、另两个为 0」（不合并的判据），外加成功登录不计数、三常量互异防粘贴合并。复用 `invite_register_test.go` 既有 doubles（`newTestAuth` / `inviteTestUserRepo` / `inviteTestCredRepo` / `fakeSessionStore`），`secMgr`/`banSvc`/`mfaSvc` 留 nil 即可（`Login` 逐个 nil 判断）。**偏离**：同样不用 `testutil`，改用 `metrics.Registry.Gather()` 按 reason 标签取值（与 `ratelimit_test.go`、`eventbus/trace_test.go` 的既有先例一致）
- [x] 9.7 回归（冒烟实测）：`GET /api/v1/metrics` → **404**；`GET :9091/metrics` → **200 / 18.9KB**，KV 42 序列、permission/sse/login/webhook/db_pool 族齐；`/api/v1/health` 响应头 `X-Request-Id` 为 32hex 且 `Traceparent` 同 trace 不同 span；完成日志含 `route=/api/v1/health`
- [x] 9.8 `go build ./...` / `go vet ./...` / `go test ./...` 退出码均为 **0**（15 包 ok，零 FAIL）。注：会话开始时的基线里 `pkg/gateway` 测试编译失败（`resp.RawCookies` 等 4 处），期间被并行会话修好，现全绿
- [x] 9.9 `docker compose -f docker-compose.cluster.yml config` 退出码 **0**（此前因 8.7 必然失败）；缺 `GRAFANA_ADMIN_PASSWORD` 时按设计报 `required variable ... is missing a value`。**偏离**：该命令不 stat 绑定挂载源，故另跑 compose 内 5 个 `./` 挂载源存在性检查（全部 OK）；spec 已按事实改写
- [x] 9.10 冒烟：`LOG_FORMAT=text` 输出为 logrus 文本行；`LOG_LEVEL=warn` 时 `request completed`(info) 行归零——级别通路实测生效。**偏离**：`LOG_LEVEL=debug` 无正向可验证对象（非测试代码 debug 调用点为 0），spec 场景已改为 warn 反向判据；`METRICS_ENABLED=false` 时指标端口无监听

## 10. 文档与规范同步

- [x] 10.1 `internal/metrics/AGENTS.md` 重写：文件表补 7 个新文件与 `http.go`，纠正暴露面（原称 `/api/v1/metrics` 为 public 是错的，实际曾为 admin 门控，现为 9091 内部端口），补基数护栏与 seed 策略
- [x] 10.2 根 `AGENTS.md`：环境变量表补 `LOG_*`/`METRICS_PORT`/`TRACING_ENABLED` 并改写 `METRICS_ENABLED`；Logger 小节改 `trace_id`；网关小节说明 `Tracing` 已实化
- [x] 10.3 `internal/middleware/AGENTS.md`：三条栈顺序补 `Trace` 并写明它 MUST 早于 `Logger` 的原因；中间件表新增 `Trace` 行、更新 `Logger` 字段
- [x] 10.4 `internal/router/AGENTS.md` 端点树去掉 `/metrics`；`internal/pkg/AGENTS.md` 补 `tracing/` 子包并重写 logger 段
- [x] 10.5 `docs/ARCHITECTURE.md` / `docs/backend-overview.md` 可观察性小节：归档后由后续会话补齐（ARCHITECTURE 有「可观测性」要点行，backend-overview §9.1 为完整小节）；2026-10-18 仅同步其中 seed 策略段（见 11.2）
- [x] 10.6 spec/design/tasks 回写实现期发现：`PublishMsg` API 名、`miss` 结果档、seed 偏离、`compose config` 不校验挂载源、无 debug 调用点、5.3 前提有误、blackbox 与 NATS 锚点修复

## 11. 归档后跟进（2026-10-18）

- [x] 11.1 测试基线修复：`internal/middleware/auth_test.go` / `auth_test_helpers.go` 是**未跟踪**的陈旧遗留文件（Aug 28-29，引用 2026-10-14 已删除的 `JWTAuth`/`OptionalJWTAuth`/`BanCheck`/`RequireRole`/`RequireOwnerOrPermission`/`Deps.Blacklist`），导致 `go test ./...` 该包编译失败。处理：迁移其中仍有效的 `AuthGate`(6) / `SpaceAuthGate`(5) / `RequirePermission`(4) 用例到新增的 `internal/middleware/auth_gate_test.go`（含 `fakeSpace` / `mockSettingReader` doubles），删除两份陈旧文件。现 `go test ./...` 全绿
- [x] 11.2（既存缺陷）`MetricsFamilyMissing` 告警两处错误：① `absent()` 用了 histogram 基础名 `orinforum_kv_operation_duration_seconds`——TSDB 里只有 `_bucket/_sum/_count`，该子句**恒为真**，告警会永久触发；② 三个受探族都只在运行时产生标签、原先不 seed，健康但安静的实例 15 分钟后同样误报。修复：`absent()` 改探 `_count`；并由生产侧接线点 seed 零值——`eventbus.AllEventTypes()` + `eventbus.New` → `metrics.SeedEventbusEvent`、`kvstore.NewNATSStore` → `SeedKVOperation`（op 名收成常量 + `observedOps` 单一列表）、`middleware.RateLimit` 与 `gateway.NewRateLimiter` → `SeedRatelimitPolicy`。依赖方向保持 生产方 → metrics。`observability_test.go` 增 `TestProducerSeedingExportsRuntimeFamilies`（含「零观测 histogram 仍导出 `_count`」断言）
- [x] 11.3 冒烟（隔离副本 `PORT=18080`/`METRICS_PORT=19091`，避开共享 `.env`）：`/api/v1/metrics` → 404、`:19091/metrics` → 200/23KB；安静实例上三族已在（`eventbus_publish_total` 12 序列、`kv_operation_duration_seconds_count` 9 序列、`ratelimit_rejections_total` 13 序列）；失败登录实测 `unknown_user=2` / `bad_password=1` / `no_credential=0`；入站 `traceparent` 沿用同 trace id 换新 span，畸形（大写）丢弃重生成，`X-Request-Id` == trace id；完成日志含 `trace_id`/`span_id`/`route`/`status`/`latency_ms`
