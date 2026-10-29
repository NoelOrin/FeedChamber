# MCP 接入指南

本站作为 **MCP Server** 对外暴露能力，AI 客户端（Claude Desktop / Cursor / Cline 等）可以读取内容、写帖子与评论、以及配置站点自定义。

规范：[`openspec/specs/mcp-server`](../../openspec/specs/mcp-server/spec.md) · 实现：[`app/server/internal/mcp/`](../../app/server/internal/mcp/AGENTS.md)

## 1. 启用

MCP 默认**关闭**。在「设置 → 站点」把 `mcp_enabled` 设为 `true`，否则所有请求都会被拒绝：

```json
{"jsonrpc":"2.0","id":1,"error":{"code":-32011,"message":"MCP 服务未启用"}}
```

同时需要 `api_key_enabled=true`（MCP 用 API Key 鉴权，不认用户会话）。

## 2. 端点

```
POST https://<你的站点>/mcp
```

- 挂在**应用根**，不在 `/api/v1` 下。
- 请求与响应是 **JSON-RPC 2.0**，不套本站的 `{code,msg,data}` 信封。
- 传输为 Streamable HTTP（`Accept: application/json, text/event-stream` 都支持）。

## 3. 创建凭证

在「系统设置 → API Key」创建，`scopes` 用逗号分隔。**精确权限码可以直接当 scope 用。**

| 用途 | 建议 scopes |
|------|-------------|
| 只读浏览 | `post:read,user:read` |
| 发帖评论 | `post:read,post:create,comment:create,content:like,favorite:self` |
| 读站点自定义 | `post:read,customize:read` |
| 改站点自定义（CSS 类） | `post:read,customize:read,customize:write` |
| 让 JS/HTML 直接生效 | 追加 `customize:publish`（**高风险，见下**） |

> ⚠️ **`customize:publish` 只能通过 API Key 的 scope 显式授予，不走角色继承。**
> 即使是 admin 创建的 Key，只要 scope 里没有它，写入 JS/HTML 也会被强制存为草稿。
> 这是刻意的：否则「admin 创建的任意 Key」都自动具备让任意脚本上线到全站的能力。

## 4. 客户端配置

### Claude Desktop（`claude_desktop_config.json`）

```json
{
  "mcpServers": {
    "feedchamber": {
      "url": "https://forum.example.com/mcp",
      "headers": { "X-API-Key": "ak_xxxxxxxx" }
    }
  }
}
```

### Cursor（`.cursor/mcp.json`）

```json
{
  "mcpServers": {
    "feedchamber": {
      "url": "https://forum.example.com/mcp",
      "headers": { "Authorization": "Bearer ak_xxxxxxxx" }
    }
  }
}
```

两种携带方式都支持：`X-API-Key: ak_...` 或 `Authorization: Bearer ak_...`。

## 5. 可用工具

写类工具**默认停用**，需在「系统设置 → MCP 工具」逐个开启。

### 读（默认启用）

| 工具 | 参数 | 说明 |
|------|------|------|
| `search_posts` | `keyword`（必填）, `page`, `pageSize` | 搜索帖子，遵循站点可见性门槛 |
| `get_post` | `id` | 帖子详情（含正文） |
| `list_spaces` | — | 版面树 |
| `list_tags` | `page`, `pageSize` | 标签列表 |
| `get_user_profile` | `id` | 公开用户资料 |
| `list_snippets` | — | 自定义片段（含草稿） |

### 内容写（默认停用）

| 工具 | 参数 | 说明 |
|------|------|------|
| `create_post` | `title`, `body`, `spaceId` | 初始状态遵循站点 `moderation_mode` |
| `create_comment` | `postId`, `body`, `parentId` | 评论 / 回复 |
| `like_post` | `postId` | toggle |
| `favorite_post` | `postId` | toggle |

### 站点自定义写（默认停用）

| 工具 | 参数 | 生效规则 |
|------|------|----------|
| `upsert_snippet` | `id`（空则新建）, `kind`(css/js/html), `position`, `content`, `enabled`, `sort` | css 直接生效；js/html 强制草稿 |
| `set_theme_tokens` | `items[]`(`token`, `value`, `mode`) | 直接生效 |
| `upsert_nav_item` | `id`, `label`, `href`, `icon`, `sort`, `visibility`, `minLevel`, `enabled` | 强制草稿 |
| `upsert_home_module` | `id`, `kind`, `title`, `sort`, `visibility`, `minLevel`, `config`, `enabled` | 强制草稿 |
| `upsert_custom_page` | `id`, `slug`, `title`, `content`, `status` | 强制草稿 |

## 6. 分级生效：`applied` 字段

自定义层写工具的返回里一定有 `applied`。**调用方必须读它**，不要假设「调用成功 = 已生效」：

```json
{
  "applied": false,
  "reason": "机器凭证未持 customize:publish 权限，已保存为草稿，需在后台启用",
  "hint": "本次写入未生效，需站长在后台「站点自定义」中启用",
  "snippet": { "id": "...", "enabled": false, "kind": "js", ... }
}
```

- `applied: true` — 已对外生效
- `applied: false` — 已保存为草稿，需站长在后台启用

## 7. 错误码

| 码 | 含义 | 处理建议 |
|----|------|----------|
| `-32700` | 请求不是合法 JSON | 检查序列化 |
| `-32600` | 不是合法 JSON-RPC 2.0 | 检查 `jsonrpc: "2.0"` 与 `method` |
| `-32601` | 未知方法 / 未知工具 | 先 `tools/list` |
| `-32602` | 参数非法 | 对照 `inputSchema` |
| `-32010` | 工具已被停用 | 让站长在后台开启 |
| `-32011` | 权限不足 / 被禁言 / 维护中 / 未启用 | 读 `message` |
| `-32012` | 超出限流预算 | 降低频率 |

业务失败（如「帖子不存在」）走**工具级** `isError: true` 的结果，不是 JSON-RPC error —— 客户端据此区分「工具执行失败」与「通信失败」。

## 8. 审计

每次 `tools/call` 都写 `audit_logs`：

- `action` = `mcp.tool.<工具名>`
- `detail` = 成功/失败 + 结果摘要（自定义层写入另有一条 `customize.*` 记录）

在「系统设置 → 审计日志」按 `mcp.tool.` 前缀筛选即可看到全部 AI 操作。

## 9. 安全建议

1. **给 MCP 专用的 Key**，不要复用给其他自动化。
2. **设过期时间**，尤其是持有 `customize:write` 的 Key。
3. **`customize:publish` 单独授权**，只在确实需要 AI 直接上线 JS 时使用。
4. **写工具按需开启**，不用的就关掉。
5. **定期查审计日志**，关注 `mcp.tool.upsert_snippet` 这类高权限调用。
6. token 泄漏等于站点可被注入任意脚本——泄漏后立刻吊销 Key 并检查「站点自定义」里是否有异常片段。
