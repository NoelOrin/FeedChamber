## Context

动机见 `proposal.md` 的 Why 段；行为契约见本次 change 的 5 个 spec（`site-customization`、`mcp-server`、`mcp-client`，以及对 `api-style` 与 `layered-architecture` 的 delta）。本节只记录塑造本设计的现状约束。

**既有接缝（可直接复用）**

- `api_keys` 表已有 `Scopes` / `Status` / `RateLimit` / `ExpiresAt` / `LastUsedAt`（`model/api_key.go`）。
- `gateway/apikey.go` 的 `apiKeyScopeRegistry` 把粗粒度 scope 映射到权限码集合；`apiKeyScopesToPerms` 还允许直接把精确权限码当 scope 用。**`admin` 这一项一把映射了 13 个权限码**——这是本设计必须绕开的既有事实。
- 权限码共 56 个，`PermissionService` 有内存缓存 + KV Pub/Sub 多实例同步。
- `audit_logs` 表（`model/audit_log.go`）：`ActorID` / `Action` / `Target` / `Detail` / `CreatedAt`，够用。
- `gate_rules` 门控引擎（`service/gate.go`）：`Evaluate(ctx, domain, GateInput)` 返回可解释的 `GateDecision`。
- `user_preferences` 表：`(UserID, Key)` 复合主键的 KV。
- `packages/tokens` 产出 `tokens.css`（CSS 变量）与 `daisyui.css`。

**关键约束**

1. `/mcp` 必须挂在应用根（MCP 客户端按路径发现服务），而 `v1.Use(gw.Preflight())` 与 `protected.Use(gw.Enforce())` 只作用于 `/api/v1`。因此 `/mcp` **拿不到任何网关保护**——这是本设计最主要的风险来源。
2. `router.go` 的注释明确记载了一个 Iris 陷阱：protected party 的中间件在注册路由那一刻取快照，子 party 与父 party 共用底层数组空闲槽，提前创建会改写后续路由的中间件链。新增路由必须遵守既有注册顺序，不能为了 `/mcp` 去动 party 结构。
3. 前端是 vinext App Router，`app/layout.tsx` 的 `generateMetadata` 与 `loadPublicSettings()` 是服务端接缝；`components/layout/nav.tsx` 是客户端组件。
4. `docs/design/extension-points.md` 已确立「不支持第三方热加载」原则；本设计与之一致（不动插件系统）。

## Goals / Non-Goals

**Goals:**

- 让站长在后台完成外观与信息架构调整，无需改代码发版。
- 让 AI（MCP）能在最小权限、可审计的前提下驱动同一套自定义能力与内容读写。
- 把「AI 写 JS」的爆炸半径压到「后台多一次点击」，同时不牺牲可用性。
- 保持既有 API Key 的权限集完全不变（零权限提升）。
- P1 / P2 / P3 各自独立可上线、可回滚。

**Non-Goals:**

- 不做插件运行时、不做自定义字段（见 proposal 的 Non-goals）。
- 不为 MCP 建立第二套业务校验——那是本设计要极力避免的。
- 不重构网关流水线本身。

## Decisions

### D1：自定义配置用 5 张专用表，而非塞进 `settings` KV

**选择**：`snippets` / `theme_tokens` / `nav_items` / `home_modules` / `custom_pages`。

**理由**：`settings` 是 `(key, value)` KV，塞 JSON 数组会同时失去三样东西——逐段启停（`enabled`）、逐段审计（`updated_by` / `updated_at`）、以及按段索引（比如「当前生效的 JS 有哪些」）。而这三样正是「允许任意 JS」场景的硬需求。

**替代方案**：单表 `customizations(kind, payload)` + JSON。更省表，但查询与校验都要在应用层做 JSON 解析，且无法给 `enabled` 建索引。放弃。

### D2：MCP 直调 service 层，但必须自带 `mcp.Enforcer`

**选择**：`/mcp` 的 handler → `internal/mcp` 的 tool adapter → `service.Xxx`，不自调用 HTTP。同时新增 `mcp.Enforcer`，在 `tools/call` 执行前完成四项检查：维护模式、ban/禁言、权限、限流。

**理由**：自调用 HTTP 要多一次网络往返、要自己构造 Bearer、要处理 401 刷新，且会把 service 层校验跑两遍。但直调 service 的代价是**丢掉整条网关链**——所以 `Enforcer` 不是可选项。它复用既有组件（`pkg.SettingReader` / `pkg.BannedChecker` / `pkg.PermissionChecker` / `ratelimit` 引擎），不新写判定逻辑。

**替代方案**：内部 HTTP 自调用。放弃，理由如上。

**这条决策对应的规范**：`layered-architecture` delta 的「网关保护面之外的端点必须自行完成等价检查」。

### D3：新增独立 `customize:*` 权限域，绝不并入 `admin`/`write`

**选择**：`customize:read` / `customize:write` / `customize:publish` 三个新权限码，登记进 `DefaultPermissions`；`apiKeyScopeRegistry` **不**把 `customize:*` 加进任何既有 scope。

**理由**：这是整个设计里唯一一个「做错就是安全漏洞」的地方。`apiKeyScopeRegistry["admin"]` 已经一把映射 13 个权限码；若把 `customize:write` 追加进去，**所有存量 admin scope 的 Key 会在部署瞬间获得向全站注入 JS 的能力**。这是权限提升，且难以察觉。因此必须显式建 scope、显式勾选。

