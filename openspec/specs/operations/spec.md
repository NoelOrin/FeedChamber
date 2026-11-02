# operations Specification

## Purpose
定义 Feed Chamber 的运行配置、环境变量、启动方式与 API 测试流程约定，作为运维与本地开发的一致依据。
## Requirements
### Requirement: 环境变量配置

系统 MUST 通过环境变量配置运行参数，关键变量包括：`PORT`（默认 8080）、`DB_TYPE`（sqlite/mysql/postgres，默认 sqlite）、`DB_DSN`、`STORAGE_TYPE`（local/s3，默认 local）、S3 系列变量、`NATS_URL`（空则启动内嵌 nats-server）、`NATS_BUCKET`（默认 feedchamber_kv）、`NATS_EMBEDDED_PORT`。

`JWT_SECRET` MUST NOT 再作为配置项存在：该变量已随令牌 opaque 化退场，未设置、留空或保留旧值 MUST NOT 影响启动，且 MUST NOT 被任何代码路径读取。系统唯一保留的 JWT 是 OIDC `id_token`（OIDC Core 强制其为签名 JWT），其签名私钥以 PEM 持久化在 settings `sso_oidc_signing_key` 中，不依赖环境变量。

OAuth2/OIDC access token 的生命周期 MUST 由 settings `oauth_access_token_ttl`（默认 86400 秒；settings 缺失时回退环境变量 `OAUTH_ACCESS_TOKEN_TTL`，再回退默认值）管理，进程启动时一次性加载；OIDC `id_token` 的生命周期 MUST 由 settings `sso_id_token_ttl`（默认 3600 秒，签发时读取）管理。两者 MUST NOT 是启动必填环境变量，也 MUST NOT 因缺失而阻止启动。

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

#### Scenario: 不设置 JWT_SECRET 也能启动

- WHEN 部署环境未设置 `JWT_SECRET`，或仍保留旧值
- THEN 应用 MUST 正常启动，MUST NOT 因该变量缺失或取到默认值而拒绝启动，且 MUST NOT 把它当作任何凭证的签名密钥

#### Scenario: 令牌有效期由 settings 决定

- WHEN 管理员修改 settings 中的 `oauth_access_token_ttl`
- THEN 新值 MUST 在进程重启后对新签发的 access token 生效（该值启动时一次性加载），且响应的 `expires_in` MUST 与之一致
- WHEN 管理员修改 settings 中的 `sso_id_token_ttl`
- THEN 此后签发的 `id_token` MUST 立即采用新 TTL（签发时读取），MUST NOT 需要重启

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

### Requirement: API 测试流程

仓库根目录 MUST 提供 `feedchamber_test.sh` 全量 API 测试脚本。启动服务后 MUST 支持 `-all` 全量测试与按模块测试（如 `-auth -post -user`）；建议在全新 SQLite 数据库上运行。

#### Scenario: 全量接口回归

- WHEN 在干净 SQLite 实例上运行 `bash feedchamber_test.sh -all`
- THEN MUST 覆盖全部模块端点并给出通过/失败结果
