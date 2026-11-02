---
title: 排障
description: 按现象排查：启动、NATS、数据库、登录、权限、上传、SSE、邮件、OAuth、审核、XP、指标与备份
---

按现象查，不按模块查。每条都是「现象 → 原因 → 处理」，命令可直接粘贴；
Docker 部署时给命令加 `docker compose -f deploy/docker-compose.yml exec backend` 前缀。

## 启动与依赖

### 启动即退出
**现象**：进程起来就结束，最后一行是 `invalid config: …` 或 `failed to initialize database`。\
**原因**：启动期配置错误一律 `Fatal`、不降级。常见是 `NATS_URL is required…`、`DB_TYPE is required`、`unsupported DB_TYPE "…"`、`DB_DSN is required when DB_TYPE=…`、`METRICS_PORT must be between 1 and 65535…`。\
**处理**：按报错补 `.env` 或 `.env.<env>`（后者覆盖前者）。若是「端口 8080 已被占用」，那是监听前探测，换端口或释放占用。

### 连不上 NATS
**现象**：`初始化 KV Store 失败: nats connect: …`，或 `nats kv bucket "feedchamber_kv": …`。\
**原因**：前者是地址或网络不通；后者通常是服务端没开 JetStream（`-js`）。\
**处理**：确认服务端带 `-js` 启动、4222 可达。运行期断连**不会**让进程退出（无限重连），只能从指标看出：

```bash
curl -s localhost:9091/metrics | grep feedchamber_nats_connected   # 0 = 不可用
```

### 数据库锁或连接耗尽
**现象**：SQLite 报 `database is locked`；Postgres 大量请求超时，`/health` 正常但变慢。\
**原因**：SQLite 只允许一个写者，长事务或并发写会撞锁（默认 DSN 的 `busy_timeout` 是 5000ms）；连接池被慢查询占满。\
**处理**：先看池子与慢查询占比，再决定调池还是查 SQL。`DB_MAX_OPEN_CONNS` 默认 25，调大只是把排队挪进数据库；SQLite 部署请避免多实例。

```bash
curl -s localhost:8080/api/v1/health | grep db_pool
curl -s localhost:9091/metrics | grep -E 'feedchamber_db_pool_in_use|feedchamber_db_slow'
```

## 登录与权限

### 找不到管理员密码
**现象**：后台进不去，也不知道初始密码。\
**原因**：`ADMIN_PASSWORD` 只在**首次** seed 时使用——`admin` 用户已存在就整段跳过；留空则随机生成并打一条 WARN。\
**处理**：先从日志捞，捞不到就让另一个有 `user:update` 的账号重置：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i '管理员密码'
curl -s -X POST localhost:8080/api/v1/auth/reset_password \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $SID" \
  -d '{"username":"admin","new_password":"新密码"}'
```

一个人都进不去时只能改库：`credentials` 表按 `user_id` 存 bcrypt 哈希，先备份再写入新哈希。

### 登录返回 401
**现象**：`/auth/login` 返回 1001 / 1010 / 1011 / 1015；或登录成功后紧接着又 401。\
**原因**：`1001` 凭证不存在、`1010` 密码错、`1011` 用户不存在、`1015` 被封禁。登录成功仍 401 通常是 cookie 带了 `Secure` 而你在 HTTP 下访问，浏览器没存。\
**处理**：按 `code` 看 `msg`；cookie 问题确认对外是 HTTPS，本地调试可临时把 `MASTER_COOKIE_SECURE` 设为 `false`。改密、改角色、封号都会删会话行，旧 sid 立刻失效。

### 权限 403 / 404
**现象**：接口返回 `1013 FORBIDDEN`，或本该 403 的位置返回 404。\
**原因**：`1013` 是权限码不满足；**被封禁的写操作刻意返回 404**（`USER_BANNED`），让探测者分不清「无权」与「不存在」。API Key 的 scope 是上限，scope 不含该码时角色再高也过不去。\
**处理**：核对角色权限矩阵与凭证类型（会话 / `X-API-Key` / `Bearer ak_…`），后台入口是「角色 → 权限矩阵」与「用户 → 禁言/封禁」。

## 内容与文件

### 上传失败
**现象**：413，或 `文件大小超过 10MB 限制`，或 `不允许的文件类型`。\
**原因**：两层限制——请求体上限（`/api/v1/upload` 10MB，其余 1MB）与业务设置（`upload_max_size` 默认 10MB、`upload_allowed_types` 默认四种图片类型，MIME 由内容嗅探决定，不看扩展名）。\
**处理**：4002 是 `BODY_TOO_LARGE`，属第一层；业务错误是 2001 并带具体文案，去后台设置调 `upload_max_size` / `upload_allowed_types`。

### 图片 404
**现象**：帖子配图裂开，`/api/v1/upload/file/<key>` 返回 404。\
**原因**：上传卷没挂上、容器重建丢了目录，或 `uploads` 表有行而磁盘没文件。\
**处理**：进程工作目录固定是 `app/server`，本地存储相对路径即 `app/server/uploads`；`down -v` 会连卷一起删，这是最常见的人为原因。

```bash
docker compose -f deploy/docker-compose.yml exec backend ls -l uploads | head
```

### SSE 断流
**现象**：通知或实时推送约 60 秒断一次，前端不停重连。\
**原因**：空闲的 SSE 流不产生字节，任何带读超时的中间层都会切断它（nginx 默认 60s）；反代开了 buffering 则把推送攒成批。\
**处理**：确认 nginx 有 SSE 专用 location（关闭 buffering、读超时 3600s），且处理程序仍在发心跳（通知流 15 秒一行 `: keepalive`）。连接数下降是正常关页，**写失败数**才是被截断。

```bash
curl -s localhost:9091/metrics | grep feedchamber_sse_write_failures
```

### 邮件发不出去
**现象**：注册、找回密码提示成功，但收不到信。\
**原因**：`email_provider` 默认是 `log`——只写日志不发送。配了 SMTP 仍失败要看主机/端口/TLS 模式（465 隐式 TLS，587 STARTTLS），以及频率限制（`email_daily_limit` 默认 10、`email_resend_cooldown` 默认 60 秒）。\
**处理**：先确认 provider 不是 `log`，再抓发信日志：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -iE 'mail|smtp|verification'
```

