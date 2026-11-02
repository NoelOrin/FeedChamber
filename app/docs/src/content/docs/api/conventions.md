---
title: API 约定
description: RPC 风格的路径与方法规则、统一信封、分页、排序、ID 与时间格式，以及重试与版本兼容
---

所有业务接口挂在 `/api/v1` 下，并遵循同一组约定。这一页把这些约定一次说清，
参数细节查 [端点索引](/reference/endpoints/) 或 OpenAPI 规格。

:::note[先记三条]
写操作一律 `POST`，只读查询才可以 `GET`；业务参数走请求体；响应永远是
`{ code, msg, data }`。
:::

## 方法选择

判断依据是**这次请求会不会改变服务端状态**，不是"像不像查询"。

| 场景 | 方法 | 例子 |
|------|------|------|
| 创建、删除、状态变更 | `POST` | `POST /api/v1/posts/create` |
| 列表、分页、多条件筛选 | `POST` + `/list` | `POST /api/v1/posts/list` |
| 单资源读取、轻量状态查询 | `GET` | `GET /api/v1/posts/{id}` |
| 文件字节、协议端点、爬虫文件 | `GET` | `GET /api/v1/upload/file/{key}` |

`PUT` / `PATCH` / `DELETE` 在业务接口上一律不存在；同一资源的读取只保留一种方法，
不会同时提供 `GET /posts/{id}` 与 `POST /posts/{id}`。当前规格的 314 个操作里，
278 个是 `POST`、36 个是 `GET`。

## 路径动作词

动作段是小写单词，不加连字符：

| 动作段 | 含义 | 例子 |
|--------|------|------|
| `/list` | 列表与分页 | `POST /notifications/list` |
| `/create` | 创建 | `POST /posts/create` |
| `/{id}/update` | 更新 | `POST /posts/{id}/update` |
| `/{id}/delete` | 删除 | `POST /posts/{id}/delete` |
| `/{id}/status` | 状态变更 | `POST /reports/{id}/status` |
| `/{id}/<动作>` | 其他业务动作 | `POST /posts/{id}/pin`、`/lock` |

只读 `GET` 不做动词后缀，直接是资源标识（`/posts/{id}/detail`）或既有业务名词
（`/checkin/status`）。多级资源靠嵌套表达归属，例如
`POST /api/v1/posts/{postId}/contents/create` 表示"给这个帖子发评论"。

## 参数传递

- **`POST` 的业务参数全在请求体**，`Content-Type: application/json`；不用 query 传
  筛选条件。
- **`GET` 用 query**，只承载过滤与分页。
- 分页字段固定 `page` 与 `page_size`，可带 `start_time` / `end_time`（RFC 3339 或
  `YYYY-MM-DD`）。字段名是**下划线**风格：发 `pageSize` 不会被识别。
- 空筛选请发 `{}`。多数列表接口容忍空请求体，但 `/posts/list` 直接解析 JSON，空
  body 会得到 `2001`——统一发 `{}` 最省事。

| 限制 | 值 | 说明 |
|------|----|------|
| 请求体上限 | 1 MB | 全站默认 |
| 上传路径上限 | 10 MB | `/api/v1/upload` |
| 单请求超时 | 30 s | 返回 `5004`；SSE 端点不参与 |

## 统一信封

```json
{ "code": 0, "msg": "success", "data": { } }
{ "code": 0, "msg": "success", "data": { "items": [], "total": 128 } }
```

成功时 `code` 恒为 `0`、`msg` 恒为字面量 `success`；失败时 `data` 为 `null`、
`code` 是错误码、`msg` 是默认中文文案。**判断成功只看 `code === 0`**，不要按 HTTP
状态分支，错误码含义见 [错误码](/reference/errors/)。

分页把 `items` 与 `total` 放进 `data`，`total` 是满足筛选条件的**总条数**。带
`{items,total}` 的列表接口既有 `POST` 也有 `GET`，形状一致。

## 分页上限

| 键 | 默认值 | 行为 |
|----|--------|------|
| `page` | `1` | 小于 1 时按 1 处理 |
| `page_size` | `20` | 小于 1 或**大于上限**时回落到 20 |
| `page_max_size` | `100` | 单页上限，来自系统设置 |

