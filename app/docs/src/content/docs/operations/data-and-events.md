---
title: 数据与事件
description: 主要数据表怎么分组、NATS KV 存了什么、事件怎么流转、通知与缓存怎么失效
---

排障时最常见的两种误判：以为数据只存在数据库里，以为通知是同步发的。
实际上数据库之外还有一层 NATS KV（会话、限流、缓存、OAuth state），而通知是
事件驱动、异步落库的。这页把两边的边界说清楚。

## 数据表分组

`AutoMigrate` 启动时注册 69 个模型，按用途可以分五组：

| 分组 | 主要表 |
|------|--------|
| 用户与身份 | `users`、`credentials`、`user_sessions`、`user_mfa`、`api_keys`、`oauth_accounts`、`roles`、`role_permissions`、`groups`、`group_members`、`group_permissions`、`invites`、`banned`、`user_follows`、`user_blocks`、`login_histories`、`security_events` |
| 内容 | `posts`、`contents`、`spaces`、`tags`、`post_tags`、`drafts`、`revisions`、`uploads`、`announcements`、`topic_read_states`、`badges`、`user_badges` |
| 互动 | `content_likes`、`post_favorites`、`watches`、`notifications`、`conversations`、`conversation_members`、`messages`、`check_ins` |
| 治理 | `reports`、`content_moderations`、`moderation_rules`、`moderation_configs`、`motions`、`motion_votes`、`audit_logs` |
| 配置与扩展 | `settings`、`meta`、`permissions`、`space_role_permissions`、`xp_rules`、`xp_streak_bonuses`、`trust_levels`、`xp_adjustments`、`gate_rules`、`gate_domain_defaults`、`webhooks`、`webhook_deliveries`、`backups`、`plugins`、`import_maps`、`mcp_servers` |

两条数据边界：

- **`AutoMigrate` 只加不删**。新增列、新增索引自动补齐；删列、改类型、数据搬迁
  不会自动发生，要写在 `app/server/db/migrations/*.sql` 里。
- **那些 SQL 没有执行器**。它们靠人按需手动跑，各自开头写着「背景 / 是否可选 /
  幂等性 / 兼容性」，执行前先读。

`users.total_xp` / `trust_level` / `level_metrics` 只有一个写入点：
`TrustService.Recompute`。别在别处直接更新这三列，下一次重算会覆盖。

## NATS KV 存了什么

KV Store 不是缓存装饰，去掉它站点会立刻不可用（会话、限流、设置全挂）。
`NATS_URL` 为空时进程直接退出，不会降级到本地内存。

| 键形状 | TTL | 用途 |
|--------|-----|------|
| `sess:<sid>` | 等于会话剩余寿命 | 会话 read-through：`user_sessions` 是权威，KV 只是免查库 |
| `rl:<ip>:<policy>` | 窗口长度（通常 1 分钟） | 命名端点的每 IP 预算，如 `rl:1.2.3.4:login` |
| `rl:ratelimit:<ip>` | 窗口长度 | 网关的全局每 IP 上限（策略名 `global`） |
| `apikey_rl:<keyID>` | 窗口长度 | 单把 API Key 的预算 |
| `setting:<key>` | 60 秒（缺失标记 10 秒） | 设置读取缓存，写操作会就地删键 |
| `oauth:<state>` / `oauth_state:<state>` | 5 / 10 分钟 | 社交登录与网关 OAuth 的 CSRF state，一次性消费 |
| `ban:<userID>` / `ban:<userID>:<spaceID>` | 30 秒 | 全局/版面禁言判定缓存 |
| `vc:<postID>` | 直到落库 | 浏览量增量缓冲，后台每 60 秒刷进 SQL |
| `notif:unread:<userID>` | 5 分钟 | 未读计数，配合 `notif:<userID>` 频道推 SSE |
| `admin:stats` | 30 秒 | 后台概览聚合缓存 |
| `mfa:login:<token>` | 5 分钟 | 登录 MFA 挑战的临时票据 |
| `emailcode:<purpose>:<email>` | 设置项 `email_code_ttl`（默认 600 秒） | 邮箱验证码 |
| `perm:sync` / `trust:sync` / `gate:sync` | 无（Pub/Sub 频道） | 多实例缓存失效广播 |

业务 key 在写入 JetStream 前会被十六进制编码（JetStream 只接受字母数字与 `-` `_` `.`），
per-key TTL 由配套的 `<bucket>_ttl` bucket 实现。看 NATS 时看到的是编码后的 key，
不要按明文去找。

## 事件类型

