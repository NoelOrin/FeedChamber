---
title: 环境变量
description: 后端可配置的环境变量全表
sidebar:
  order: 3
---

## 服务与数据库

| 变量 | 默认 | 说明 |
|------|------|------|
| `PORT` | `8080` | 服务端口 |
| `DB_TYPE` | `sqlite` | `sqlite` / `mysql` / `postgres` |
| `DB_DSN` | 空 | 数据库连接串 |
| `DB_MAX_OPEN_CONNS` | `25` | 连接池最大连接 |
| `DB_MAX_IDLE_CONNS` | `10` | 连接池最大空闲 |
| `DB_CONN_MAX_LIFETIME` | `300` | 连接最长存活（秒） |
| `DB_CONN_MAX_IDLE_TIME` | `60` | 空闲连接回收（秒） |

## 必填

| 变量 | 说明 |
|------|------|
| `NATS_URL` | **必填**。外部 NATS 地址，需启用 JetStream。为空则启动失败 |
| `NATS_BUCKET` | 默认 `feedchamber_kv`，JetStream KV bucket 名 |

## 存储

| 变量 | 默认 | 说明 |
|------|------|------|
| `STORAGE_TYPE` | `local` | `local` / `s3` |
| `S3_ENDPOINT` | 空 | 仅 s3 |
| `S3_BUCKET` | 空 | 仅 s3 |
| `S3_REGION` | 空 | 仅 s3 |
| `S3_ACCESS_KEY_ID` | 空 | 仅 s3 |
| `S3_SECRET_ACCESS_KEY` | 空 | 仅 s3 |

:::note[存储配置以 settings 表为唯一来源]
`STORAGE_TYPE` / `S3_*` 不在 `config.Load()` 直接读取，而是由
「settings 表 → 环境变量 → 默认值」的优先级解析。后台改了 S3 配置要重启后端。
:::

## 观测与日志

| 变量 | 默认 | 说明 |
|------|------|------|
| `METRICS_ENABLED` | `true` | 是否启用指标中间件与 listener |
| `METRICS_PORT` | `9091` | 指标端口，**仅内部可达** |
| `TRACING_ENABLED` | `true` | 是否解析入站 `traceparent` 并回写 |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `json` | `json` / `text` |
| `LOG_OUTPUT` | `stderr` | `stdout` / `stderr` / 文件路径（打不开回退 stderr） |

## 初始化

| 变量 | 默认 | 说明 |
|------|------|------|
| `ADMIN_PASSWORD` | 空 | 初始 admin 密码；空则随机生成并打日志 |

## 不在环境变量里的配置

这些从 **settings 表**读取（后台可改，部分需重启）：

| 配置 | 设置 key | 何时生效 |
|------|----------|----------|
| 站点名 / Logo / SEO | `site_name` / `site_logo` / `seo_*` | 立即 |
| 审核模式 | `moderation_mode` | 立即 |
| 注册模式 | `register_mode` | 立即 |
| 分页默认值与上限 | `page_default_size` / `page_max_size` | 立即 |
| OAuth access token TTL | `oauth_access_token_ttl` | 立即 |
| OIDC id_token TTL | `sso_id_token_ttl` | 立即 |
| 存储（S3） | `storage_*` | **需重启** |
| MCP 开关 / 工具启停 | `mcp_enabled` / `mcp_disabled_tools` | 立即 |

:::note[已废弃的变量]
`JWT_SECRET` 已从启动校验中删除，不设也能启动。
`jwt_access_token_ttl` / `jwt_refresh_token_ttl` 不再被读取。
:::
