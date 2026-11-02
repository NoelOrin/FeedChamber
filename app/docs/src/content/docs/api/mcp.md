---
title: MCP 集成
description: 用 API Key 通过 POST /mcp 调用 15 个工具，理解启用开关、工具级权限、停用名单与外部 MCP 客户端
---

`POST /mcp` 让 AI 客户端（Claude Desktop、Cursor、Cline 等）读写本站内容与站点自定义
配置。它说 JSON-RPC 2.0，**不套**论坛信封，鉴权只用 API Key。反向也成立：站点可以作为
MCP Client 调外部服务，主要用途是内容审核。

## 前置条件

两个开关没开，`/mcp` 就一律拒；`mcp_disabled_tools` 是工具停用名单，不是第三个开关。
另外需要一枚 API Key（`POST /api/v1/api-keys`），
scope 覆盖要用的工具权限码，建 Key 的账号本身也要有对应权限（scope 是上限而非替代品）。

| 设置键 | 默认 | 作用 |
|--------|------|------|
| `mcp_enabled` | `false` | MCP Server 总开关 |
| `api_key_enabled` | `false` | 凭证层开关；关掉等于切段这条通道 |
| `mcp_disabled_tools` | 空 | 工具停用名单（写工具默认停用） |

## 接入

端点挂在**应用根**：`https://你的域名/mcp`。它以 `POST` 收发，`OPTIONS` 返回 204。

```json
{ "mcpServers": { "feedchamber": {
  "type": "http",
  "url": "https://forum.example.com/mcp",
  "headers": { "X-API-Key": "ak_..." }
} } }
```

用 `Authorization: Bearer ak_...` 也可以，两种头等价。

## 协议形状

