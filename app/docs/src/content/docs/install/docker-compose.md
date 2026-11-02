---
title: Docker Compose 安装
description: 单机、NAS 或虚拟机上用 Compose 跑起完整栈，含端口、卷、密码与升级
---

这套编排在 `deploy/docker-compose.yml`，包含五个服务：

| 服务 | 作用 | 是否对外 |
|------|------|----------|
| `nginx` | 唯一入口：`/` 前端、`/api/v1` 后端、`/mcp` | 是（`HTTP_PORT`，默认 80） |
| `frontend` | React 19 SSR 进程（含 `/admin`） | 否，仅 compose 网络 |
| `backend` | Go / Iris API | 否；指标 9091 也不对外 |
| `postgres` | 主库（PostgreSQL 16） | 否 |
| `nats` | JetStream：KV + 事件总线 | 否 |

另有 `nats-exporter` 把 NATS 指标转成 Prometheus 格式，同样只在容器网络内。

## 前置条件

- Docker Engine 24+ 与 Compose v2（`docker compose version` 有输出）
- 能拉取 `postgres:16-alpine`、`nats:2.10-alpine`、`nginx:alpine`
- 80 端口空闲，或准备用 `HTTP_PORT` 换一个

## 安装

```bash
git clone https://github.com/NoelOrin/FeedChamber.git
cd FeedChamber

# 可选：显式设置管理员密码与端口
ADMIN_PASSWORD='换成你的密码' HTTP_PORT=8080 \
  docker compose -f deploy/docker-compose.yml up -d --build
```

构建期依赖下载慢时，可以换源：

```bash
GOPROXY=https://goproxy.cn,direct \
NPM_REGISTRY=https://registry.npmmirror.com \
  docker compose -f deploy/docker-compose.yml build
```

## 可覆盖的变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HTTP_PORT` | `80` | nginx 映射到宿主机的端口 |
| `ADMIN_PASSWORD` | 空 | 首次 seed 的 admin 密码；空则随机生成并打印日志 |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `feedchamber` | 数据库账号，与 `DB_DSN` 联动 |
| `SITE_URL` | 空 | sitemap / canonical / OG 的对外域名（settings 的 `site_url` 优先） |
| `MASTER_COOKIE_SECURE` | `true` | 会话 cookie 的 `Secure` 位；纯 HTTP 本地调试时设 `false` |
| `LOG_LEVEL` / `LOG_FORMAT` / `LOG_OUTPUT` | `info` / `json` / `stderr` | 后端日志 |
| `TZ`（给 postgres） | `Asia/Shanghai` | 与 `DB_DSN` 的 `TimeZone` 保持一致 |

## 验证

```bash
docker compose -f deploy/docker-compose.yml ps
# 五个服务都应为 running；postgres / nats / backend / frontend 另有 healthy

curl -s http://localhost:8080/api/v1/health   # 换成你的 HTTP_PORT
# {"code":0,"msg":"success","data":{...}}

curl -sI http://localhost:8080/ | head -1     # HTTP/1.1 200 OK
```

拿管理员密码：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i admin
```

日志里形如：

```text
WARN ADMIN_PASSWORD 未设置，已生成随机管理员密码: 1f3a…
```

## 数据在哪

| 卷 | 内容 | 备份方式 |
|----|------|----------|
| `feedchamber_pg` | Postgres 数据目录 | `pg_dump` 或卷快照 |
| `feedchamber_nats` | JetStream（KV + 事件队列） | 可重建，丢失会丢会话缓存与限流窗口 |
| `feedchamber_uploads` | 本地上传文件 | 文件级同步 / 对象存储 |

删卷等于删站点：

```bash
# 危险操作：连同数据一起删除
docker compose -f deploy/docker-compose.yml down -v
```

## 升级

```bash
git pull
docker compose -f deploy/docker-compose.yml up -d --build
```

后端启动时会跑 `AutoMigrate`，向后兼容的新增列自动补齐；破坏性变更见
[升级与迁移](/operations/upgrades/)。升级前先做一次备份：

```bash
docker compose -f deploy/docker-compose.yml exec postgres \
  pg_dump -U feedchamber feedchamber > backup-$(date +%F).sql
```

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| `port is already allocated` | 80 被占用 | `HTTP_PORT=8080` 重跑 |
| backend 反复重启 | Postgres/NATS 未 healthy | 等 30 秒；仍失败看 `logs backend` |
| 登录后立刻掉线 | 用 HTTP 访问但 `MASTER_COOKIE_SECURE=true` | 本地调试设 `false`，生产保持 `true` 并上 TLS |
| 图片 404 | 上传卷没挂上 / 被清空 | 检查 `feedchamber_uploads`，别 `down -v` |
| `/mcp` 404 | nginx 有精确 location，但后端没开 `mcp_enabled` | 后台设置开启后再试 |

更多见 [排障](/operations/troubleshooting/)。
