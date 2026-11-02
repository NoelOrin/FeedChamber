---
title: 系统与集成
description: 服务设置、API Key、OAuth 客户端、Webhook、备份、插件、MCP、审计日志与安全事件
---

「系统设置」是后台里唯一直接改写运行时行为的分组：设置写进 `settings` 表即时生效，
对外集成（API Key、OAuth、Webhook、MCP）则给机器发凭证。这些操作的影响面比日常内容管理大，
每一项都要求 `setting:manage` 或对应的管理权限码。

## 前置条件

- 已进入后台，且具备目标模块的权限码：设置 / OAuth 客户端 / MCP 要 `setting:manage`，
  Webhook 要 `role:manage`，备份要 `backup:manage`，插件要 `plugin:manage`。
- API Key 认证默认关闭（`api_key_enabled=false`），启用前任何 Key 请求都返回 403。

## 服务设置

`/admin/settings?tab=<分类>` 有六个分类：

| 分类 | 管什么 |
|------|--------|
| `site` | 站点名、描述、Logo、站点地址、SEO 与分享图、搜索引擎验证、维护模式 |
| `user` | 注册开关与注册模式、邀请、密码策略、邮件验证、MFA、会话策略 |
| `content` | 审核模式、分页尺寸、上传大小与类型、SSE 与浏览量刷入间隔 |
| `oauth` | 第三方登录（GitHub / Google / QQ / 微信 / 微博）与应用凭证 |
| `storage` | 本地存储或 S3 的参数 |
| `dashboard` | 你自己的后台账号：语言、账号信息、修改密码 |

保存时会把当前分类下的全部键一起提交（`POST /admin/settings/batch`），所以切分类不会丢掉未看的字段。
每次保存都写一条审计日志（`settings.batch_update`）。

:::caution
存储配置是启动期单例：改完 `storage` 分类需要重启进程才生效，不像其他设置那样即时可用。
:::

## API Key

`/admin/api-keys` 管理程序化访问凭证。

1. 「新建」→ `POST /api-keys`，填名称、`scopes`、`rate_limit`、`expires_at`。
2. 创建响应里的 `api_key`（形如 `ak_<64 位十六进制>`）**只出现这一次**，之后列表不再返回明文。
3. 调用时用 `X-API-Key: ak_...`，或 `Authorization: Bearer ak_...`。
4. 作废 → `POST /api-keys/{id}/revoke`。

关键实现细节：

- 库里只存 **bcrypt 哈希**（`api_keys.secret_hash`），拿到数据库也反推不出明文。
- `scopes` 支持粗粒度 `read` / `write` / `admin`，也接受精确权限码（如 `post:read,report:manage`）。
  创建时会把 scope 展开成权限码，逐个检查创建者是否真的拥有——**超出自身权限的申请直接 403**。
- 限流是每 Key 每分钟计数：Key 上填的 `rate_limit` 优先，填 0 则用设置 `api_key_default_rate_limit`
  （默认 100）。超限返回 429。
- `expires_at` 是 Unix 时间戳，过期后请求返回凭证过期错误；留空表示不过期。

调用时的权限判定是**交集**：API Key 的 scope 与所属用户的角色权限都为真才放行。
所以给 Key 加 `admin` scope 不会让它越过它所属账号本身的权限。

## OAuth 客户端

`/admin/oauth-clients` 管理把本站当作 OAuth 2.0 Provider 的第三方应用。

1. 新建 → `POST /oauth/clients`，填名称、`redirect_uris`（JSON 数组）、允许的 `scopes`。
2. 响应里的 `client_secret` 只返回这一次；库里存 bcrypt 哈希。
3. 密钥泄漏 → `POST /oauth/clients/{id}/reset-secret` 轮换；不再使用 → `POST /oauth/clients/{id}/delete`。

客户端拿到的 access token 是 opaque 字符串（不是 JWT），撤销即失效。OAuth 客户端没有「用户角色」，
判定只看 scope——这是它和 API Key 最大的区别。协议端点（`/oauth/authorize`、`/oauth/token`、`/oauth/revoke`）
返回的是 RFC 形状，不套统一信封。

## Webhook

`/admin/webhooks` 把站内事件推到你指定的地址。

1. 新建 → `POST /admin/webhooks/create`：`url`、`secret`、`events`、`enabled`、`tls_verify`。
2. 「Ping」→ `POST /admin/webhooks/{id}/ping`，发一条 `{"event":"ping"}` 验证连通性。
3. 「投递记录」→ `POST /admin/webhooks/{id}/deliveries/list`，看状态、尝试次数、响应码、耗时。
4. 失败的投递可手工重投 → `POST /admin/webhooks/{id}/deliveries/{deliveryId}/retry`。

可订阅的事件共 12 个：`post.created`、`post.closed`、`content.created`、`content.liked`、
`content.replied`、`content.watched`、`user.created`、`user.registered`、`user.followed`、
`user.mentioned`、`report.created`、`moderation.decided`。填别的事件名会被拒绝。

投递语义：

