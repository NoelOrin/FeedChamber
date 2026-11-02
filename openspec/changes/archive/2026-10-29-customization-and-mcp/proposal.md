## Why

Feed Chamber 现在几乎没有「不改代码就能改站点」的能力，也没有任何机器可调用面。主题只有两个硬编码项（`service/theme.go`），导航项硬编码在 `components/layout/nav.tsx`，首页模块写死，没有自定义 CSS/JS 注入点，没有自定义页面；同时全仓无任何 MCP 痕迹，AI 客户端无法连接本站读写内容，也无法驱动站点自定义。外部 AI 能力同样是硬编码的：`moderation_mode=external` 时 `service/moderation.go:CheckExternal` 写死了一个 HTTP POST + Bearer 契约，换不了 MCP 生态里的能力。

同时地基已经够用：`api_keys` 表已有 `Scopes`/`Status`/`RateLimit`/`ExpiresAt` 与 scope→权限码映射（`gateway/apikey.go`），56 个权限码，`audit_logs` 审计表，`gate_rules` 门控规则引擎。

核心诉求：自定义能力不仅要能在后台点，还要能被 **AI 通过 MCP + 鉴权 token 驱动**。

## What Changes

分三个阶段交付，每阶段独立可上线。

### P1 — 站点自定义层（外观 + 信息架构）

- 新增 5 张表：`snippets`（多段 CSS/JS/HTML，可逐段启停，位置 `head`/`body_end`）、`theme_tokens`（主题 token 覆盖）、`nav_items`（自定义导航）、`home_modules`（首页模块）、`custom_pages`（自定义静态页，路由 `/p/{slug}`）。
- 自定义 CSS/JS 允许任意内容，仅 `admin` 角色可写，每次写入落 `audit_logs`（含 diff 摘要）。
- 主题 token 覆盖 `packages/tokens` 的 CSS 变量（`--primary`、`--radius` 等），后台给色板与滑杆。
- `nav.tsx` 改为「配置优先，无配置回退硬编码」；`/timeline` 支持按 `home_modules` 渲染。

### P2 — MCP Server（对外暴露能力，含驱动自定义）

- 新增 `/mcp` 端点（app 根，不在 `/api/v1` 下），JSON-RPC 2.0，Streamable HTTP，实现 `initialize` / `tools/list` / `tools/call`。
- 直调 service 层（不自调用 HTTP），工具与后台 UI 共用同一组 service 函数。
- 凭证复用 `X-API-Key` / `Bearer ak_`。**`/mcp` 不在 `gw.Enforce()` 覆盖内，MCP 层 MUST 自己补齐维护模式、ban、权限、限流四项检查。**
- 新增独立权限域 `customize`：`customize:read` / `customize:write` / `customize:publish`，**MUST NOT 并入既有 `admin` 或 `write` scope**（否则既有 admin Key 立即获得注入 JS 的能力，构成权限提升）。
- 分级生效：CSS / theme token 持 `customize:write` 即可直接生效；JS / HTML snippet / 导航 / 首页模块 / 自定义页 MUST 落草稿（`enabled=false`），需站长在后台启用，或 token 额外持 `customize:publish`。
- 工具目录：读工具（`search_posts`/`get_post`/`list_spaces`/`list_tags`/`get_user_profile`）、内容写工具（`create_post`/`create_comment`/`like_post`/`favorite_post`，走既有 `moderation_mode`）、自定义层工具（6 个）。
- 工具级开关，默认写工具与自定义写工具均关闭；每次 `tools/call` 落 `audit_logs`。

### P3 — MCP 客户端（把外部 MCP 当能力源）

- 新增 `mcp_servers` 表登记外部 MCP Server；新增 `service/mcp_client.go` 最小客户端（`initialize` → `tools/list` → `tools/call`）。
- `moderation_mode=external` 时优先走 MCP `tools/call`，否则回退既有 HTTP 契约（向后兼容）。
- 后台 `/admin/mcp-servers` 支持登记与连通性测试。

### Non-goals

- 不动插件系统（`plugins` 表保持仅元数据，不引入 `.so` / 脚本沙箱），与 `docs/design/extension-points.md` 原则一致。
- P1 不做自定义字段（拆为独立提案 `P1.5-custom-fields`）。
- 不做 MCP 的 resources / prompts，只做 Tools。
- 不做 stdio 传输，服务端只做 Streamable HTTP。
- 不做 MCP 规范里的 OAuth 2.1 授权服务器模式，P2 只用 API Key。
- MCP 工具集 MUST NOT 包含创建/提权 API Key 的能力，避免 AI 自我扩权。

**BREAKING**：无。全部为新增表与新增端点；`nav.tsx` 改为配置优先、无配置回退；既有 API Key 权限集不变（新 scope 需显式勾选）。

## Capabilities

### New Capabilities

- `site-customization`: 站点自定义层的数据模型、生效语义、审计要求，以及「AI 写入必须分级生效」的安全约束（P1 + P2 对自定义层的约束）。
- `mcp-server`: 站点对外暴露的 MCP Server——协议形状、凭证与权限模型、`customize` 权限域、工具目录、工具级开关与审计要求（P2）。
- `mcp-client`: 把外部 MCP Server 作为能力源——登记、连通性测试、以及 `moderation_mode=external` 的 MCP 调用与回退顺序（P3）。

### Modified Capabilities

- `api-style`: `/mcp` 是协议端点，需在「系统 / 协议端点 MUST 使用 GET」的例外清单中显式登记，并说明其请求体为 JSON-RPC 而非论坛信封。
- `layered-architecture`: 新增 `internal/mcp` 包，需明确其包位置与「不得跨层直访 repository」的约束。

## Impact

- **Affected code**：
  - 后端：`internal/model/`（5+2 张表）、`internal/repository/`、`internal/service/`、`internal/handler/`、`internal/mcp/`（新包）、`internal/router/`、`server/iris.go`、`server/seed.go`、`internal/container/`、`internal/gateway/apikey.go`（新增 scope 登记）
  - 前端：`packages/api-client/src/services/`（新增 4 个 service）、`app/web/app/admin/`（新增 6 个路由）、`components/layout/nav.tsx`
  - 共享包：`packages/tokens/`（运行时 token 覆盖注入层）
- **Affected APIs**：新增 `/mcp`、`/p/{slug}`、`/api/v1/admin/customization/*`、`/api/v1/admin/mcp-servers/*`
- **Security impact**：新增一个可注入 JS 的高权限 token 面。缓解：独立 scope + 草稿优先 + 审计全覆盖 + 建议强制过期 + 后台「当前生效 JS」一眼可见。
