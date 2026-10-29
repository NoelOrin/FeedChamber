# Tasks — 深度自定义与 MCP 能力

提案：[proposal.md](proposal.md) · 设计：[design.md](design.md)

分三阶段，每阶段独立可上线。**P1 必须先稳下 service 签名**，P2 只做适配层（见 design D2/D7）。

## 1. P1 权限域与数据模型

- [x] 1.1 `model/permission.go` 新增 `PermCustomizeRead = "customize:read"`、`PermCustomizeWrite = "customize:write"`、`PermCustomizePublish = "customize:publish"`，登记进 `DefaultPermissions`（含中文名）
- [x] 1.2 `model/snippet.go` 新增 `Snippet`（ID/Kind/Position/Content/Enabled/Sort/UpdatedBy/UpdatedAt），Kind ∈ `css|js|html`，Position ∈ `head|body_end`
- [x] 1.3 `model/theme_token.go` 新增 `ThemeToken`（Token/Value/Mode/UpdatedBy/UpdatedAt），Mode ∈ `all|light|dark`
- [x] 1.4 `model/nav_item.go` 新增 `NavItem`（ID/Label/Href/Icon/Sort/Visibility/MinLevel/Enabled）
- [x] 1.5 `model/home_module.go` 新增 `HomeModule`（ID/Kind/Sort/Visibility/MinLevel/Config/Enabled），Kind ∈ `latest|hot|spaces|html`
- [x] 1.6 `model/custom_page.go` 新增 `CustomPage`（ID/Slug/Title/Content/Status/UpdatedBy/UpdatedAt），Status ∈ `draft|published`
- [x] 1.7 `server/iris.go` 的 `AutoMigrate` 追加 5 个模型；`server/seed.go` 种子 3 个新权限码到 admin 角色

## 2. P1 仓储与服务层

- [x] 2.1 `repository/snippet.go`、`theme_token.go`、`nav_item.go`、`home_module.go`、`custom_page.go`：接口 + SQL 实现，错误统一 `WrapDBErr`
- [x] 2.2 `service/customize_iface.go`：定义 `CustomizeService` 接口（snippets / tokens / nav / modules / pages 五组 CRUD + 启用停用 + 前台读取聚合）
- [x] 2.3 `service/customize.go`：实现上述接口，写操作统一走 `auditLogs` 记录（actor / action / target / diff 摘要）
- [x] 2.4 `service/customize.go` 实现**分级生效判定**：入参含「调用来源」（`ui` / `machine`）与凭证是否持 `customize:publish`；JS/HTML snippet、nav、module、page 在 machine 且无 publish 时强制 `enabled=false` / `status=draft`
- [x] 2.5 写操作返回体包含 `applied bool` 与 `reason string`，供 MCP 与后台共用
- [x] 2.6 `custom_page` 的 slug 唯一性与保留字校验（与既有顶层路由冲突时拒绝，返回 `2xxx`）
- [x] 2.7 前台聚合读取：`GetActiveSnippets(ctx, position)`、`GetThemeOverrides(ctx)`、`GetPublicNav(ctx, viewer)`、`GetHomeModules(ctx, viewer)`——按可见性过滤
- [x] 2.8 `container/container.go` 装配 repo 与 `CustomizeService`；`server/wiring.go` 映射进 `handler.Deps`

## 3. P1 HTTP 层与后台