```bash
curl -s -X POST https://forum.example.com/mcp \
  -H 'X-API-Key: ak_...' -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

响应是 `{"jsonrpc":"2.0","id":1,"result":{"tools":[…]}}`。支持的方法只有四个入口：
`initialize`、`tools/list`、`tools/call`，以及不需要结果的 `notifications/initialized`
与 `ping`。`initialize` 也要求凭证——MCP 服务本身不是公开信息面。协商回报的协议版本是
`2025-06-18`，`serverInfo` 为 `{"name":"feedchamber","version":"1.0.0"}`。

`tools/list` 只列出**当前身份有权调用且已启用**的工具，按名称排序。看不到某个工具通常
不是 bug，而是权限或停用状态。

## 两层错误

协议问题走 JSON-RPC 层的 `error`；业务失败走**工具级**结果（`content[].text` +
`isError: true`），客户端据此区分"通信失败"与"工具执行失败"。

| 码 | 含义 |
|----|------|
| `-32700` / `-32600` | 不是合法 JSON / 不是合法 JSON-RPC 2.0 请求 |
| `-32601` / `-32602` | 未知方法或工具 / 参数不合法（含工具参数解码失败） |
| `-32010` | 工具存在但**被停用**（与"不存在"刻意区分） |
| `-32011` | 权限 / 封禁 / 维护模式拒绝 |
| `-32012` | 超出该 Key 的限流预算 |

`-32010` 与 `-32601` 分开，是为了让客户端能提示"请站长开启工具"而不是怀疑工具名。

## 15 个工具

| 工具 | 作用 | 权限码 | 写 |
|------|------|--------|----|
| `search_posts` | 搜索帖子（遵循可见性） | `post:read` | |
| `get_post` | 按 ID 读帖子详情 | `post:read` | |
| `list_spaces` | 列版面树 | `post:read` | |
| `list_tags` | 列标签（分页） | `post:read` | |
| `get_user_profile` | 按 ID 读公开资料 | `user:read` | |
| `list_snippets` | 列全部自定义片段（含草稿） | `customize:read` | |
| `create_post` | 发帖（初始状态遵审核策略） | `post:create` | ● |
| `create_comment` | 发评论 / 回复 | `comment:create` | ● |
| `like_post` | 点赞或取消（toggle） | `content:like` | ● |
| `favorite_post` | 收藏或取消（toggle） | `favorite:self` | ● |
| `upsert_snippet` | 写 CSS / JS / HTML 片段 | `customize:write` | ● |
| `set_theme_tokens` | 覆盖主题 token | `customize:write` | ● |
| `upsert_nav_item` | 写导航项 | `customize:write` | ● |
| `upsert_home_module` | 写首页模块 | `customize:write` | ● |
| `upsert_custom_page` | 写自定义页 `/p/{slug}` | `customize:write` | ● |

没有"创建 API Key"或"改角色"这类工具——避免 AI 通过一条通道给自己扩权。

写工具**默认全部停用**，要在后台「MCP 工具」（`/admin/mcp`）里逐个打开。停用名单存在
`mcp_disabled_tools`，编码是**相对默认的覆盖**而不是完整启停表：

| 名单内容 | 效果 |
|----------|------|
| 空 | 读工具全开，写工具全关 |
| `search_posts` | 停用这个读工具 |
| `create_post` | 该写工具保持停用 |
| `-create_post` | **显式启用** `create_post` |

用负号表达"开启"是为了让误清空名单回到最安全的状态（全关）。单条切换走
`POST /api/v1/admin/mcp/tools/status`，body `{"name":"create_post","enabled":true}`，
需要 `setting:manage`，立即落库。

## 授权模型

- **凭证**：`ak_` 前缀、bcrypt 校验、检查过期与吊销；`role=ban` 直接拒；禁言者读工具
  照常、写工具 403；维护模式只放管理员写。
- **权限**：scope **且** 角色权限；`customize:publish` 只认 scope，不随角色继承。
- **限流**：按 Key 的 `rate_limit`，未设置时 120 次/分钟。

`customize:publish` 不继承是刻意的：admin 角色本来持有它，若继承则"admin 创建的任意
Key"都能让 JS 直接生效。持 `customize:write` 的 Key 写 JS / HTML / 导航 / 首页模块 /
自定义页时会被强制存为草稿，返回 `applied: false` 并附 `hint`。

工具调用以**调用者身份**进 service 层，可见性、所有权、门控规则与 HTTP 一致：作者能读
自己的待审帖，`login` / `level` 版面按等级判定。MCP 没有审核旁路——`create_post` 的初始
状态照旧由 `moderation_mode` 决定。

单条结果文本上限由 `mcp_max_result_bytes` 控制（默认 8192 字节），超出会截断并标注；每次
调用都写审计（前缀 `mcp.tool.`，记录调用者、工具名、成败与 200 字符内摘要），审计写失败
只记日志、不阻断调用。

## 反向：作为 MCP Client

站点可以登记外部 MCP Server（后台「外部 MCP」，`/admin/mcp-servers`），在 `moderation_mode=external`
时优先用它做审核。端点 `POST /api/v1/admin/mcp-servers/{list|create|{id}/update|{id}/delete|{id}/test}`
都需要 `setting:manage`；`test` 会握手并列出工具，用来验证配置。

| 约束 | 说明 |
|------|------|
| 传输 | 只支持 Streamable HTTP；记住服务返回的 `Mcp-Session-Id` 并在后续请求回传 |
| SSRF | 端点必须是 `http(s)`，拒绝回环 / 私网 / 链路本地地址与 `localhost` |
| 重定向 | **不跟随**：自定义鉴权头不会被带到重定向目标 |
| 鉴权头 | 存为字符串到字符串的 JSON 对象；接口只返回值键名，**不回显值** |
| 超时 / 体积 | 单次调用 3 秒；响应上限 1 MB，超出报"响应过大"而非静默截断 |

配置里把 `purpose` 设为 `moderation` 表示该服务承担审核职责，`moderation_tool` 指定
工具名（留空用 `check_content`）。审核结论接受 `{"action":"allow"}` 或
`{"content":[{"type":"text","text":"allow"}]}` 两种形态。

**回退规则**：没登记审核用途的服务 → 静默走既有 HTTP 审核契约；登记了但调用失败 →
记一条警告后回退 HTTP 契约。两种情况都不会让发帖流程挂住。

## 验证

```bash
curl -s -X POST https://forum.example.com/mcp \
  -H 'X-API-Key: ak_...' -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '.result.tools[].name'
```

应拿到工具名列表而不是 `-32011`。后台「MCP 工具」页显示同一份启用状态，审计日志里能看到
`mcp.tool.<工具名>` 的调用记录。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| `-32011 MCP 服务未启用` | 两个开关有一个没开 | 打开 `mcp_enabled` 与 `api_key_enabled` |
| `-32010` | 工具被停用 | 后台打开，或从 `mcp_disabled_tools` 移除 |
| `-32601` / 清单少工具 | 名字写错，或 Key 与账号权限不足 | 先 `tools/list`，再两边补权限 |
| 写入返回 `applied: false` | 高风险内容落草稿 | 后台启用，或给 Key 加 `customize:publish` |