- 请求体是 `{"event": "...", "data": {...}, "ts": <unix>}`，`Content-Type: application/json`。
- 配了 `secret` 时带 `X-Orin-Signature: sha256=<HMAC-SHA256(secret, body)>`，接收方要用原始 body 验签。
- 非 2xx 视为失败，自动重试最多 5 次，退避 1 分钟 → 5 分钟 → 30 分钟 → 2 小时 → 12 小时；
  第 5 次仍失败则标记 `failed`，只能手工重投。
- `tls_verify=false` 会跳过证书校验，只建议对自签名的内网地址临时使用。

## 备份与恢复

`/admin/backups` 生成、下载与恢复快照。

1. 「创建备份」→ `POST /admin/backups/create`，可填名称（留空自动按时间戳命名）。
   产物是两部分：数据库快照 JSON，以及一份上传目录归档 `tar.gz`。
2. 列表 → `POST /admin/backups/list`；下载 → `POST /admin/backups/{id}/download`（以附件形式返回 JSON）。
3. 恢复 → `POST /admin/backups/{id}/restore`，**会用快照覆盖当前数据**，执行前先确认你选对了文件。
4. 清理 → `POST /admin/backups/cleanup`，按设置 `backup_retention_days`（默认 30 天）删除过期备份。

备份文件落在当前存储后端（本地目录或 S3），所以换存储后旧备份仍在原处，需要手工迁移。
不登录后台也可以从命令行导出本地备份（必须给输出目录）：
`cd app/server && go run main.go -e prod backup --out /path/to/backup-dir`。

## 插件

`/admin/plugins` 只有一份**声明式元数据**：名称、版本、作者、描述、主页、设置 JSON、启用状态。
安装（`/admin/plugins/install`）只是登记，启用（`/admin/plugins/{id}/status`）只是改标记，
**插件代码不会被执行**——当前版本没有插件运行时，也没有扩展点。把这里当成「已装组件清单」用。

## MCP

后台有两块：

- `/admin/mcp`：本站 MCP Server 的工具清单与启停。工具共 15 个（6 个读、9 个写，写工具默认关闭）。
  总开关 `mcp_enabled` 默认 `false`，关闭时 `/mcp` 一律拒绝；启停名单存 `mcp_disabled_tools`，
  要打开某个写工具，在名单里写 `-<工具名>`（负号表示显式启用），这条设计是为了避免误清空表单后写工具全开。
  `mcp_max_result_bytes`（默认 8192）限制单次返回体积。
- `/admin/mcp-servers`：登记外部 MCP 服务。字段是 `name`、`endpoint`、`transport`
  （当前只支持 `streamable_http`）、`purpose`（`general` / `moderation`）、`moderation_tool`
  （审核用途调用的工具名，默认 `check_content`）、`enabled`。`headers` 里的鉴权头在响应中脱敏。
  「测试」→ `POST /admin/mcp-servers/{id}/test` 验证连通性。

`purpose=moderation` 的服务会被 `moderation_mode=external` 的审核链路优先调用；
调用失败时回退到异步审核。协议与工具契约见 [MCP](/api/mcp/)。

## 审计日志

`/admin/audit-logs` 是敏感操作的流水（`POST /admin/audit_logs/list`），按操作者与动作过滤。
每条记录有 `actor_id`、`action`、`target`、`detail`、`created_at`。

常见动作名：`settings.update`、`settings.batch_update`、`user.ban`、`user.unban`、`user.role_update`、
`user.delete`、`moderation.approve`、`moderation.reject`、`report.status`、`trash.restore_post`、
`trust.xp_adjust`、`gate.rule_save`、`admin.batch_users.*`。排查「谁动了这条数据」先看这里。

## 安全事件与会话

- `/admin/security`：安全事件列表，字段含类型、严重级别（`low` / `medium` / `high` / `critical`）、
  用户、IP、详情、是否已处理。它展示的是**当前登录账号**的事件，不是全站流水。
- `/admin/sessions`：当前账号的会话列表（设备、最近活跃），可单条撤销或「全部登出」。

处置他人登录状态没有单独开关：改角色 / 封号会吊销对方全部会话，这是唯一的设计入口。

## 验证

1. 新建一个只读 API Key，用 `curl -H "X-API-Key: ak_..."` 调 `POST /posts/create`：应 403（scope 不含写）。
2. 给 Webhook 点一次 Ping：投递记录里应出现 `event=ping`、`status=success`，接收方收到签名头。
3. 创建备份后下载，文件是 JSON；恢复后抽查一条设置与一篇帖子是否回到快照状态。
4. 打开 `/admin/mcp`，「写工具」默认是关闭的；开启一个后调用对应工具应成功。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| API Key 请求 403「认证未启用」 | `api_key_enabled` 还是 `false` | 在服务设置里打开 |
| Key 创建 403「超出创建者的权限」 | scope 展开后含创建者没有的权限码 | 降低 scope，或换权限更高的账号创建 |
| Webhook 一直重试 | 接收端非 2xx 或超时 | 看投递记录的响应码与 `last_error` |
| 备份恢复后配置没变 | 恢复的是旧快照，或恢复失败未报错 | 核对备份时间与大小，重试并看日志 |
| MCP 工具调用被拒 | 总开关关闭、工具在停用名单，或 Key 缺对应权限码 | 查 `mcp_enabled`、`mcp_disabled_tools` 与 Key scope |
| 存储改完上传仍走旧后端 | 存储是启动期单例 | 重启后端进程 |
