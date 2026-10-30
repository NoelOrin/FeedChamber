# MCP Server（`internal/mcp/`）

站点对外暴露的 MCP Server（P2）。挂在应用根 `/mcp`，让 AI 客户端（Claude Desktop / Cursor / Cline）以 API Key 凭证读写本站内容与站点自定义配置。

> 上级：[app/server/AGENTS.md](../../AGENTS.md) · 规范：[openspec/specs/mcp-server](../../../../openspec/specs/mcp-server/spec.md) · 设计：[design.md](../../../../openspec/changes/customization-and-mcp/design.md)

## 为什么这个包必须自带鉴权

`/mcp` 注册在应用根，**不在 `/api/v1` 之下**——MCP 客户端按约定路径发现服务。代价是它拿不到 `gw.Preflight()` / `gw.Enforce()` 的任何保护。

因此 `Enforcer` 不是可选项。四项检查一个都不能省：

| 检查 | 等价物 | 缺失后果 |
|------|--------|----------|
| 凭证（API Key） | `gateway.APIKeyManager` | 任何人可读写 |
| 封禁 / 禁言 | 网关 `banCheck` | **被封禁用户的 Key 仍能通过 MCP 写数据** |
| 权限码 | `gateway.PermissionCheckerMiddleware` | 越权调用工具 |
| 限流 | `pkg/ratelimit` | 无限调用 |

维护模式也在这里判（网关的 `MaintenanceGate` 同样只作用于 `/api/v1`）。

## 文件

| 文件 | 职责 |
|------|------|
| `jsonrpc.go` | JSON-RPC 2.0 请求/响应/错误结构；`AuditActionPrefix` 常量 |
| `enforcer.go` | 鉴权与授权四件套；`Identity` 是鉴权结果 |
| `lookup.go` | `APIKeyLookup` 的 GORM 实现（查 Key + 查用户） |
| `bcrypt.go` | 默认 bcrypt 校验（与 `gateway/apikey.go` 同语义） |
| `registry.go` | 工具注册表 + 工具级启停（写工具默认停用） |
| `tool.go` | `Tool` 定义、`ToolInput`、`ToolResult` |
| `server.go` | `initialize` / `tools/list` / `tools/call` 分发 |
| `tools.go` | 全部工具的注册与实现（读 / 内容写 / 自定义层写） |
| `handler.go` | HTTP 入口 `Serve(ctx)`；`ToolStates` / `SetToolEnabled(ctx,…)`（**落库**）供后台 |

## 三条硬约束

1. **不套论坛信封**。响应是 JSON-RPC 2.0（`{jsonrpc,id,result|error}`），不是 `{code,msg,data}`。这与 `api-style` 里 OAuth/OIDC 协议端点的既有例外同构。业务失败用**工具级** `isError:true` 结果表达，JSON-RPC 层的 `error` 只留给协议问题——客户端据此区分「工具执行失败」与「通信失败」。
2. **直调 service 层**，不自调用 HTTP。MCP 只做适配，不新增业务规则；所有权检查、门控规则、可见性门槛、审核策略全部沿用既有实现。**不得为 MCP 开审核旁路**。
3. **写工具默认停用**。`Registry.Register` 见到 `Write:true` 自动进停用集。持久化在设置 `mcp_disabled_tools`，编码规则见下。

## `mcp_disabled_tools` 的编码

存的是**相对默认的覆盖**，不是完整启停表：

- 名单里出现读工具名 → 停用该读工具
- 名单里出现写工具名 → 该写工具保持停用
- 名单里出现 `-<写工具名>` → **显式启用**该写工具

用负号而不是「完整启用名单」的原因：写工具的安全默认值是「关」。完整名单一旦被误清空（后台保存了空表单），所有写工具会一起打开；负号表达的是「显式开启某一项」，误清空只会回到全关。

## 自定义层工具的分级生效

自定义层工具以 `WriteSourceMachine` 调 `CustomizeService`，由 service 层决定高风险内容是否落草稿：

- **CSS / theme token**：持 `customize:write` 即直接生效（最坏是站点变丑，可撤销）
- **JS / HTML / 导航 / 首页模块 / 自定义页**：强制草稿，需人工启用

