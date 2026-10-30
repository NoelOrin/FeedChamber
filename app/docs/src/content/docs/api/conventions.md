---
title: API 约定
description: RPC 风格、写操作一律 POST、统一响应信封、单一错误码命名空间
sidebar:
  order: 1
---

后端的 HTTP 接口不是 REST，是 **RPC 风格**。这几条是硬约定，所有模块、`openapi.json` 与前端 service 都必须遵守。

## 方法与 URL

**按有无副作用选方法**：

- **写操作**（创建 / 更新 / 删除 / 状态变更）→ 必须 `POST`
- **只读查询** → 可以 `GET`，也可以 `POST`；列表 / 分页 / 多条件筛选推荐 `POST` + `/list`，参数走 body
- `PUT` / `PATCH` / `DELETE` **一律禁止**用于业务接口
- 同一资源的读取只保留一种方法，不允许 `GET` / `POST` 双读入口

URL 用「资源 + 动作」表达，动作词固定：

| 动作 | 路径 |
|------|------|
| 列表 / 分页 | `/list` |
| 创建 | `/create` |
| 更新 | `/{id}/update` |
| 删除 | `/{id}/delete` |
| 状态变更 | `/{id}/status` |
| 其他业务动作 | `/{id}/pin`、`/{id}/lock`、`/{id}/publish` |

系统 / 协议 / 文件类端点必须 `GET`：`/health`、sitemap、robots、RSS、OAuth2/OIDC 协议端点、文件字节读取、认证流程入口。

### 正例与反例

```http
✅ POST /api/v1/spaces/list        body: {} 或筛选条件
✅ POST /api/v1/spaces/create      body: { name, description }
✅ POST /api/v1/spaces/{id}/delete
✅ GET  /api/v1/posts/{id}
✅ GET  /api/v1/checkin/history?year=2026&month=10

❌ PUT    /api/v1/spaces/{id}
❌ DELETE /api/v1/spaces/{id}
❌ GET    /api/v1/spaces/{id}/delete
❌ POST   /api/v1/reports/list?status=pending&page=1
```

## 参数传递

- `POST` 一律走 request body（`application/json`），**不得**用 query string 承载业务参数
- `GET` 允许 query 承载过滤与分页
- 分页字段固定 `page` / `pageSize`

## 统一响应信封

所有出口（handler、网关、中间件）都返回同一个形状：

```json
{ "code": 0, "msg": "success", "data": { } }
```

分页响应的 `data` 是 `{ items, total }`。

:::note[协议端点例外]
OAuth 2.0 / OIDC 与 MCP 端点必须保持 RFC / JSON-RPC 形状，不套这个信封。
比如 `/oauth/token` 返回 `{"access_token":…,"token_type":"Bearer"}`，
`/mcp` 返回 `{jsonrpc,id,result|error}`。
:::

## 错误码

错误码是**单一共享命名空间**，网关与业务层共用，不允许模块自造号段。分段：

| 段 | 含义 |
|----|------|
| `1xxx` | 认证 / 凭证 |
| `2xxx` | 参数 |
| `3xxx` | 资源 |
| `4xxx` | 限流与请求体 |
| `5xxx` | 服务端 |

完整码表见 [错误码参考](/api/errors/)。

`msg` 取该码的默认中文文案；需要补充具体原因时作为第二个参数传入。

## 认证

凭证是**服务端会话**，不是 JWT：

- 登录成功签发 64 位十六进制 `sid`
- 同时支持两种携带方式：HttpOnly cookie `master_session`，或 `Authorization: Bearer <sid>`
- 没有 token 黑名单：凭证是否有效 ≡ 会话行是否存在。登出 / 改密 / 改角色 / 封号都直接删行

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"你的密码"}'
```

拿到的 `sid` 用于后续请求：

```bash
curl -s -X POST http://localhost:8080/api/v1/user/profile \
  -H 'Authorization: Bearer <sid>' \
  -H 'Content-Type: application/json' -d '{}'
```

### API Key

机器调用走 API Key，前缀 `ak_`，两种头都认：

```bash
curl -s http://localhost:8080/api/v1/posts/list \
  -H 'X-API-Key: ak_xxx' -H 'Content-Type: application/json' -d '{}'
```

Key 上带 scope（粗粒度 `read` / `write` / `admin`，或精确权限码如 `post:read`）。
**scope 是凭证上限**：权限判定要求 scope 与角色权限同时覆盖。
`

## 权限

平台级权限码 61 个，另有版面级权限（支持层级继承）。资源粒度的「作者 OR 有权限」在 service 层判定。

后台路由由网关的「路径 → 权限码」映射拦截；新增受保护路径必须登记，否则默认拒绝。