- [x] 3.1 `handler/customize.go`：`/api/v1/admin/customization/snippets/list|create|{id}/update|{id}/delete|{id}/toggle`、`tokens/list|set|delete`、`nav/list|create|{id}/update|{id}/delete`、`modules/list|create|{id}/update|{id}/delete`、`pages/list|create|{id}/update|{id}/delete|{id}/publish`（全部 `POST` + 动词后缀，带 swag 注解）
- [x] 3.2 `router/routes_admin.go`：注册上述路由，权限门 `PermCustomizeWrite`（读取用 `PermCustomizeRead`）；**同时在 `gateway/permission.go` 补 path→permission 映射**（漏了会被网关以 1013 拒绝，且报错指不出原因）
- [x] 3.3 `handler/custom_page.go` 或并入既有公共面：`GET /api/v1/pages/{slug}` 返回已发布页内容（草稿返回 404）
- [x] 3.4 `app/web/app/(app)/p/[slug]/page.tsx`：自定义页渲染（Markdown）
- [x] 3.5 `packages/api-client/src/services/customization.ts` + `types.ts` + `constants.ts`（新增 3 个权限码）
- [x] 3.6 `app/web/app/admin/customize/snippets/page.tsx`：片段列表 + 编辑器 + 启停 + 「草稿」徽标
- [x] 3.7 `app/web/app/admin/customize/theme/page.tsx`：色板与圆角滑杆，实时预览，保存即生效
- [x] 3.8 `app/web/app/admin/customize/nav/page.tsx`：导航项 CRUD + 拖拽排序 + 可见性
- [x] 3.9 `app/web/app/admin/customize/home/page.tsx`：首页模块 CRUD + 排序
- [x] 3.10 `app/web/app/admin/customize/pages/page.tsx`：自定义页 CRUD + 发布
- [x] 3.11 `components/admin/layout/admin-shell.tsx` 登记「自定义」菜单组；`lib/admin/auth/route-access.ts` 登记权限码
- [x] 3.12 后台增加「当前生效的 JS / CSS」汇总视图（一次看全站点在跑什么）

## 4. P1 前台注入与导航改造

- [x] 4.1 `app/web/app/layout.tsx`：服务端读取启用片段，`head` 段内联注入 `<style>` / `<script>`；`body_end` 段通过客户端组件延迟注入
- [x] 4.2 主题 token 覆盖：把 `GetThemeOverrides` 结果生成为 `:root` / `.dark` 内联 CSS 变量块，置于 tokens.css 之后
- [x] 4.3 配置化导航接入**侧边栏**（`components/aside/sidebar-content.tsx` 的 `CustomNav`）与首页快捷入口（`home-feed.tsx`）；无配置时两处都回退既有默认，不渲染空导航
- [x] 4.4 `/timeline` 按 `home_modules` 渲染（`components/home/home-modules.tsx`），无配置时组件返回 null → 回退现有布局
- [x] 4.5 注入内容做基本转义审查（`</script>` 逃逸等）并记录：这是**有意允许**的任意 JS，不引入 CSP 阻断，但在后台明确提示风险

## 5. P2 MCP 骨架与鉴权

- [x] 5.1 `internal/mcp/jsonrpc.go`：JSON-RPC 2.0 请求/响应/错误结构与编解码（不套论坛信封）
- [x] 5.2 `internal/mcp/enforcer.go`：`Enforcer` 四项检查——维护模式（非 admin 拒绝写）、ban/禁言、权限码、限流（复用 `internal/pkg/ratelimit`）
- [x] 5.3 `internal/mcp/server.go`：`initialize` / `tools/list` / `tools/call` 分发；`tools/call` 前置 `Enforcer`
- [x] 5.4 `internal/mcp/registry.go`：工具注册表（名称 / 描述 / JSON Schema / 所需权限 / 是否写类 / 是否默认启用）
- [x] 5.5 `internal/gateway/apikey.go`：**不改动** `apiKeyScopeRegistry` 既有项；仅确认 `customize:*` 可作为精确 scope 直接使用（`validAPIScopes` 已支持含 `:` 的精确码）
- [x] 5.6 `router/router.go`：在 app 根注册 `app.Post("/mcp", ...)`（与 `/.well-known/*` 相邻位置），**不改动 party 结构**（design Risks）
- [x] 5.7 新增设置项 `mcp_enabled`（默认 false）；关闭时 `/mcp` 返回明确的未启用错误

## 6. P2 工具实现