**替代方案**：复用 `admin` scope。放弃，理由如上。

### D4：AI 写自定义内容分级生效（CSS/token 直接，JS 类强制草稿）

**选择**：
- CSS snippet、theme token → 持 `customize:write` 即生效。
- JS / HTML snippet、nav item、home module、custom page → 写入时强制 `enabled=false`（或 `status=draft`），需人工启用；持 `customize:publish` 才可直接生效。
- 响应体必须显式返回 `applied: true|false` 与原因。

**理由**：CSS 只能改样式，最坏情况是站点变丑（可撤销）；JS 等于对全部访客执行任意脚本，最坏情况是凭证泄漏 → 持久化 XSS。两者风险量级差一个数量级，用同一策略不合理。

**替代方案**：全部草稿（安全但 AI 改个主题色也要人点一下）、全部直生效（简单但爆炸半径过大）。两者都在提问中被否，采用分级。

### D5：`/mcp` 用 Streamable HTTP + JSON-RPC 2.0，不套论坛信封

**选择**：`POST /mcp`，请求与响应都是 JSON-RPC 2.0 消息；业务错误用 JSON-RPC error 表达，不套 `{code,msg,data}`。

**理由**：MCP 客户端按 JSON-RPC 解析，套信封会直接破坏互操作——这与 `api-style` 里 OAuth/OIDC 协议端点的既有例外完全同构。

**这条决策对应的规范**：`api-style` delta 里新增的 MCP 例外与 scenario。

### D6：P3 的 MCP 客户端「优先 MCP、回退 HTTP」

**选择**：`moderation_mode=external` 时，若 `mcp_servers` 里存在启用且承担审核职责的条目 → 走 MCP `tools/call`；否则或调用失败 → 回退既有 `CheckExternal` 的 HTTP 契约。

**理由**：`CheckExternal` 是既有生产路径，直接替换会让现网部署（配了 HTTP 审核服务但没配 MCP）静默失效。双轨 + 明确回退顺序是唯一不破坏兼容的做法。

### D7：前端「配置优先，无配置回退硬编码」

**选择**：`nav.tsx` 保留现有硬编码数组作为 fallback 常量；配置存在时用配置。

**理由**：`nav_items` 表初始为空。若改成「只读配置」，部署瞬间导航就空了。回退常量同时保证「配置被误删」时站点仍可用。

## Risks / Trade-offs

- **[token 泄漏 = 持久化 XSS]** → 独立 scope + JS 强制草稿 + 审计全覆盖 + 后台显示「当前生效的 JS」+ 文档建议该类 Key 设过期时间。
- **[`/mcp` 绕过网关导致封禁用户仍可写]** → `mcp.Enforcer` 四项检查，且必须有测试钉住（写一个「被封禁用户的 Key 调写工具必须失败」的用例）。这是本设计**必须**有回归测试的一条。
- **[在 `apiKeyScopeRegistry` 里误加 customize → 权限提升]** → 加一条测试：断言 `admin` / `write` / `read` scope 的权限集**不含**任何 `customize:*`。
- **[Iris party 中间件快照陷阱]** → 不改动 `router.go` 的 party 结构，`/mcp` 在 `Register` 里以 `app.Post("/mcp", ...)` 直接注册（与 `/.well-known/*` 同行位置）。`middleware_chain_test.go` 已是源码扫描式测试，可加断言。
- **[AI 反复写草稿造成噪音]** → 片段按 `id` upsert 而非每次新建；后台草稿区可批量丢弃。
- **[`nav.tsx` 改动影响首屏]** → 配置随 `loadPublicSettings()` 一并下发，不新增首屏请求。
- **[P1/P2/P3 顺序耦合]** → P2 的 tool 实现依赖 P1 的 service；因此 P1 必须先把 service 签约定稳，P2 只做适配层。若 P1 服务签名变动，P2 需同步——这是刻意接受的耦合，好过维护两套校验。

## Migration Plan

1. **P1**：新增 5 张表（`AutoMigrate` 追加即可，无破坏性变更）→ 新增 `customize:*` 权限码与 seed → service/handler/router → 后台 6 个页面 → 前端注入层与 `nav.tsx` 回退逻辑。全部为空配置时行为与现状完全一致，可安全上线。
2. **P2**：新增 `internal/mcp` 包与 `/mcp` 路由 → 工具注册表默认写工具关闭 → 后台工具开关页。上线后不开启任何写工具时，`/mcp` 仅暴露只读工具，风险面接近零。
3. **P3**：新增 `mcp_servers` 表与客户端 → `CheckExternal` 加分支（默认走旧路径）→ 后台登记页。

**回滚**：三阶段均为「新增表 + 新增端点 + 配置开关默认关闭」，回滚只需关掉对应设置项（`mcp_enabled` / 工具开关 / `customize` 相关注入开关），无需回退数据库迁移。`nav.tsx` 的硬编码 fallback 保证即使清空 `nav_items` 也不会白屏。

## Open Questions

- MCP 工具的分页与结果体积上限（是否要截断长帖子正文）：可先用固定上限（如正文 8KB），按实际调用体验再调，不影响规范与任务拆分。
- 自定义页是否要支持内嵌 HTML（除 Markdown 外）：当前 spec 只承诺 Markdown；若后续需要，作为 `site-customization` 的增量提案。
- 主题 token 覆盖是否需要「按版面差异化」：当前为全站单套；若要做，属于 P1.5 范围。
