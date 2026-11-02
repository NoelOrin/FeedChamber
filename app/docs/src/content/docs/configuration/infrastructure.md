---
title: 数据库、NATS 与存储
description: 数据库选型与迁移、外部 NATS 的必填原因与 KV 用途、本地或 S3 存储与邮件提供商
---

这一页讲三件「依赖级」的配置：数据库、NATS 与文件存储，外加同样属于基础设施的邮件
发送。它们的共同点是**配置在启动时被读取**，配错要么起不来，要么必须重启才生效。
判断标准很简单：**改站点行为去后台，改依赖重启进程。**

## 前置条件

- 有部署主机的 shell 权限（这些配置不在后台表单里）。
- 数据库与 NATS 已经可达：先验证连通性，再动应用配置。
- 环境变量清单见 [环境变量](/reference/environment/)；本地起步见 [本地开发](/install/local/)。

## 数据库

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_TYPE` | `sqlite` | `sqlite` / `mysql` / `postgres` |
| `DB_DSN` | 空 | `sqlite` 是文件路径；`mysql` / `postgres` 必填 |
| `DB_MAX_OPEN_CONNS` | `25` | 连接池上限 |
| `DB_MAX_IDLE_CONNS` | `10` | 空闲连接上限 |
| `DB_CONN_MAX_LIFETIME` | `300` | 连接最长存活（秒） |
| `DB_CONN_MAX_IDLE_TIME` | `60` | 空闲回收（秒） |

`sqlite` 适合单机与小规模部署，`DB_DSN` 留空时落在后端工作目录下的 `db/app.db`。
多实例部署必须用 `postgres` 或 `mysql`：`sqlite` 文件的并发写会互相锁住。

迁移是两套并行的机制：

1. **启动时自动建表**（GORM AutoMigrate），补齐新增的列与索引，可重复执行。
2. **手写 SQL** 放在 `app/server/db/migrations/`，**不会自动执行**——需要按脚本里的注释
   手工运行。它们处理 AutoMigrate 表达不了的数据搬迁与清理。

升级前先看 [升级](/operations/upgrades/) 一节确认是否有必须手工执行的脚本。

## NATS：为什么必填

`NATS_URL` 为空时进程**直接退出**，不是降级运行。原因是 NATS 同时承担两件不能缺的职能：

- **KV 存储**：会话缓存、限流计数、设置缓存、OAuth state、验证码等短期状态都在里面。
  没有它，限流与登录会退化成不可用或跨实例不一致。
- **事件总线**：发帖、评论、点赞等事件走 NATS 分发给通知、审核与 webhook 消费方。

早期版本会启动一个进程内 NATS，看起来「零依赖」，实际代价是重启即丢队列、多实例无法共享
会话缓存与限流窗口。所以现在必须是外部 NATS，并且**启用 JetStream**：

```bash
# 本地起一个（-js 就是启用 JetStream）
nats-server -js
# 或容器
docker run --rm -p 4222:4222 -p 8222:8222 nats:2.10-alpine -js -m 8222
```

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NATS_URL` | 空 | **必填**，如 `nats://127.0.0.1:4222` |
| `NATS_BUCKET` | `feedchamber_kv` | JetStream KV bucket 名；多环境共用一个 NATS 时必须区分 |

KV bucket 里存什么（按用途看，不按实现看）：

| 数据 | 典型键前缀 | 丢了会怎样 |
|------|-----------|-----------|
| 会话读透缓存 | `sess:` | 退化成每次请求查库，功能不受影响 |
| 限流窗口 | `rl:` | 窗口清零，短时限额被放宽 |
| 设置与禁言缓存 | `setting:`、禁言相关键 | 退化成查库，重启即可恢复 |
| OAuth state | `oauth:`、`oauth_state:` | 登录中的授权回调会失败，重试即可 |
| 邮箱验证码 / MFA 挑战 | `emailcode:`、`mfa:login:` | 当时的验证码与挑战失效 |
| 浏览量缓冲 | `vc:` | 缓冲未刷入的浏览数会丢（默认每 60 秒刷一次） |

:::caution[不要用共享 bucket 跑多套环境]
同一个 `NATS_BUCKET` 被两套站点共用会互相覆盖设置缓存与会话缓存。预发布环境务必换名。
:::

事件总线的语义是「每个事件在集群内只消费一次」：消费方用队列订阅，所以多实例部署时
通知不会重复发。事件本身是 NATS core pub/sub，不走 JetStream 持久化——总线短暂中断
期间的事件不补发，这是有意的取舍：通知与 webhook 丢失一次可以接受，把积压写进磁盘
反而会掩盖问题。

## 存储

