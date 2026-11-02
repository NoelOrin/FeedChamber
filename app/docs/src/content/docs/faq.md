---
title: 常见问题
description: 启动、登录、上传、通知、API 与 MCP 的高频问题，按现象查
---

按"你看到的现象"组织。每条给原因与处理；更系统的排障路径见 [排障](/operations/troubleshooting/)。

## 启动

### 后端启动即退出，日志说 NATS

`NATS_URL` 是必填项，后端不自带 NATS：

```bash
nats-server -js
NATS_URL=nats://127.0.0.1:4222 go run main.go -e dev
```

Docker 部署时确认 `nats` 容器先 healthy；compose 已经写了 `depends_on`，自定义编排要自己加。

### `go run main.go dev` 报 unknown command

环境只经 `-e` 选择，没有 `dev` / `prod` 子命令：

```bash
go run main.go -e dev      # ✅
go run main.go dev         # ❌ unknown command
```

### 端口冲突

后端默认 `8080`，前端 `3000`，文档站 `4321`，Compose 的 nginx `80`。
换端口用 `PORT=8081` / `HTTP_PORT=8080`，前端端口看 vinext 启动输出。

### `unsupported DB_TYPE` 或 `DB_DSN is required`

`DB_TYPE` 只接受 `sqlite`、`mysql`、`postgres`；后两者必须给完整 `DB_DSN`。
SQLite 不需要 DSN，留空即落在 `app/server/db/app.db`。

## 账号

### 管理员密码是什么

首次启动 seed 用户名 `admin`，密码取 `ADMIN_PASSWORD`；没设就随机生成并打印：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i admin
```

### 登录后立刻掉线 / cookie 不生效

- 纯 HTTP 访问但 `MASTER_COOKIE_SECURE=true`：本地调试设 `false`，生产保持 `true` 并上 TLS；
- 用 `127.0.0.1` 与 `localhost` 混用：cookie 域不同，统一用一个；
- 反代没传 `X-Forwarded-Proto`：后端认为不是 HTTPS。

### 首次登录被强制改密

设计行为。seed 密码只用于第一次登录，改密后旧会话全部失效。

### 忘记管理员密码

两条路，按"还有没有另一个能登录的管理员"选：

**还有另一个管理员**：用它的会话调重置接口（需要 `user:update` 权限）：

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/reset_password \
  -H "Authorization: Bearer $SID" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","new_password":"新的强密码"}'
```

成功后目标用户的全部会话立即失效。

**一个管理员都进不去**：直接改库写入新的 bcrypt 哈希，或先备份再重建管理员账号。
改库前先停服并备份：

```bash
go run main.go -e prod backup --out /backup/before-reset
```

:::caution[不要删 credentials 行来"重置"]
删掉凭据只会让账号无法登录，不会触发任何重置流程；忘记密码的邮箱流程需要目标
邮箱可达，且管理员账号未必绑定了邮箱。请改哈希或走上面的重置接口。
:::

## 内容与审核

### 发帖成功但列表里看不到

默认审核模式是 `async`：帖子先公开、异步审核。若后台把 `moderation_mode` 改成了
`manual`，所有新帖都是 `pending`，只有审核通过才出现在公开列表。

### 帖子在详情页 403

版面可见性是 `login` 或 `level`，而当前用户等级不够。详情页会提示所需等级；
作者本人与有 `report:manage` 的审核者不受限。

### XP 不涨 / 等级掉了

XP 是行为聚合而不是累加：删帖、取消点赞、取消收藏、撤销精华、审核拒绝都会回收。
改了 XP 规则后想让存量立即对齐，跑一次全量重算：

```bash
go run main.go -e prod recompute-xp
```

### 通知重复或收不到

一条评论最多一条通知（作者 > @提及 > 关注话题，按优先级去重）。收不到时先看
`/api/v1/notifications/stream` 是否被反代切断——空闲流需要心跳与更长读超时，
见 [实时事件](/api/realtime/)。

## 上传与图片

### 上传 413

反代的 `client_max_body_size` 小于上传上限。后端默认 `upload_max_size=10485760`（10 MiB），
nginx 默认只有 1 MiB：

```nginx
client_max_body_size 10m;
```

### 图片 404

本地存储时，容器重建若没挂 `feedchamber_uploads` 卷，文件就没了；S3 模式下检查
`storage_provider` 是否为 `s3` 且凭证正确。切存储需要重启。

## API

### 为什么写操作不能用 PUT / PATCH / DELETE

项目约定是 RPC 风格：写操作一律 `POST`，路径带动作后缀（`/create`、`/{id}/update`、
`/{id}/delete`）。PUT/DELETE 在业务接口上一律不存在，客户端不要按 REST 直觉拼路径。

### 401 与 403 怎么区分

- `1001/1002/1003/1014`：没有凭证、凭证错误、过期、已撤销 → 重新登录或刷新；
- `1013` / `403`：已登录但权限不足；
- 被封禁的账号请求受保护资源会得到 **404**（故意不暴露账号存在）。

### 分页参数叫什么

固定 `page` 与 `page_size`（**下划线**）。`page_size` 小于 1 或大于 `page_max_size`
（默认 100）会回落到默认 20，不是夹到上限；写 `pageSize` 不会被识别。

## MCP

### `/mcp` 返回未启用

需要同时满足：`mcp_enabled=true`、`api_key_enabled=true`，请求带有效 API Key。
两个开关默认都是关闭的。

### AI 能读但不能写

写工具默认关闭，需要在后台 MCP 页面逐个启用；`mcp_disabled_tools` 里以 `-tool`
形式显式停用的工具也会被拦住。

### 工具报错和协议报错怎么分

工具执行失败用结果里的 `isError: true`；JSON-RPC 层的 `error` 只表示协议问题
（方法不存在、参数不合法）。客户端应按这两层分别处理。
