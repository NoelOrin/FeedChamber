# operations Specification (delta)

## MODIFIED Requirements

### Requirement: 环境变量配置

系统 MUST 通过环境变量配置运行参数，关键变量包括：`PORT`（默认 8080）、`JWT_SECRET`、`DB_TYPE`（sqlite/mysql/postgres，默认 sqlite）、`DB_DSN`、`STORAGE_TYPE`（local/s3，默认 local）、S3 系列变量、`NATS_URL`（空则启动内嵌 nats-server）、`NATS_BUCKET`（默认 orin_kv）、`NATS_EMBEDDED_PORT`。

Storage 配置 MUST 以 settings 表为唯一可信来源，环境变量仅作回退（H-04 修复）。

#### Scenario: 内嵌 NATS 回退

- WHEN `NATS_URL` 为空
- THEN 系统 MUST 启动进程内 nats-server（JetStream KV），支撑令牌黑名单、限流、OAuth state、权限多实例同步

#### Scenario: Storage 配置优先级

- WHEN settings 表存在存储配置
- THEN `BuildStorageConfig()` MUST 优先采用 settings 表值，环境变量仅作缺失回退

#### Scenario: DB_TYPE 取值域校验

- WHEN `DB_TYPE` 取值不在 sqlite/mysql/postgres 之内
- THEN `DBConfig.Validate()` MUST 返回不支持的后端错误