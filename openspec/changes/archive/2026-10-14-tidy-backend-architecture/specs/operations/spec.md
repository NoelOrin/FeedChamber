# operations Specification (delta)

## MODIFIED Requirements

### Requirement: 环境变量配置

系统 MUST 通过环境变量配置运行参数，关键变量包括：`PORT`（默认 8080）、`JWT_SECRET`、`DB_TYPE`（sqlite/mysql/postgres，默认 sqlite）、`DB_DSN`、`STORAGE_TYPE`（local/s3，默认 local）、S3 系列变量、`NATS_URL`（空则启动内嵌 nats-server）、`NATS_BUCKET`（默认 feedchamber_kv）、`NATS_EMBEDDED_PORT`。

可观察性相关变量 MUST 同样由环境变量提供并带默认值：`LOG_LEVEL`（默认 `info`）、`LOG_FORMAT`（默认 `json`，可选 `text`）、`LOG_OUTPUT`（默认 `stderr`，可为文件路径）、`METRICS_ENABLED`（默认 `true`，控制 Prometheus 中间件与指标 listener）、`METRICS_PORT`（默认 9091，指标独立暴露端口）、`TRACING_ENABLED`（默认 `true`）。

Cookie 安全位由 `MASTER_COOKIE_SECURE` 控制（默认 false）；`ORIN_ADMIN_PASSWORD` 为空时 MUST 随机生成初始 admin 口令并打印，MUST NOT 使用可猜测的默认口令。

Storage 配置 MUST 以 settings 表为唯一可信来源，环境变量仅作回退（H-04 修复）。

#### Scenario: 内嵌 NATS 回退

- WHEN `NATS_URL` 为空
- THEN 系统 MUST 启动进程内 nats-server（JetStream KV），支撑会话 read-through 缓存、限流计数、OAuth state、View Count 缓冲、Settings/Ban 缓存与权限多实例同步（Pub/Sub）

#### Scenario: Storage 配置优先级

- WHEN settings 表存在存储配置
- THEN `BuildStorageConfig()` MUST 优先采用 settings 表值，环境变量仅作缺失回退

#### Scenario: DB_TYPE 取值域校验

- WHEN `DB_TYPE` 取值不在 sqlite/mysql/postgres 之内
- THEN `DBConfig.Validate()` MUST 返回不支持的后端错误

#### Scenario: 关闭可观察性

- WHEN `METRICS_ENABLED=false`
- THEN 系统 MUST NOT 注册 Prometheus 中间件、MUST NOT 启动 `METRICS_PORT` listener，且 MUST NOT 因此影响业务请求处理
- WHEN `LOG_OUTPUT` 指定的文件无法创建
- THEN 系统 MUST 回退到 `stderr` 并输出一条告警，MUST NOT 中断启动

### Requirement: 启动与运行命令

后端 MUST 通过 `cd app/server && go run main.go -e dev`（或 `-e prod`）启动。环境由持久标志 `-e/--env` 指定，默认 `dev`；`dev` 与 `prod` MUST NOT 作为子命令存在（`main.go dev` 实测报 `unknown command`）。Cobra 子命令仅有 `backup` 与 `import-discourse`。启动 MUST 按环境读取 `.env` 与 `.env.dev` / `.env.prod`（后者覆盖前者）。

前端开发 MUST 使用 `pnpm dev:web` 与 `pnpm dev:dashboard`。

#### Scenario: 开发模式启动

- WHEN 执行 `go run main.go -e dev`
- THEN 应用 MUST 加载 `.env.dev`、以开发配置运行 Iris 服务，并允许 dev 专属的种子内容重建

#### Scenario: 位置参数形式被拒绝

- WHEN 执行 `go run main.go dev`
- THEN Cobra MUST 以非零退出码报出 unknown command，MUST NOT 静默按默认环境启动