### OAuth 回调失败
**现象**：`invalid or expired state`，或授权后回到登录页。\
**原因**：state 存在 KV 里且**一次性消费**（社交登录 5 分钟、网关 OAuth 10 分钟），过期或重复回调都会失败；回调地址必须与后台登记完全一致（含协议、端口、路径）。\
**处理**：核对 client id / secret / callback，确认 `site_url` 是外部可访问地址，然后从头走一次完整流程，别在旧标签页点第二次「授权」。

## 运营与观测

### 审核卡住
**现象**：帖子看不到了，审核队列里也没有；或者内容根本没公开。\
**原因**：发帖初始状态由 `moderation_mode` 决定：`off` 直接公开；`async`（默认）先公开再异步判定，命中转 `pending`；`external` 先调外部服务；`manual` 全部 `pending`。`pending` 不进公开列表，只进审核队列。\
**处理**：在后台「内容审核」里直接通过或拒绝；改模式只影响新帖，历史待审不会自动清空。

### XP 不涨
**现象**：发了帖、签了到，`total_xp` 还是旧值。\
**原因**：XP 是行为聚合派生的，只有 `TrustService.Recompute` 写快照。改完规则只让**下一次**重算用新分值；每日全量重算在启动 2 分钟后跑第一次，之后每 24 小时一次。手工设过等级的用户（`level_manual`）不会被重算打回。\
**处理**：手动对账，再到用户详情「成长」页看 XP 分解。

```bash
go run main.go -e prod recompute-xp      # recomputed 1234 users
```

### 指标端口抓不到
**现象**：`curl localhost:9091/metrics` 拒绝连接，站点却正常。\
**原因**：`METRICS_ENABLED=false`；端口被占（listener 起不来只打 `metrics listener on :9091 unavailable` 的 WARN 就继续）；或你在宿主机打容器内端口——compose 用 `expose`，没有 `ports` 映射。\
**处理**：在容器网络内抓；不要把 9091 加到 nginx 上游（它没有鉴权）。

```bash
docker compose -f deploy/docker-compose.yml exec backend \
  wget -qO- http://127.0.0.1:9091/metrics | head
```

### 备份失败
**现象**：`backup --out` 报错，或产物里只有 `db-note.txt`。\
**原因**：`backup` 是本地站点备份——只复制 SQLite 文件、生成上传清单，不为 MySQL/PostgreSQL 做 dump，也不打包上传文件本身。\
**处理**：非 SQLite 用引擎原生工具，上传目录单独备走；恢复前先在副本上演练，并确认版本与备份同代。

```bash
go run main.go -e prod backup --out /backup/2026-10-30
docker compose -f deploy/docker-compose.yml exec postgres \
  pg_dump -U feedchamber feedchamber > backup-2026-10-30.sql
```

## 还没解决

带三样东西再问人：`trace_id`（日志里同一个 id 能串起整条请求）、`/api/v1/health` 的完整输出、以及 `docker compose ps` 的健康状态。
