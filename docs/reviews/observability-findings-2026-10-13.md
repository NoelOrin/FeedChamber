# 可观察性建设 · 问题记录

> 日期：2026-10-13
> 来源：openspec change `2026-10-13-build-observability` 的实现过程
> 范围：`app/server`（Go 后端）与 `app/server/deploy`；不含前端 RUM（本轮明确排除）
> 逐条任务状态见 `openspec/changes/2026-10-13-build-observability/tasks.md`，本文只记问题与处置，不重复进度

---

## 1. 本轮发现的既存缺陷

### 1.1 已在本次修复

| # | 问题 | 证据 | 原影响 |
|---|---|---|---|
| F1 | cluster compose 挂载 `./prometheus.yml`，但该文件在仓库中不存在 | `deploy/docker-compose.cluster.yml:90`（原 `volumes:` 起于 `:89`）与 `ls deploy/` | Prometheus 容器起不来，整套监控栈形同虚设 |
| F2 | NATS 锚点 `&nats` 内含 `depends_on: [nats-2, nats-3]`，被 `<<: *nats` 复制给 nats-2/nats-3，形成自依赖 | `docker compose ... config` 报 `dependency cycle detected: nats-2 -> nats-2` | cluster profile 无法通过任何 compose 校验 |
| F3 | Grafana 管理员口令明文 `admin` 写死在 compose 里 | `docker-compose.cluster.yml:97`（原） | 弱口令直接上线 |
| F4 | 网关 `Tracing` 是空函数体，却在流水线首位被每请求调用 | `pkg/gateway/tracing.go:16-19`（原 `// TODO: implement in Task 3`），调用点 `gateway.go:45,94` | 「有 tracing」是假象，全链路无 trace |
| F5 | `logger.Config{Level,Format,Output}` 是死代码：全仓无 `logger.New`/`SetGlobal` 的生产调用 | `internal/pkg/logger/logger.go:44-49,119-121` | 生产改不动日志级别与格式 |
| F6 | `request_id` 只写进 iris `Values()` 与响应头，从不进 `request.Context()` | `internal/middleware/middleware.go:37-46`（原） | Handler→Service 边界丢关联，日志无法串链 |
| F7 | logger 包 4 个符号（`ContextWithRequestID`/`ContextWithUserUUID`/`GetFromValues`/`ValuesGetter`）零生产调用者 | 全仓 grep 唯一使用者是 `middleware.go:41` 的 `logger.Global()` | 死码；`WithContext` 读的是没人写的 key |
| F8 | `/api/v1/metrics` 曾为 admin 门控，Prometheus 匿名抓取拿不到数据，而文档称其 public | `router/router.go:119`（原） vs `internal/metrics/AGENTS.md:16` | 指标可抓性 + 文档误导 |

### 1.2 未修复，留待后续