事件总线基于 NATS core pub/sub。**同一个事件类型用一个队列订阅**，所以集群里
每个事件只被一个实例消费，那个实例再分发给本进程内的全部订阅者。内存缓冲 100 条、
4 个 worker；缓冲满时事件被丢弃并计入 `feedchamber_eventbus_dropped_total`——
丢弃是永久的，对应的通知或积分缺口不会自己补上。

| 事件 | 发布者 | 消费者 | 影响 |
|------|--------|--------|------|
| `post.created` | 发帖 | webhook、异步审核 | `moderation_mode=async` 时决定是否转入工队列 |
| `post.closed` | 关闭/重开帖子 | webhook | 外部系统同步 |
| `content.created` | 发评论 | webhook | 外部系统同步 |
| `content.replied` | 新评论或回复 | 通知、webhook | 收件人收到一条 `comment` 通知 |
| `content.liked` | 点赞评论 | 通知、webhook | 一条 `like` 通知 |
| `content.watched` | 关注的话题有新内容 | 通知、webhook | 一条 `watch` 通知 |
| `user.created` | 注册 | webhook | 注册当下同时发 `user.registered` 别名 |
| `user.followed` | 关注用户 | 通知、webhook | 一条 `follow` 通知 |
| `user.mentioned` | 正文里 @人 | 通知、webhook | 一条 `mention` 通知 |
| `report.created` | 举报 | webhook | 外部工单系统 |
| `moderation.decided` | 审核通过/拒绝 | webhook | `decision` 为 `approved` 或 `rejected` |
| `motion.created` / `motion.resolved` | 动议公示 / 结案 | 通知、webhook | 一条 `motion` 通知，复用同一 SSE 通道 |

事件的 trace 上下文写在 NATS 消息头的 `traceparent` 里，消费侧据此重建同一个 trace，
所以一条通知的日志能和触发它的 HTTP 请求对上。

## 通知扇出去重

一条评论可能同时命中三条扇出路径：**帖子（或父评论）作者 → @提及 → 关注该话题**。
三个步骤共享同一个 `notified` 集合，按上面的优先级先到先得，因此**一条评论只产生
一条通知**。

改动扇出逻辑时注意：新增步骤必须接收并遵守这个集合，否则同一个人会重复收到通知。
另外三件事值得记住：

- 自己评论自己的帖子不发通知。
- 通知在事件消费侧落库，发帖请求不会因为通知失败而回滚。
- 未读数走 KV 自增，读不到时回落到 `COUNT(*)`，所以计数偏差别急着怀疑数据库。

## 缓存失效广播

多实例部署下，配置写在一个实例、生效在别的实例上，靠三个 Pub/Sub 频道：

| 频道 | 谁在改 | 失效什么 |
|------|--------|----------|
| `perm:sync` | 角色/版面/用户组权限 | 权限缓存（角色 → 权限码、版面层级） |
| `trust:sync` | XP 规则、等级定义 | XP 配置快照 |
| `gate:sync` | 门控规则与域默认 | 门控规则快照 |

**设置没有广播频道**。`setting:<key>` 是 60 秒 TTL 的读缓存，写入方会就地删掉本实例
的键，其它实例最迟 60 秒后看到新值。所以「刚改完设置、另一个实例还是旧行为，
一分钟后就对了」是设计内行为，不是 bug。要立刻全站生效，就重启后端。

## SSE 心跳

两条长连接流的保持方式不同，都不能省：

- `POST /api/v1/notifications/stream`：每 15 秒写一行 SSE 注释 `: keepalive`。
  空闲流不产生任何字节，任何带读超时的中间层都会切断它（nginx 的
  `proxy_read_timeout` 默认 60 秒，实测 61 秒断开）。前端解析器只认 `data: ` 行，
  注释行不会被误当成事件。
- `POST /api/v1/realtime/stream`：每 15 秒发一个 `heartbeat` 事件。

`deploy/nginx/nginx.conf` 里另有 SSE 专用 location（关闭 buffering、读超时放到
3600 秒）作为兜底。缺了它，推送会变成批量延迟，或者干脆连不上。

## 验证

```bash
# 事件与通知是否在跑（族存在即接线成功）
curl -s localhost:9091/metrics | grep feedchamber_eventbus_publish_total

# KV 是否可用（health 会真的写一次、读一次、删一次）
curl -s localhost:8080/api/v1/health | grep kvstore
```

- 通知收不到：先看 `feedchamber_eventbus_dropped_total` 是否在涨，再看
  `feedchamber_sse_write_failures_total`。
- 改了配置不生效：确认改的是哪个实例，等 60 秒；仍不生效再查频道订阅是否启动成功
  （启动日志里有订阅失败的 WARN）。
