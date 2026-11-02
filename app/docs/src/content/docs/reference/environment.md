---
title: 环境变量
description: 后端与前端读取的环境变量、默认值，以及它们和 settings 表的关系
---

环境变量只负责**启动一个进程所需的东西**：监听端口、数据库连接、NATS 地址、日志。
站点行为（注册开关、审核模式、邮件、OAuth、上传大小）属于运行时配置，住在 `settings` 表，
改完即时生效，不需要重启，也不该写进 compose 文件。

一句话记法：**改行为去后台，改依赖重启。**

## 后端

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | HTTP 监听端口 |
| `DB_TYPE` | `sqlite` | `sqlite` / `mysql` / `postgres` |
| `DB_DSN` | 空 | `mysql` / `postgres` **必填**；`sqlite` 下是文件路径 |
| `DB_MAX_OPEN_CONNS` | `25` | 连接池最大连接数 |
| `DB_MAX_IDLE_CONNS` | `10` | 连接池最大空闲连接 |
| `DB_CONN_MAX_LIFETIME` | `300` | 连接最长存活（秒） |
| `DB_CONN_MAX_IDLE_TIME` | `60` | 空闲连接回收（秒） |
| `NATS_URL` | 空 | **必填**。外部 NATS 地址，必须启用 JetStream；为空时启动直接失败 |
| `NATS_BUCKET` | `feedchamber_kv` | JetStream KV bucket 名 |
| `ADMIN_PASSWORD` | 空 | 首次 seed 的 admin 密码；为空则随机生成并打印到日志 |
| `SITE_URL` | 空 | 仅在没有 `site_url` 设置时兜底，用于 sitemap / canonical |
| `METRICS_ENABLED` | `true` | 是否启动 9091 指标 listener |
| `METRICS_PORT` | `9091` | 指标端口，**只在内部网络可达** |
| `TRACING_ENABLED` | `true` | 是否解析入站 `traceparent` 并回写 |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `json` | `json` / `text` |
| `LOG_OUTPUT` | `stderr` | `stdout` / `stderr` 或文件路径 |

### 存储与邮件

这两组变量的优先级**低于** `settings` 表：表里有值就用表里的，空值才回退到环境变量。
后台改完存储配置需要重启（存储是启动期单例）。

| 变量 | 对应设置键 | 默认值 |
|------|-----------|--------|
| `STORAGE_PROVIDER` | `storage_provider` | `local` |
| `STORAGE_LOCAL_DIR` | `storage_local_dir` | `uploads` |
| `S3_ENDPOINT` | `s3_endpoint` | 空 |
| `S3_REGION` | `s3_region` | `us-east-1` |
| `S3_BUCKET` | `s3_bucket` | 空 |
| `S3_ACCESS_KEY_ID` | `s3_access_key_id` | 空 |
| `S3_ACCESS_KEY_SECRET` | `s3_access_key_secret` | 空 |
| `S3_USE_SSL` | `s3_use_ssl` | `true` |
| `S3_CUSTOM_DOMAIN` | `s3_custom_domain` | 空 |
| `EMAIL_PROVIDER` | `email_provider` | `log` |
| `EMAIL_FROM` | `email_from` | 空 |
| `EMAIL_FROM_NAME` | `email_from_name` | `Feed Chamber` |
| `EMAIL_SMTP_HOST` | `email_smtp_host` | 空 |
| `EMAIL_SMTP_PORT` | `email_smtp_port` | `587` |
| `EMAIL_USERNAME` | `email_username` | 空 |
| `EMAIL_PASSWORD` | `email_password` | 空 |
| `EMAIL_USE_TLS` | `email_use_tls` | `false` |
| `EMAIL_API_KEY` | `email_api_key` | 空 |

### 时区

后端没有单独的时区变量：签到日期、统计窗口都按进程本地时区计算。需要固定时区时给容器加 `TZ`：

```yaml
environment:
  TZ: Asia/Shanghai
```

## 前端（构建期）

Web 端的公开后端地址在构建时注入；同源部署（nginx 反代 `/api/v1`）不需要设置。

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NEXT_PUBLIC_API_URL` | 空（同源） | 前后端分域部署时的 API 基地址 |
| `DOCS_SITE` | `https://noelorin.github.io` | 仅文档站，GitHub Pages 的站点地址 |
| `DOCS_BASE` | `/` | 仅文档站，项目页子路径前缀 |

本地起前端用 `pnpm dev:web`，默认连 `http://localhost:8080`。

## 启动校验

`config.Validate()` 在启动时检查，任何一条不满足都**直接退出**，不会带病运行：

1. `DB_TYPE` 必须是 `sqlite` / `mysql` / `postgres`
2. `mysql` / `postgres` 必须有 `DB_DSN`
3. `NATS_URL` 不能为空
4. `METRICS_ENABLED=true` 时 `METRICS_PORT` 必须在 1–65535

:::tip[容器部署]
`deploy/docker-compose.yml` 已经把 Postgres、NATS、上传卷与 nginx 配好；
只有 `ADMIN_PASSWORD` 建议显式覆盖。见 [Docker Compose 安装](/install/docker-compose/)。
:::