| # | 问题 | 证据 | 影响 | 建议处置 |
|---|---|---|---|---|
| G1 | **两套限流实现并存**，且标着 Deprecated 的那套才是主力 | `internal/middleware/ratelimit.go:1-2` 头部写「Deprecated: Use pkg/gateway.RateLimiter」，但 `router/router.go:56,59,62,65,68` 有 5 处真实注册（register/login/forgot/reset/mfa）；`gateway/ratelimit.go` 只服务 protected party 的全局 IP 限流 | `feedchamber_ratelimit_*` 的 `name` 标签跨两种语义（endpoint 名 vs 常量 `gateway`），看板与告警要同时看两处；文件头的 Deprecated 是误导 | 二选一：把 5 个命名端点迁入 gateway 的分级限流配置，或删掉错误注释并承认两套分工。**2026-10-28 复核：错误注释已删除，两套分工已在两侧文件头写明并共用同一引擎（`pkg/ratelimit`），该项已消解** |
| G2 | 告警阈值全部是拍的初值，无 SLO 依据 | `deploy/prometheus/rules/feedchamber.yml`（5xx>5%、P99>1s、登录失败>50/5m 等） | 上线初期大概率要么不响要么狼来了 | 跑满两周真实流量后按分位回归校准；`LoginFailureSpike` 的 50 应改成相对基线（如 5 倍均值）。**2026-10-28：`LoginFailureSpike` 已改为「> 7 天均值的 5 倍，下限 10」；5xx / P99 / DB 池三条需真实分位数，仍待回归** |
| G3 | 埋点空白区：NATS 连接状态/内嵌 server、SSE 写失败与重连、`webhook.Enqueue` 积压、GORM 慢查询、权限缓存规模、备份任务成败 | 这些子系统当前无指标（本轮只覆盖 spec 表列的 7 族） | 相应故障仍只能靠日志事后翻 | 按故障代价排序补：NATS 断连与 SSE 写失败优先级最高（两者静默）。**2026-10-28：全部六项已补**——NATS 连接状态、SSE 写失败、webhook 积压（修正原缺陷）、GORM 慢查询、权限缓存规模、备份任务成败；见下「后续处理」 |
| G4 | `METRICS_PORT` 无鉴权，靠部署纪律兜底 | `metrics/http.go` 不做认证；compose 用 `expose` 而非 `ports` | 有人把 `expose` 改成 `ports` 或加条 nginx location，DB 池/流量内部数就公网可读 | 加一条 CI 检查：compose 中 backend 不得出现 `published`/`9091` 端口映射；或给 listener 加静态 token。**2026-10-28：已落地 CI 检查（`scripts/check-metrics-exposure.sh` + `metrics-exposure` job），静态 token 未做** |
| G5 | nginx 仅在 `/api/v1/` 透传 `X-Request-ID` | `deploy/nginx/nginx.conf`：`/swagger/` 与前端 location 未设该头 | 前端与后端日志无法用同一 id 对齐 | 若要做端到端关联，需在 `location /` 一并 `proxy_set_header X-Request-ID $req_id`（前端 SSR 侧还要读它）。**2026-10-28：两处 location 已补，且前端 SSR 经 `AuthAdapter.getSsrRequestId()` 转发该头** |
| G6 | `FlushViewCounts` 等 3 处 `context.Background()` 属刻意豁免 | `internal/handler/handler.go:85,116,144`（刷计数 / health 的 db、kv 超时） | 这几条路径的日志不带请求 trace（本就非请求作用域） | 已写入 `layered-architecture` 的「后台任务自成 trace 根」要求；刷计数可考虑起 trace root |
| G7 | 「不引入 OpenTelemetry」这条不可逆取向没有 ADR | `docs/adr/` 目前只有 `chat-mvp.md`；决策记录只在 change 的 `design.md` 决策 1 | change 归档后，为什么不用 OTel 的语境只剩提案文件，容易被后人当技术债推翻 | **2026-10-28：已补 `docs/adr/no-otel-sdk-yet.md`**（决策 + 保留 traceparent 兼容缝的理由） |

---

## 2. 文档与规范中的错误

| # | 位置 | 错误表述 | 事实 | 备注 |
|---|---|---|---|---|
| D1 | `openspec/specs/operations/spec.md:24,28` | 「后端 MUST 通过 `go run main.go dev`（或 `prod`）启动；Cobra CLI MUST 分发 `dev`/`prod` 子命令」 | `cmd/root.go` 只有 `RootCmd.Run` + 持久标志 `-e/--env`（默认 dev），**不存在** `dev`/`prod` 子命令。实测 `go run main.go prod` → `unknown command "prod" for "feedchamber"` | 规范文件本身错，非文档滞后。修正须走 openspec 提案（AGENTS 规定 specs 为单一可信来源） |
| D2 | `AGENTS.md:14` | 「cmd/root.go # Cobra 根命令，分发 dev / prod 子命令」 | 同上 | — |
| D3 | `AGENTS.md:416`（运行小节） | `cd app/server && go run main.go dev` | 应为 `go run main.go`（默认 dev）或 `go run main.go -e prod` | 照抄会直接启动失败 |
| D4 | `app/server/AGENTS.md:10` | 同 D2 | 同上 | — |
| D5 | `internal/metrics/AGENTS.md:16`（本轮已改） | 「Metrics handler 在 router.go 以 `GET /api/v1/metrics` 注册（public）」 | 曾经实际是 admin 门控，现已迁到 `METRICS_PORT` 独立 listener | 属 F8 的一部分 |
| D6 | `internal/router/AGENTS.md:19`（本轮已改） | 端点树列有 `/metrics GET (protected, perm:admin:metrics)` | 该路由已删除 | — |
| D7 | `internal/middleware/ratelimit.go:1-2` | 「Deprecated …kept for backward compatibility」 | 与 G1 冲突：它承载 5 个真实命名限流端点 | — |

