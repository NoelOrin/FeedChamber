## Purpose

定义 Feed Chamber 的运行配置、环境变量、启动方式与 API 测试流程约定，作为运维与本地开发的一致依据。

## ADDED Requirements

### Requirement: 环境变量配置

系统 MUST 通过环境变量配置运行参数，关键变量包括：`PORT`（默认 8080）、`JWT_SECRET`、`DB_TYPE`（sqlite/mysql/postgres/mongo，默认 sqlite）、`DB_DSN`、`STORAGE_TYPE`（local/s3，默认 local）、S3 系列变量、`NATS_URL`（空则启动内嵌 nats-server）、`NATS_BUCKET`（默认 feedchamber_kv）、`NATS_EMBEDDED_PORT`。

Storage 配置 MUST 以 settings 表为唯一可信来源，环境变量仅作回退（H-04 修复）。

#### Scenario: 内嵌 NATS 回退

- WHEN `NATS_URL` 为空
- THEN 系统 MUST 启动进程内 nats-server（JetStream KV），支撑令牌黑名单、限流、OAuth state、权限多实例同步

#### Scenario: Storage 配置优先级

- WHEN settings 表存在存储配置
- THEN `BuildStorageConfig()` MUST 优先采用 settings 表值，环境变量仅作缺失回退

### Requirement: 启动与运行命令

后端 MUST 通过 `cd app/server && go run main.go dev`（或 `prod`）启动；Cobra CLI MUST 分发 `dev` / `prod` 子命令，读取 `.env` / `.env.dev` / `.env.prod`。前端开发 MUST 使用 `pnpm dev:web` 与 `pnpm dev:dashboard`。

#### Scenario: 开发模式启动

- WHEN 执行 `go run main.go dev`
- THEN 应用 MUST 加载 `.env.dev` 并以开发配置运行 Iris 服务

### Requirement: API 测试流程

仓库根目录 MUST 提供 `feedchamber_test.sh` 全量 API 测试脚本。启动服务后 MUST 支持 `-all` 全量测试与按模块测试（如 `-auth -post -user`）；建议在全新 SQLite 数据库上运行。

#### Scenario: 全量接口回归

- WHEN 在干净 SQLite 实例上运行 `bash feedchamber_test.sh -all`
- THEN MUST 覆盖全部模块端点并给出通过/失败结果
