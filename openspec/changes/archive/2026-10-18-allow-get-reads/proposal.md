## Why

2026-10-18 拍板：**允许只读查询使用 `GET`**，并把这条规则写进规范。

背景（全部为实测，非推断）：

1. `AGENTS.md` 自 2026-06-04（`ad3bcd9`）起规定「全部 HTTP 接口使用 `POST`」，并已固化进 `openspec/specs/api-style`；但 `router.go` 现在仍有 **30 处 `GET`**，规范与代码长期背离。
2. 逐条回溯引入时间：其中 **17 条只读业务接口全部是 2026-06-16 ~ 06-27 新增**（`f24de58`、`9bff72d`、`b7c016c`、`8c8cde3`、`d3de0a4`），即约定生效之后才加的，不是「约定前遗留」；全仓也没有任何 ADR / openspec 提案 / commit message 记录过「读接口允许 GET」。结论是漏整改 + 规范没覆盖现实，而不是有意放宽。
3. 另外 **13 条 `GET` 是协议/浏览器/爬虫/文件字节语义**：`/.well-known/openid-configuration`、`/.well-known/jwks.json`、`/oauth/{provider}/callback`、OAuth Provider `/oauth/authorize`、SSO `/sso/authorize`、`/sso/userinfo`、`/sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt`、`/latest.rss`、`/health`、`/upload/file/{key}`（流式字节，供 `<img src>`）、`/upload/{id}`（302 到文件 URL）。对这些端点强制 POST 会直接破坏 OAuth2/OIDC 互操作、爬虫抓取与浏览器直链。
4. 现行规范文本「仅 `/health` 等纯系统探活接口允许 `GET`」把这 13 条合规端点也一并判成违规；同时「过滤条件不得走 query string」与 `GET /checkin/history?year=&month=` 直接冲突。规则与现实两头都不成立。

继续维持现状的代价是：规范说谎 → 新人无法判断什么才是对的 → 下一批接口继续靠直觉写（2026-08-13 就又在两处加了 GET）。

## What Changes

- **`api-style`（修改）**：把「RPC 风格动词后缀 URL」重写为「HTTP 方法选择与 RPC 风格 URL」。方法判据从「一律 POST」改为**唯一判据 = 有无副作用**：
  - 写操作（创建 / 更新 / 删除 / 状态变更 / 任何改变服务端状态）MUST `POST`，MUST 动词后缀；
  - 只读查询 MAY `GET` 或 `POST`：单资源读取与轻量状态查询 SHOULD `GET`，列表/分页/多条件筛选 SHOULD `POST` + `/list`；
  - **同一资源只暴露一种读方法**（禁止 GET/POST 双读入口）；
  - `GET` 允许 query 承载过滤与分页（字段名仍 `page`/`pageSize`），`POST` 禁止 query 承载业务参数；
  - `PUT` / `PATCH` / `DELETE` 仍禁止；仍禁止 RESTful 的资源层级语义；
  - 明确系统/协议/文件类端点 MUST `GET`（协议互操作、爬虫、浏览器直链），认证流程端点（`/oauth/{provider}/auth`）同族。
- **`frontend-packages`（修改）**：`api-client` 契约由「所有请求默认 `method: 'POST'`」改为「写操作 `POST` + 动词后缀；只读查询 MAY `GET`，且 MUST 与后端方法一致，同一资源只保留一处定义」。
- **不动代码**：不新增、不删除、不改造任何路由；`router.go` 的 30 处 `GET` 在新规则下全部合规，`openapi.json` 无需重生成。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `api-style` — 方法判据（写 POST / 读 MAY GET）、query 参数规则按方法区分、GET 白名单类别、单资源单读入口；前端 service 约定同步放宽。
- `frontend-packages` — `api-client` 的 service 方法约定（写操作 POST，只读 MAY GET）。

## Impact

- **代码**：零改动。`go test` / `pnpm check` 等门禁不受影响。
- **文档**：根 `AGENTS.md`「API 风格约定」、`docs/ARCHITECTURE.md`、`docs/backend-overview.md` §13.1-5 与 §13.2-30、`docs/api-reference.md`、`docs/design/extension-points.md`、`packages/api-client/AGENTS.md`。
- **对外契约**：无变化（路由、请求体、响应信封、错误码均不变）。
- **风险**：两种读风格并存 → 由两条硬约束收敛（写操作必须 POST；同一资源只能有一种读方法）。新增 GET 的前置条件是不可有副作用，需要评审时逐条确认。