越界不是"夹到上限"而是"回落默认值"：传 `page_size: 500` 会拿到 20 条，想要 100 就
正好传 100。页码过大不报错，只是取到空列表。

## 排序

用 `sort_by` 字符串选排序，未知值走默认排序而不是报错：

| 取值 | 排序 |
|------|------|
| 不传或 `latest` | 置顶优先，然后 `last_active_at`、`created_at` 倒序 |
| `hot` | 置顶优先，然后点赞 + 评论 ×2 + 浏览/10 的综合分倒序 |
| `most_commented` | 置顶优先，然后评论数倒序 |

置顶帖始终在最前，且不受 `sort_by` 影响。

## ID 与时间

- 内容与用户实体的 ID 是 **ULID 字符串**（26 位 Crockford Base32，前 10 位是毫秒
  时间戳，因此字典序近似创建顺序）。帖子、评论、用户、版面、标签、举报都是这种形式。
- 少数自增表仍用整数 ID：`POST /api-keys/{id}/revoke` 与 `POST /sessions/{id}/revoke`
  传的是数据库自增主键，不是 ULID。
- 实体字段（`created_at`、`updated_at`）是 **RFC 3339 字符串**；凭证与安全类记录
  （API Key、登录会话）的时间是 **Unix 秒整数**。两种格式并存，按字段所在对象判断。

## 幂等与重试

服务端没有 `Idempotency-Key` 机制，写接口**不是幂等的**：重发一次
`POST /posts/create` 就是两篇帖子。可以放心重试的只有这几类：

| 情况 | 处理 |
|------|------|
| `401`（`1003` / `1014`） | 调 `POST /auth/refresh` 换新 sid 后重试一次 |
| `429`（`4001`） | 指数退避后重试，别原地循环 |
| `5xx`（`5001` / `5004`） | 读接口可重试；写接口先查状态 |
| 点赞 / 收藏 / 关注 | toggle 语义，重试等于撤销，不要自动重试 |

写请求超时后不要盲重试：先按业务键查一次（例如 `/posts/list` 带 `author_id`）确认
上一次是否已生效。

## 版本与兼容

版本只体现在路径前缀 `/api/v1`，没有 header 版本协商，也没有弃用期机制。兼容性靠
三条约定：新增能力走新路径而不改既有语义；响应只增字段，客户端要忽略不认识的字段；
受保护路由的权限映射是**默认拒绝**（新路由没登记权限码就是 403），因此新增端点不会
悄悄对外开放。

三类端点不套论坛信封，按协议形状返回：`POST /mcp` 是 JSON-RPC 2.0（见
[MCP 集成](/api/mcp/)）；`/api/v1/oauth/token`、`/api/v1/sso/token`、`/api/v1/sso/userinfo`、
`/.well-known/*` 是 OAuth 2.0 / OIDC 规范形状（见 [认证与权限](/api/authentication/)）；
爬虫文件（`/sitemap*.xml`、`/robots.txt`、`/latest.rss`）在应用根返回 XML / 文本。

## 反例清单

| 反例 | 为什么错 | 正确写法 |
|------|----------|----------|
| `PUT /spaces/{id}` | 方法禁止 | `POST /spaces/{id}/update` |
| `DELETE /posts/{id}` | 方法禁止 | `POST /posts/{id}/delete` |
| `GET /posts/{id}/delete` | 写操作用了 GET | `POST /posts/{id}/delete` |
| `POST /reports/list?status=pending` | 业务参数走 query | body 里放 `status` |
| `pageSize=50` | 这个字段名不存在 | `page_size: 50` |
| 同一路径同时有 `GET` 与 `POST` | 双读入口 | 只保留一种 |

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 列表永远只有 20 条 | 传了 `pageSize` 或被上限回落 | 改用 `page_size`，不超过 100 |
| `2001 参数无效` | 空 body、字段名拼错、类型不符 | 至少发 `{}`，对照 OpenAPI schema |
| `4002 请求体过大` | 超过 1 MB（上传路径 10 MB） | 改用预签名直传 |
| `403 禁止访问` | 权限码不足或路径未登记映射 | 见 [认证与权限](/api/authentication/) |