---

## 3. 实现期踩到的技术陷阱（可复用的教训）

| # | 陷阱 | 说明 |
|---|---|---|
| T1 | **Iris 全局中间件先于 per-party 中间件** | `app.Use(Logger())` 在 `party.Use(gw.Handler())` 之前跑完排队。因此 trace 必须建在全局中间件里，只在网关 `Tracing` 里建立的话请求完成日志永远没有 `trace_id`。这是本轮最容易写错的一处，已有专门回归测试 `internal/middleware/trace_test.go` |
| T2 | 管道吞退出码导致误判构建状态 | 形如 `go vet ./... 2>&1` 接 `tail -5` 再接 `&& echo OK` 的组合，退出码来自最后那个 `tail`，编译失败也照报 OK。正确写法是先把测试输出重定向到文件再单独取 `$?`：`go test ./... >/tmp/out 2>&1; echo $?`。本轮据此一度把不绿的基线当成绿 |
| T3 | nats.go v1.39.1 没有 `MsgPublish` | 带 header 的发布是 `nc.PublishMsg(*Msg)`。提案里写 `MsgPublish` 会编译不过 |
| T4 | `hex.DecodeString` 接受大写十六进制 | W3C `traceparent` 要求小写，必须自己校验字符集，否则 `00-AAAA…` 会被当成合法值 |
| T5 | 把返回值改成具名后，`_, err := f()` 不再合法 | `:=` 要求左侧至少一个新变量；`_` 不算。`kvstore/nats.go` 的 SetNX/Exists 两处必须改成 `=` |
| T6 | `New()` 里同步 `logrus.StandardLogger()` 会污染测试 | 副作用移到 `SetGlobal()`：启动路径一次调用，测试可安全 `New(Config{})`。由两条测试分别锁「SetGlobal 同步」与「New 不同步」 |
| T7 | 泛型常量不能当类型实参 | 原想用 `decodeID[N]` 配合 `TraceIDLen` 常量，Go 不支持以常量作类型参数；改写为两个显式函数 |
| T8 | `Registry.Gather()` 返回的标签按字母序 | `permission_decisions_total` 的 `{scope,result}` 读出是 `result,scope`；断言须排序比较，别按声明序 |
| T9 | `docker compose config` 不 stat 绑定挂载源 | 它能抓出 schema 与插值错误（含 `${VAR:?}` 必填），但**不会**发现 `./prometheus.yml` 不存在——正是 F1 长期没被发现的原因。必须另跑挂载源存在性检查 |
| T10 | `/api/v1/health` 返回 JSON 而非 Prometheus 文本 | 不能直接把它当 scrape 目标（会恒 `up=0`），健康类告警必须有 prober（本轮加了 blackbox-exporter） |
| T11 | nginx `log_format` 名称冲突是启动错误 | conf.d 里重复定义 `combined` 会让 nginx 起不来，自定义格式要换名（本轮用 `feedchamber_main`） |
| T12 | 标签集合无法在 metrics 包内静态枚举 | eventbus 事件名定义在 `eventbus/events.go`，metrics 若要 seed 这些 label 组合就得反向 import eventbus（环）。结论：只 seed 静态可知的族，其余用 `absent()` 告警兜底 |

---

## 4. 本轮的验证空白（明确承认，未标完成）

