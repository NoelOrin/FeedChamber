---
title: Docker Compose 部署
description: 一条命令起一整套，含组成说明与常见调整
sidebar:
  order: 1
---

## 一键启动

在**仓库根目录**：

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

打开 <http://localhost>。

## 组成

| 服务 | 说明 | 数据位置 |
|------|------|----------|
| `postgres` | PostgreSQL 16 | named volume `feedchamber_pg` |
| `nats` | NATS 2.10 + JetStream | named volume `feedchamber_nats` |
| `backend` | Go 后端（端口 8080） | named volume `feedchamber_uploads` |
| `frontend` | 前端 SSR（vinext） | — |
| `nginx` | 入口：`/` 前端、`/api/v1/` 后端、`/mcp` 后端、爬虫面后端 | — |

数据库账号与库名可覆盖：`POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB`（默认都是 `feedchamber`）。

:::caution[端口暴露]
`METRICS_PORT`（9091）**只在容器网络内可达**，编排里用 `expose` 而不是 `ports`。
不要给它加宿主端口映射，也不要在 nginx 里加指向它的 location。
:::

## 显式设置管理员密码

```bash
ADMIN_PASSWORD='你的密码' docker compose -f deploy/docker-compose.yml up -d
```

不设会随机生成并打印在日志里：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep "管理员密码"
```

首次登录会强制改密（R-05 设计行为）。

## 改站点配置

绝大多数站点配置在**后台**里改，不用碰文件或环境变量：

| 想改什么 | 去哪 |
|----------|------|
| 站点名 / Logo / SEO | `/admin/settings?tab=site` |
| 存储（本地 → S3） | `/admin/settings?tab=storage`（改完需重启后端） |
| 邮件（SMTP / SendGrid / Resend） | `/admin/settings?tab=user` |
| 第三方登录 | `/admin/settings?tab=oauth` |
| 审核模式 | `/admin/settings?tab=content` 或 `/admin/moderation` |

:::note[存储是启动期单例]
存储 provider 在启动时按 settings 表构建。改 S3 配置后要重启后端容器才生效。
:::

## 升级

```bash
git pull
docker compose -f deploy/docker-compose.yml up -d --build
```

数据库结构用 AutoMigrate 在启动时自动推进；手写迁移脚本在 `app/server/db/migrations/`，
**没有自动执行器**，需要按需手动跑。

## 托管在平台（非本机）

如果由平台（Traefik 等）接管 80/443，直接把默认档的入口交给平台即可。
`docker-compose.dokploy.yml` 已随部署收敛删除——不要照旧文档去找它。

## 托管文档站

文档站是纯静态产物：

```bash
pnpm build:docs   # 产物在 app/docs/dist
```

丢给任意静态服务器即可（GitHub Pages / Cloudflare Pages / nginx）。
若要和论坛同域挂在 `/docs`，在 nginx 加一条静态 location 指向该目录。
