---
title: 常见问题
description: 起不来、找不到密码、改配置改不动
sidebar:
  order: 3
---

## 启动相关

### 后端启动就退出，日志说 NATS

`NATS_URL` 是**必填**。后端不会内嵌 nats-server，需要外部 NATS 且启用 JetStream：

```bash
nats-server -js
NATS_URL=nats://127.0.0.1:4222 go run main.go -e dev
```

### `go run main.go dev` 报 unknown command

环境只经 `-e` 选择，没有 `dev` / `prod` 子命令：

```bash
go run main.go -e dev      # ✅
go run main.go dev         # ❌ unknown command
```

### 端口冲突

后端默认 `8080`，前端开发默认自己一套端口，文档站是 `4321`。
改后端端口用 `PORT=xxxx`。

## 账号相关

### 管理员密码是什么

首次启动 seed 用户名 `admin`，密码取 `ADMIN_PASSWORD`；
没设就随机生成并打印在日志里：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep "管理员密码"
```

### 登录后被要求改密码

首次登录强制改密是设计行为（R-05），不是故障。

### 忘记管理员密码

直接改库最省事：清掉 `credentials` 表里该用户的 `password_hash`，
或重置整个数据库重新 seed（会丢数据）。

## 配置相关

### 后台改了 S3 配置没生效

存储 provider 是**启动期单例**，改完要重启后端。

### 改了站点名，浏览器标题没变

检查 `/admin/settings?tab=site` 的 `seo_title`——标题模板用的是它，
不是 `site_name`。

### 前台看不到我配的导航 / 首页模块

可能被可见性过滤了：`visibility=level` 的项要求登录用户等级 ≥ `min_level`。
也可能配置未启用（后台列表里有「启用」开关）。

## 内容相关

### 发帖后帖子不在列表里

默认审核模式是 `async`：先公开、后台异步复查，命中规则才转 `pending`。
如果模式是 `manual`，则一律先 `pending`。作者本人能看到自己的待审帖。

### 评论重复收到通知

这是 bug，不是配置。一条评论只应产生一条通知（按「作者 > 提及 > 关注」去重）。
如果复现了，检查是否有新增的扇出步骤没接同一个 `notified` 集合。

### AI 改了 JS 片段但没生效

机器凭证（API Key / MCP）写 JS / HTML 类内容会被**强制存草稿**，
需要人工在 `/admin/customize/snippets` 启用。这是刻意的闸门。

## MCP 相关

### `/mcp` 返回 404

检查 nginx 是否有 `location = /mcp`。少了它请求会落到前端。
如果是本地开发，确认后端在跑且路径是 `POST`。

### MCP 能连上但工具列表是空的

三件事都要成立：
1. `mcp_enabled` = `true`
2. `api_key_enabled` = `true`
3. Key 的 scope 覆盖你要用的工具权限码

### 写工具不见了

写工具**默认全部关闭**，需要在 `/admin/mcp` 逐项开启。

## 文档站相关

### 文档站构建后没有内容

确认 `app/docs/src/content/docs/` 下有 `.md` / `.mdx` 文件。
collection 名必须是 `docs`，否则 Starlight 找不到。
