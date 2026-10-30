---
title: 快速开始
description: 用 Docker Compose 在五分钟内跑起一套 Feed Chamber
sidebar:
  order: 1
---

这一页的目标：**五分钟后你能登录后台并发出一篇帖子**。

## 前置条件

| 需要 | 版本 | 说明 |
|------|------|------|
| Docker + Compose | 任意近期版本 | 一键档只需要这个 |
| 或：Go / Node / pnpm | Go 1.25+ / Node 22+ / pnpm 10+ | 本地开发路径需要 |

## 一键启动（推荐）

在**仓库根目录**执行：

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

起来后打开 <http://localhost>。

编排档自带两个依赖服务，不需要你另外装：

| 组成 | 说明 |
|------|------|
| PostgreSQL 16 | 数据在 named volume `feedchamber_pg` |
| NATS 2.10（`-js`） | 启用 JetStream，数据在 named volume `feedchamber_nats` |
| 上传目录 | named volume `feedchamber_uploads` |
| 入口 | nginx：`/` → 前端 SSR，`/api/v1/` → 后端，`/mcp` → 后端 |

:::caution[NATS 是硬依赖]
后端**必须**连到一个外部 NATS（JetStream 已启用）。`NATS_URL` 为空时启动直接失败（fail fast），
不会静默降级成"只有本进程可见"的内存存储。Compose 档已经替你配好了。
:::

## 拿到管理员账号

首次启动会 seed 一个管理员：

- 用户名：`admin`
- 密码：取环境变量 `ADMIN_PASSWORD`；**没设就会随机生成**，并打印在后端日志里

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep "管理员密码"
```

输出形如：

```
WARN ADMIN_PASSWORD 未设置，已生成随机管理员密码: 1f3a...
```

:::tip[生产环境请显式设置]
`ADMIN_PASSWORD=你的密码 docker compose ... up -d` 更可控。
另外：**首次登录会强制修改密码**，这是设计行为，不是故障。
:::

## 登录后台

1. 打开 <http://localhost/login>
2. 用 `admin` + 上面的密码登录
3. 被引导到 `/admin/first-change-password`，设置新密码
4. 进入 `/admin` 运营概览

后台和前台是**同一个站点**（`/admin` 段），不是独立应用。

## 发第一篇帖子

1. `/admin/settings?tab=site` 改站点名、Logo、SEO 标题
2. `/admin/spaces` 建一个版面（或用 seed 出来的默认版面）
3. 回前台 `/timeline`，点发帖，写正文，提交
4. 默认审核模式是 `async`：先公开，后台异步跑规则，命中才转人工审核

## 本地开发（不用 Docker）

两个终端，各自起一半：

```bash
# 终端 A：后端
cd app/server && go run main.go -e dev

# 终端 B：前端（含 /admin）
pnpm dev:web
```

本地开发仍需一个 NATS：

```bash
nats-server -js
# 然后后端带 NATS_URL 启动：
cd app/server && NATS_URL=nats://127.0.0.1:4222 go run main.go -e dev
```

前端默认连 `http://localhost:8080` 的后端。

## 文档站本身

```bash
pnpm dev:docs      # http://localhost:4321
pnpm build:docs    # 静态产物在 app/docs/dist
```

## 下一步

- 想知道每个模块能干什么 → [功能总览](/capabilities/)
- 想接 API → [API 约定](/api/conventions/)
- 想给 AI 客户端用 → [MCP 概览](/mcp/overview/)
- 起不来 → [常见故障](/faq/)
