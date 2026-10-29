## Why

后端「有指标」不等于「可观察」。现状 `internal/metrics/` 已注册独立 Registry 与 4 个 HTTP 指标，`/api/v1/health` 也做了 db/KV/storage 探活，但三处是**看起来存在、实际不生效**的死线：

1. `pkg/gateway/tracing.go:16-19` 函数体只有 `// TODO: implement in Task 3`，却排在网关流水线首位（`gateway.go:45`、`:94`）——每个请求都调用一个空函数，全链路无 trace。
2. `internal/pkg/logger/logger.go:44-49` 定义了 `Config{Level,Format,Output}`，但全仓没有 `logger.New()` / `SetGlobal()` 的生产调用点（`init()` 用零值，`logger.go:119-121`）——生产环境改不动日志级别，`Config` 是死代码。
3. `middleware/middleware.go:37-46` 只把 `request_id` 写进 iris `Values()` 与响应头，从不写入 `request.Context()`；`logger.ContextWithRequestID`（`logger.go:106`）除 `logger_test.go:31` 外无调用者。而 Handler 一律用 `ctx.Request().Context()`（`handler/auth.go:88` 等）、Service 一律收 ctx 首参（`service/auth.go:276`）——**关联标识在 Handler→Service 边界被静默丢弃**，日志无法串成一条链。

此外 `deploy/docker-compose.cluster.yml:89` 挂载 `./prometheus.yml`，而 `app/server/deploy/` 下根本没有这个文件（只有 5 个 compose + `nginx/`），cluster 监控栈一 `up` 就失败；Grafana 无 provisioning、`GF_SECURITY_ADMIN_PASSWORD: admin` 硬编码（`:96`）。指标侧只有 HTTP 与 DB 池两类，EventBus / KV / 限流 / 权限缓存 / SSE / 登录失败 / webhook 这些真正会出问题的环节零埋点，也没有任何告警规则。

本轮范围：**后端可观察性闭环**（地基 + tracing + 业务指标与告警）。前端 RUM 不在本轮（`app/web` 正处 vinext big-bang 迁移、故意不可部署，落点应等 P3）。

## What Changes

- **日志地基接通**：`internal/config` 新增 `LOG_LEVEL` / `LOG_FORMAT` / `LOG_OUTPUT`，`server/iris.go` 在 `cfg.Validate()` 之后、任何业务日志之前 `logger.SetGlobal(logger.New(cfg.Log))`。
- **tracing 做成真的**：新增 trace 内核包（W3C `traceparent` v00 解析/生成，128-bit trace id + 64-bit span id），实现 `Tracing.Middleware`，并把 trace 上下文 `ResetRequest` 进 `request.Context()`；`X-Request-ID` 与日志统一用同一个 id（`request_id` 字段更名为 `trace_id`）。
- **跨进程传播**：`eventbus.go:177` 的 `nc.Publish` 改为 `nc.PublishMsg`（nats.go v1.39.1 无 MsgPublish） 并携带 `traceparent` 头；消费端 `handleNATSMessage`（`eventbus.go:114`）恢复上下文，替换 `:123` 的 `Ctx: context.Background()`。nginx 增设 `proxy_set_header X-Request-ID` 与含该字段的 `log_format`。
- **指标暴露面迁移**：`/metrics` 从 `/api/v1` 业务路由（`router/router.go:119`）移除，改由 `METRICS_PORT`（默认 9091）独立监听，不经 nginx 对外暴露。
- **业务指标 7 族**：EventBus（发布/分发耗时/panic/队列深度/丢弃）、KV 操作、限流拒绝与 fail-open、权限判定、SSE 在线数、登录失败分类、webhook 投递与积压——全部注册到 `internal/metrics.Registry`。
- **部署与告警**：补 `deploy/prometheus.yml` + `deploy/prometheus/rules/feedchamber.yml`（8 条告警）+ `deploy/grafana/provisioning/`，Grafana 口令改 env 注入。

## Capabilities

### New Capabilities

- `observability` — 关联标识与追踪、日志可配置、业务指标族清单、暴露面边界、监控栈可部署性、基数护栏。

### Modified Capabilities

- `operations` — 环境变量配置补 `LOG_*` / `METRICS_PORT`（`METRICS_ENABLED` 已存在但语义收窄为控制独立 listener）；新增集群监控栈的运行与校验方式。
- `layered-architecture` — 新增「关联上下文沿依赖链向下传递」要求：网关建立、Handler 转发、Service/Repository 收 ctx，请求路径上禁止 `context.Background()`。

## Impact

- **代码**：`app/server/internal/config`、`internal/pkg/logger`、`internal/middleware`、`internal/metrics`、`internal/pkg/eventbus`、`internal/pkg/kvstore`、`internal/pkg/ratelimit`、`internal/service/{permission,auth,webhook}.go`、`internal/handler/{metrics,notification,realtime}.go`、`pkg/gateway/{tracing,gateway,ratelimit,security}.go`、`server/iris.go`、`internal/router/router.go`。
- **行为变更（对外可见）**：
  - `/api/v1/metrics` 不再可访问（现为 admin 门控），改由 `:METRICS_PORT/metrics`；`e2e-test.js` 不涉及此端点。
  - `X-Request-ID` 响应头取值格式由 UUID 变为 32 位十六进制 trace id（头名不变）。
  - 请求日志字段 `request_id` → `trace_id`；仓内无日志解析消费者。
  - 登录失败/限流拒绝等原 `fmt.Printf` 输出（`pkg/gateway/ratelimit.go:83,89`）改走结构化 logger。
- **不引入新依赖**：追踪用自研 W3C `traceparent`，不引 OpenTelemetry SDK / OTLP exporter / collector（详见 design 决策 1，保留后续替换缝）。
- **部署**：`deploy/` 新增 3 个文件、`docker-compose.cluster.yml` 改 3 处（backend env、prometheus 挂载、grafana provisioning+口令）。`basic/full/lite/yml` 不含监控栈，不动。
- **文档**：`internal/metrics/AGENTS.md:16` 现有表述错误（称 `/metrics` 为 public，实际 admin 门控），须随暴露面变更一并纠正；根 `AGENTS.md` 环境变量表同步。实现期发现的既存缺陷、文档错误（含 `operations` spec 要求的 `go run main.go dev` 子命令实际不存在）与技术陷阱统一记录在 `docs/reviews/observability-findings-2026-10-13.md`。
- **应用顺序**：本 change 的 `operations` delta 建立在 `2026-10-13-remove-mongodb-backend` 的 delta 之上（`DB_TYPE` 已无 mongo），须在其归档之后应用。