存储是**启动期单例**：进程启动时读一次配置，之后固定不变。所以后台改完存储设置必须重启。

| 键（设置表） | 环境变量回退 | 默认值 |
|--------------|--------------|--------|
| `storage_provider` | `STORAGE_PROVIDER` | `local` |
| `storage_local_dir` | `STORAGE_LOCAL_DIR` | `uploads` |
| `s3_endpoint` | `S3_ENDPOINT` | 空 |
| `s3_region` | `S3_REGION` | `us-east-1` |
| `s3_bucket` | `S3_BUCKET` | 空 |
| `s3_access_key_id` | `S3_ACCESS_KEY_ID` | 空 |
| `s3_access_key_secret` | `S3_ACCESS_KEY_SECRET` | 空 |
| `s3_use_ssl` | `S3_USE_SSL` | `true` |
| `s3_custom_domain` | `S3_CUSTOM_DOMAIN` | 空 |

读取优先级是 **settings 表 → 环境变量 → 默认值**：表里有值就用表里的，空值才回退。
后台入口 `/admin/settings?tab=storage`；`s3_custom_domain` 用于 CDN 或自定义域名回源。

切到 S3 的四步：

1. 在 `/admin/settings?tab=storage` 填 endpoint、region、bucket、密钥，provider 选 `s3`。
2. 保存后**重启后端**。
3. 上传一张图片，确认返回的文件地址指向 S3 或你的自定义域名。
4. 旧的本地上传记录不会自动搬过去；需要保留旧文件就把 `uploads/` 目录一起带上。

## 邮件

同样在后台配置，`/admin/settings?tab=user` →「邮件服务」。总开关是 `email_enabled`，
关闭时取验证码会直接报「邮件功能未启用」。

| 键 | 默认值 | 说明 |
|----|--------|------|
| `email_provider` | `log` | `log` / `smtp` / `sendgrid` / `resend` |
| `email_from` | 空 | 发件人邮箱，多数提供商要求与账号域名一致 |
| `email_from_name` | `Feed Chamber` | 发件人显示名 |
| `email_smtp_host` / `email_smtp_port` | 空 / `587` | SMTP 服务器与端口 |
| `email_username` / `email_password` | 空 | SMTP 认证信息 |
| `email_use_tls` | `false` | `true` 表示隐式 TLS（465）；`false` 走 STARTTLS（587） |
| `email_api_key` | 空 | SendGrid / Resend 的 API 密钥 |

`log` 提供商不发信，只把邮件写进服务端日志——本地调试与内网部署最省事，也是排查
「SMTP 到底发出去没有」的对照基线。`resend` 与 `sendgrid` 缺 `email_api_key` 时会直接
报配置错误，而不是静默失败。

发信限制不在邮件配置里，而在验证码设置里：`email_resend_cooldown`（默认 60 秒）、
`email_daily_limit`（默认 10 封/邮箱/天）、`email_code_ttl`（默认 600 秒），
细节见 [认证与安全](/configuration/authentication/)。

## 验证

```bash
# 数据库：能连通并读到设置
curl -s -X POST http://localhost:8080/api/v1/settings/public | jq '.code'

# NATS：启动日志应出现这一行
#   EventBus/KV Store: external NATS at nats://...
# 且没有 "初始化 KV Store 失败"

# 存储：申请一次预签名，看返回的 fileUrl 指向前缀是本地目录还是 S3
curl -s -X POST http://localhost:8080/api/v1/upload/presign \
  -H 'Authorization: Bearer <sid>' -H 'Content-Type: application/json' \
  -d '{"file_name":"t.png","content_type":"image/png"}' | jq '.data.fileUrl'
```

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 启动即退出，日志 `NATS_URL is required` | 没配 NATS 或变量名写错 | 先起 `nats-server -js`，再填 `NATS_URL` |
| 日志 `初始化 KV Store 失败` | NATS 没启用 JetStream | 用 `-js` 启动，或换支持 JetStream 的实例 |
| `failed to initialize database` | DSN 错、库不存在、账号无权限 | 先用 `psql` / `mysql` 手工连一次 |
| 多实例下线时 429 乱跳 | 各实例连了不同 NATS 或不同 bucket | 统一 `NATS_URL` 与 `NATS_BUCKET` |
| 改了 S3 配置，上传仍落本地 | 存储是启动期单例 | 重启后端 |
| 上传 403 / 报 S3 错误 | 密钥或 bucket 策略不对、`s3_use_ssl` 与 endpoint 不匹配 | 先用同一密钥跑一次 `aws s3 ls` 对照 |
| 验证码永远收不到 | provider 还是 `log` | 日志里能搜到验证码；要真发信就切 SMTP 或 API 提供商 |