| # | 缺口 | 为什么没做 | 现有补偿 |
|---|---|---|---|
| V1 | SSE gauge 的 handler 级测试（tasks 9.5） | 计数路径需要鉴权 + KV 订阅 + 支持 Flusher 的 ResponseWriter，现有测试基建无此夹具 | 仅 `internal/metrics/sse.go` 的族级测试；Inc/Dec 靠 `defer` 的静态正确性 |
| V2 | `login_failures_total` 三 reason 覆盖测试（tasks 9.6） | `internal/service` 下没有任何测试调用 `Login`，覆盖需 mock 全套依赖 | 埋点是三行同构 `Inc()`，位置经 `auth.go:283/294/303` 逐行核对 |
| V3 | `docs/ARCHITECTURE.md` / `docs/backend-overview.md` 缺可观察性小节（tasks 10.5） | 两份文档正被并行会话编辑（本轮 root AGENTS 就被外部改写一次），此时动它们会互相覆盖 | 变更点已写进 4 个 AGENTS.md 与 spec delta |
| V4 | `LOG_LEVEL=debug` 的正向验证（tasks 9.10） | 全仓非测试代码 debug 级调用点为 0，无输出可观测 | 改用 `LOG_LEVEL=warn` 反向判据实测：info 行归零；spec 场景已按事实改写 |
| V5 | 监控栈的真实 up 运行 | Docker daemon 未运行 | `compose config` 退出 0 + 5 个挂载源存在 + 后端侧指标已由本地冒烟实测（`:9091` 返 200/18.9KB、`/api/v1/metrics` 404） |

---

## 5. 协作与环境事实（多会话同仓工作的前提）

| # | 事实 | 应对 |
|---|---|---|
| C1 | 工作区有约 390 个未提交文件，属并行会话的在途工作（session-auth 重构、vinext 迁移、mongo 下线） | 本轮**未提交任何东西**：`git add -A` 会把别人的在途改动一并卷入 |
| C2 | `AGENTS.md` 与 `go.mod`/`go.sum` 会被其他会话中途改写 | 每次编辑前重读磁盘；避免引入需要 `go mod tidy` 的新依赖（曾想用 `prometheus/testutil`，改为自读 `Registry.Gather()`） |
| C3 | 基线会漂移，不能依赖记忆里的「全绿」 | 会话开始时 `pkg/gateway` 测试编译失败（`resp.RawCookies` 等 4 处），结束前已被并行会话修好；开工即测 |
| C4 | Go 在非登录 shell 的 PATH 里不可见 | `export PATH="/usr/local/go/bin:$PATH"` 或 `zsh -lc` |
| C5 | 本机没有 `openspec` CLI | 产物按仓内既有 change 的格式手写并对齐 delta 头（`## ADDED/MODIFIED Requirements`），**未经 `openspec validate` 校验**；归档前建议装 CLI 跑一次 |
| C6 | `go run main.go` 会占用 8080/9091 等共享端口 | 冒烟统一用 1808x/1809x 高位端口，跑完即杀并复核监听为空 |

---

## 6. 后续处理（2026-10-28，change `2026-10-28-observability-gaps`）

本节记录 G3/G4/G5 的落地，不修改上面的原始评审内容（那是当时的记录）。