- [x] 6.1 读工具：`search_posts`、`get_post`、`list_spaces`、`list_tags`、`get_user_profile`（复用既有 service，遵循可见性门槛）
- [x] 6.2 内容写工具：`create_post`、`create_comment`、`like_post`、`favorite_post`（走既有 service 与 `moderation_mode`，不引入审核旁路）
- [x] 6.3 自定义层工具：`list_snippets`、`upsert_snippet`、`set_theme_tokens`、`upsert_nav_item`、`upsert_home_module`、`upsert_custom_page`（调 `CustomizeService`，`source=machine`）
- [x] 6.4 工具级开关表（或设置项）+ 默认关闭写类工具；被停用工具不出现在 `tools/list`
- [x] 6.5 每次 `tools/call` 写 `audit_logs`（action = `mcp.tool.<name>`，detail 含参数摘要与 applied 状态）
- [x] 6.6 明确排除「创建/提权 API Key」类工具（规格要求）
- [x] 6.7 结果体积上限：正文截断 8KB（design Open Questions 的暂定值）

## 7. P2 后台与文档

- [x] 7.1 `app/web/app/admin/mcp/page.tsx`：MCP 总开关、工具清单与逐个启停、连接信息（端点 + 示例 Key 用法）
- [x] 7.2 `app/web/app/admin/api-keys/page.tsx`：新增 `customize:read` / `customize:write` / `customize:publish` 三个快捷勾选按钮；勾选 publish 时显示「等于向全站访客执行任意脚本」的红色提示
- [x] 7.3 `docs/design/mcp.md`：对外使用说明（如何用 Claude Desktop / Cursor 连接、可用工具、权限模型）

## 8. P3 MCP 客户端

- [x] 8.1 `model/mcp_server.go` 新增 `MCPServer`（ID/Name/Endpoint/Transport/Headers/Enabled/Purpose），`Purpose` ∈ `moderation|general`；`AutoMigrate` 追加
- [x] 8.2 `repository/mcp_server.go` + `service/mcp_client.go`：`initialize` → `tools/list` → `tools/call`，超时与失败隔离
- [x] 8.3 `service/mcp_client.go`：`TestConnection`（返回工具清单或失败原因，不修改已存配置）
- [x] 8.4 `service/moderation.go`：`CheckExternal` 增加 MCP 分支——存在启用且 `Purpose=moderation` 的条目时走 MCP，失败回退既有 HTTP 契约（design D6）
- [x] 8.5 `handler/mcp_server.go` + `router/routes_admin.go`：`/admin/mcp-servers/list|create|{id}/update|{id}/delete|test`，凭据响应脱敏
- [x] 8.6 `app/web/app/admin/mcp-servers/page.tsx`：登记 + 连通性测试 + 工具清单展示
- [x] 8.7 `packages/api-client/src/services/mcp-server.ts`
- [x] 8.8 `container/container.go` 装配 `MCPServerRepository` 与客户端

## 9. 测试与验收

- [x] 9.1 **安全回归（必做）**：断言 `apiKeyScopeRegistry` 的 `read` / `write` / `admin` 三个 scope 的权限集**不含**任何 `customize:*`（防权限提升）
- [x] 9.2 **安全回归（必做）**：被封禁 / 禁言用户的 API Key 调用 MCP 写工具必须失败（防绕过网关）
- [x] 9.3 `internal/mcp` 单测：`initialize` / `tools/list` / `tools/call` 的 JSON-RPC 形状；停用工具不出现在清单
- [x] 9.4 `service/customize_test.go`：machine 来源写 JS → `enabled=false`；写 CSS → 直接生效；持 publish 写 JS → 直接生效；响应含 `applied`
- [x] 9.5 `router/middleware_chain_test.go`：追加断言，`/mcp` 注册未改动 party 链
- [x] 9.6 端到端：空配置启动时导航/首页与现状一致（回退路径验证）
- [x] 9.7 `make swag` 重新生成 `app/server/docs/` 与根 `openapi.json`；`openspec validate --all` 通过
- [x] 9.9 **回归（必做）**：`TestAdminCustomizeRoutesHaveGatewayPermission` —— 路由注册了但网关映射漏了会被 1013 拒绝，且读接口必须映射到 `customize:read` 而非被写权限覆盖
- [x] 9.10 **P3 回归**：`TestCheckExternal_*` 五条 —— MCP 审核优先、未配置静默回退、调用失败回退不静默放行、未知结论回退
- [x] 9.8 更新根 `AGENTS.md` 模块文档索引与相关模块 `AGENTS.md`（新增 `internal/mcp/` 文档）
