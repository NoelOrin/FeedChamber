# operations Specification (delta)

## MODIFIED Requirements

### Requirement: 环境变量配置

系统 MUST 通过环境变量配置运行参数，关键变量包括：`PORT`（默认 8080）、`JWT_SECRET`、`DB_TYPE`（sqlite/mysql/postgres，默认 sqlite）、`DB_DSN`、`STORAGE_TYPE`（local/s3，默认 local）、S3 系列变量、`NATS_URL`（空则启动内嵌 nats-server）、`NATS_BUCKET`（默认 feedchamber_kv）、`NATS_EMBEDDED_PORT`。

可观察性相关变量 MUST 同样由环境变量提供并带默认值：`LOG_LEVEL`（默认 `info`）、`LOG_FORMAT`（默认 `json`，可选 `text`）、`LOG_OUTPUT`（默认 `stderr`，可为文件路径）、`METRICS_ENABLED`（默认 `true`，控制 Prometheus 中间件与指标 listener）、`METRICS_PORT`（默认 9091，指标独立暴露端口）、`TRACING_ENABLED`（默认 `true`）。

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

#### Scenario: 关闭可观察性

- WHEN `METRICS_ENABLED=false`
- THEN 系统 MUST NOT 注册 Prometheus 中间件、MUST NOT 启动 `METRICS_PORT` listener，且 MUST NOT 因此影响业务请求处理
- WHEN `LOG_OUTPUT` 指定的文件无法创建
- THEN 系统 MUST 回退到 `stderr` 并输出一条告警，MUST NOT 中断启动

## ADDED Requirements

### Requirement: 集群监控栈运行与校验

cluster 部署 MUST 可一条命令拉起可用的监控栈：`cd app/server/deploy && docker compose -f docker-compose.cluster.yml up -d` 后 Prometheus（9090）与 Grafana 可访问，Prometheus 抓取目标包含 backend 的指标端口。compose 引用的每个挂载文件（`./prometheus.yml`、告警规则目录、Grafana provisioning 目录）MUST 存在于仓库中；服务口令与密钥 MUST 经环境变量注入而非写入 compose 明文。

本地校验 MUST 可在 Docker daemon 未运行时完成：`docker compose -f docker-compose.cluster.yml config` MUST 成功；由于该命令**不会**检查绑定挂载的源路径是否存在，缺失文件 MUST 由一条对 compose 中每个 `./` 挂载源做存在性检查的脚本发现（两者都纳入验收）。

#### Scenario: 缺失挂载文件被发现

- WHEN compose 中某 `volumes` 项指向仓库中不存在的路径
- THEN `docker compose ... config` MUST NOT 报错（它不 stat 源路径），该缺陷 MUST 由挂载源存在性检查发现并纳入变更验收

#### Scenario: 非 cluster 部署不含监控栈

- WHEN 使用 `docker-compose.basic.yml` / `.lite.yml` / `.full.yml` / `docker-compose.yml`
- THEN MUST NOT 要求 Prometheus 或 Grafana 存在，backend 在无监控栈时 MUST 正常启动