| 原项 | 处置 | 关键事实 |
|---|---|---|
| G3 · NATS 连接状态 | **已补** `feedchamber_nats_connected{client}` / `_disconnects_total` / `_reconnects_total` | 两处 `nats.Connect` 都以 `MaxReconnects(-1)` 无限重试，进程永不退出——断连原本只在日志里偶然出现。2026-10-28 的改动把 NATS 从「内嵌可选」变成「外部必填」，故障代价从降级升为「会话缓存/限流/事件总线/权限同步全失效」，故优先级最高 |
| G3 · SSE 写失败 | **已补** `feedchamber_sse_write_failures_total{channel}` | 与连接 gauge **分开**：gauge 下降包含「客户端正常关标签页」，无法承担投递失败语义 |
| G3 · webhook 积压 | **已修正（原实现是缺陷，不只是空白）** | gauge 原取 `len(ListPendingDeliveries(ctx, 20))`，20 是 worker 批大小，积压 500 时仍显示 20，且任何基于它的告警**结构性不可触发**。改为 `CountPendingDeliveries` 的 `COUNT(*)`，新增 `WebhookBacklogGrowing` 告警 |
| G3 · GORM 慢查询 | **已补** `feedchamber_db_queries_total` + `feedchamber_db_slow_queries_total`（>200ms） | 默认 GORM logger 把错误写标准库 stdout，**绕过 `LOG_LEVEL`/`LOG_FORMAT`**；慢查询既不计数也不进结构化日志。因 `metrics` 已 import `db`（连接池 collector），经 `db.SetQueryObserver` 钩子注入，避免 import cycle。SQL 只进日志不进标签（无界文本） |
| G3 · 权限缓存规模 | **已补** `feedchamber_permission_cache_entries{cache}` | 三个内存 map 此前无任何规模信号。取值在 `LoadCache` 持写锁期间设置，是重建完成态而非部分填充态；缓存收缩时 gauge 随之下降（非高水位） |
| G3 · 备份成败 | **已补** `feedchamber_backup_jobs_total{kind,outcome}` + `feedchamber_backup_job_duration_seconds{kind}` | 备份失败会留下「上一条仍 ready」的列表，只有一行日志，直到需要恢复那天才暴露。outcome 由 `err` 推导，不由调用方传入 |
| G4 | **已补 CI 护栏** | `scripts/check-metrics-exposure.sh` + `metrics-exposure` CI job（排在 40 分钟镜像构建之前）。脚本用 awk 状态机按服务解析 `ports:` 块，实测能抓短语法 `9091:9091`、`127.0.0.1:9091:9091` 与长语法 `target/published: 9091` 三种形状，且不会把 postgres 的 `ports` 误判给 backend |
| G5 | **已补端到端** | nginx 的 `/swagger/` 与前端 `location /` 补 `proxy_set_header X-Request-ID`；`/health`（nginx 本地应答）`add_header ... always`。**关键在于前端侧也接上了**：`AuthAdapter` 新增 `getSsrRequestId()`，SSR 请求把它转发给后端。只改 nginx 而前端不读，SSR 渲染与它触发的 API 调用仍会各自生成不同 id |

| G2 | **部分处置** | `LoginFailureSpike` 从绝对阈值 `>50/5m` 改为「> 该实例 7 天均值（按 5 分钟切片）× 5，且下限 10」。绝对阈值在小实例永不触发、大实例持续触发，两种情况下告警都无意义。5xx / P99 / DB 池三条需要真实分位数，本轮不动 |
| G7 | **已补 ADR** | `docs/adr/no-otel-sdk-yet.md`：决策、五条理由、**保留的兼容缝**（W3C traceparent 双向兼容，将来接入只需把 SDK 接到解析结果上）、明确承认的三项代价（无 span 树 / 无采样 / 无 OTLP） |
| D2/D3/D4 | **已修** | `AGENTS.md` 与 `app/server/AGENTS.md` 仍写「Cobra 分发 dev/prod 子命令」。实际只有持久标志 `-e/--env`，`go run main.go dev` 会报 `unknown command`；真正的子命令只有 `backup` / `import-discourse` |

指标族 20 → **29**，告警 13 → **18** 条，Grafana 面板 9 → **13**。`go build` / `go vet` / `go test ./...` 全绿（19 包），`tsc` 前后端均通过。

慢查询分类经实测确认：`TestSlowQueryIsClassified` 用递归 CTE 跑出 333ms，被正确标记并写出带 `latency_ms`/`rows`/`sql` 的 warn 日志；运行实例上 `db_queries_total=1140`（启动期建表与 seed 查询全被统计）证明跨包钩子真的在计数，而不是只有 seed 值。

一处**语义断点**需要运维知情：`feedchamber_webhook_pending_deliveries` 名称未变，但取值从「批大小（≤20）」变为「真实积压」。历史数据与新数据不可直接接续，看板应以改动时间分段解读。