`customize:publish` **只能由 API Key 的显式 scope 授予，不走角色继承**。原因：admin 角色默认持有它，若接受角色继承，则「admin 创建的任意 Key」都自动具备让 JS 直接生效的能力，草稿保护对最需要它的那批凭证完全失效。这条由 `enforcer.go` 的 `hasPerm` 特判与 `TestCanPublishCustomize_NotInheritedFromAdminRole` 钉住。

其余权限码一律走 `internal/authz.Allowed`：**API Key 的 scope 与角色权限必须同时覆盖**。此前这里是「scope 或角色」，比网关宽一格——admin 的 read-only Key 只要角色表里有 `customize:write` 就能调 `upsert_snippet`。判据现在只在 `authz` 实现一次，网关、`middleware.RequirePermission`、本包三处共用。

`Enforcer.Enabled` 同时要求 `mcp_enabled` 与 `api_key_enabled` 都为 `true`：站长的「关掉 API Key」是凭证层面的止血开关，`/mcp` 不吃它等于对这条通道失效。

## 工具目录

| 类别 | 工具 |
|------|------|
| 读 | `search_posts` `get_post` `list_spaces` `list_tags` `get_user_profile` `list_snippets` |
| 内容写 | `create_post` `create_comment` `like_post` `favorite_post` |
| 自定义写 | `upsert_snippet` `set_theme_tokens` `upsert_nav_item` `upsert_home_module` `upsert_custom_page` |

**不含**创建/提权 API Key 的工具——避免凭证自我扩权。

## 测试

| 测试 | 钉住什么 |
|------|----------|
| `TestAPIKeyScopeRegistryExcludesCustomize` | scope 注册表（现在位于 `internal/authz`）不得含 `customize:*`（**防权限提升**，源码扫描） |
| `TestAuthorizeAPIKeyCannotExceedScope` | read-only API Key 不得调写工具（**scope 是凭证上限**） |
| `TestAuthenticateRejectsWhenAPIKeyAuthDisabled` | `api_key_enabled=false` 时 /mcp 必须一起拒绝 |
| `TestHandlerSetToolEnabledPersists` / `…RollsBackOnSaveFailure` | 工具启停落库；保存失败回滚内存状态 |
| `TestEnforcer_BannedUserKeyCannotWrite` | 被禁言用户的 Key 不能写（**防绕过网关**） |
| `TestEnforcer_BannedRoleRejected` | 封号角色鉴权阶段即拒 |
| `TestEnforcer_MaintenanceBlocksNonAdminWrites` | 维护模式只放管理员写 |
| `TestCanPublishCustomize_NotInheritedFromAdminRole` | 发布权限不走角色继承 |
| `TestRegistry_DisabledListRoundTrip` | 停用名单编码（清空回到全关） |
| `TestServer_*` | JSON-RPC 形状、停用/未知工具可区分、凭证必需 |

前三条是**安全回归**，任何改动 `Enforcer` 或 scope 注册表的变更都必须让它们保持通过。

## 工具启停必须落库

`Registry.SetEnabled` 只是进程内状态：重启回默认、多实例各存一份，而后台会显示「已停用」——安全方向的错误假象。

- `Handler.SetToolStateStore(store)` 注入持久化出口（`server/mcp.go` 用 `mcpToolStateStore` 适配 `service.SettingService`）；
- `Handler.SetToolEnabled(ctx, name, enabled)` 先算目标状态 → 写 `mcp_disabled_tools` → 成功才保留内存变更；失败回滚并返回错误，后台拿到 500 而不是假成功；
- 启动时 `server/mcp.go` 读同一个 key 并 `ApplyDisabledList`，读写闭合。

## 改这里要注意

- 新增工具必须在 `tools.go` 的 `RegisterAll` 里登记，并明确 `Write` 与 `Perm`。漏了 `Write:true` 会让一个写工具默认启用。
- 新增权限域时，`customize:*` 的处理方式（不进粗粒度 scope 注册表、`publish` 不走角色继承）是模板，不是特例。
- 改 `Enforcer` 的检查顺序会影响错误码语义（`CodeForbidden` / `CodeRateLimited` / `CodeToolDisabled`），客户端依赖这些码做提示。
- 结果体积上限由设置 `mcp_max_result_bytes` 控制（默认 8192），超限截断正文并标注。
