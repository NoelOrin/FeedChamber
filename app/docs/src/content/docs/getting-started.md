---
title: 快速开始
description: 用 Docker Compose 在五分钟内跑起 Feed Chamber，登录后台并发第一篇帖子
---

这页的目标只有一个：**五分钟后你能登录后台，并在前台发出一条帖子**。

不需要读完架构，也不用先理解 XP 或门控——先把站点跑起来，剩下的按导航继续。

## 你需要什么

| 方式 | 前置条件 |
|------|----------|
| Docker Compose（推荐） | Docker Engine + Compose v2 |
| 本地开发 | Go 1.25+、Node.js 22+、pnpm 10+，以及一个启用了 JetStream 的 NATS |

两条路都能跑通全部功能。已有 Kubernetes 或自己的反代，看 [选择部署方式](/install/choose-path/)。

## 五分钟跑起来

```bash
git clone https://github.com/NoelOrin/FeedChamber.git
cd FeedChamber
docker compose -f deploy/docker-compose.yml up -d --build
```

首次构建会拉取镜像并编译前后端，通常 3–8 分钟；之后启动只需几秒。

编排里有五个服务：Postgres、NATS、后端、前端、nginx。数据落在三个 named volume
（`feedchamber_pg`、`feedchamber_nats`、`feedchamber_uploads`），重建容器不丢数据。

## 拿到管理员账号

首次启动会 seed 一个 `admin`：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i "admin"
```

- 如果你在 compose 里显式设了 `ADMIN_PASSWORD`，密码就是它；
- 没设则随机生成，日志里会有 `WARN` 行打印明文密码。

## 登录并完成首次改密

1. 打开 <http://localhost/login>
2. 用 `admin` + 上面的密码登录
3. 会强制跳到 `/admin/first-change-password`，设置新密码
4. 进入 `/admin` 运营概览

首次改密是设计行为，不是故障——seed 出来的密码只用于这一次。

## 建版面、发第一篇帖子

1. 进 `/admin/spaces`，新建一个版面（或直接用 seed 出来的默认版面）
2. 回前台 `/timeline`，点右上角「发布主题」
3. 写标题与正文，提交

默认审核模式是 `async`：帖子先公开，后端异步跑规则，命中才转人工审核。
所以发完立刻能在时间线看到。想改成全部人工审核，去 `/admin/settings?tab=site` 把
`moderation_mode` 设为 `manual`。

## 验证装好了

| 检查 | 期望 |
|------|------|
| `curl -s localhost/api/v1/health` | `{"code":0,...}` 或 2xx |
| 打开 <http://localhost> | 时间线能加载，静态资源 200 |
| `curl -s localhost/sitemap.xml` | 返回 sitemap 索引 XML |
| `docker compose ... ps` | 五个服务都是 `healthy` / `running` |

后端日志默认是 JSON（`LOG_FORMAT=json`），出现 `level":"error"` 时优先看同一行的
`trace_id`，再用它串起整个请求。

## 起不来？

| 现象 | 处理 |
|------|------|
| `port 80 already in use` | `HTTP_PORT=8080 docker compose ... up -d` |
| 后端反复重启 | 看 `logs backend`；多半是 Postgres/NATS 还没 healthy，等 30 秒再看 |
| 登录提示密码错误 | 重新从日志取密码；或清空 volume 重来（会丢数据） |
| 页面能开但接口 502 | 后端没起来：`docker compose ... logs backend` |

更多见 [排障](/operations/troubleshooting/)。

## 下一步

- 想理解它为什么这样设计 → [架构总览](/operations/architecture/)
- 想改站点名、注册方式、审核策略 → [站点设置](/configuration/site/)、[内容管线](/configuration/content/)
- 想接 API 或给 AI 用 → [API 约定](/api/conventions/)、[MCP 集成](/api/mcp/)
