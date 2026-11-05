# 插件后台管理闭环修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:test-driven-development for every task. Steps use `- [ ]` syntax.

**Goal:** 让管理员从后台装/启/停/删插件真正驱动 v2 子进程运行时，并把前台对齐 v2 契约。

**Architecture:** 后端把 `*plugin.Host` 经一个窄接口注入 `PluginService`，让 Install/SetEnabled/Delete/Uninstall 在落库后调用 StartPlugin/StopPlugin；Install 成功后置 `running` 使重启可恢复；container 补 `APIDepsFor` 真实 service 适配；`/{id}/status` 权限码改 `plugin:manage`。前端 api-client 补来源审批方法/类型/权限码，页面按 v2 重做安装表单与来源管理，修「编辑误禁用」与 error 态。

**Tech Stack:** Go 1.x / Iris / GORM / testify；React 19 / TanStack Query / zod / @feedchamber/api-client / biome。

**Spec:** `openspec/changes/2026-11-04-plugin-hooks/`（proposal + specs/plugin-hooks）；本计划只做「后台管理最小闭环」，不做包签名/npm install/插件市场（Non-goals）。

## 执行记录（2026-11-05，已全部完成并验证）

- Task 1（`2ba791a`）：`PluginRuntime` 窄接口 + `SetRuntime`/`HasRuntime`；Install→StartPlugin+rollback、SetEnabled→Start/Stop、Delete/Uninstall→Stop+双表清理+审计、ListInstalls。6 个 TDD 测试 `plugin_lifecycle_test.go` 红→绿。
- Task 2（`6f95896`）：`plugin_api_deps.go` 七个接缝适配器 + `hostRuntime`（固定 `context.Background()`）；`Host.HasAPIDeps()`；`ContentService.SetStatusForPlugin`（2 个 TDD 测试）。`plugin_wiring_test.go` 装配断言红→绿。
- 前端（`6b38b40`、`b8295a6`）：api-client v2 契约（27/27 测试）、`/admin/plugins` 页面重写（来源审批 / manifest 安装 / 运行状态 / 卸载 / isError）。
- Task 3（`4fd466a`）：`/{id}/status` 权限改 `plugin:manage`，新增 `installs/list`、`{id}/uninstall`；网关 + 路由 + 回归测试三层同步。
- Task 6（`737caa7`）：`make swag` 重生成（插件端点 5→11）；`go build ./...` + `go test -race ./...` 全绿；`pnpm --filter @feedchamber/api-client test/check/build` + `pnpm --filter @feedchamber/web typecheck` 全绿；`make swag-check`、CI `check-agents-docs --base HEAD` 通过；6 个 AGENTS.md + 5 个根/docs 文档同步。

**与计划的两处偏差（如实记录）**：① 未做 `/{id}` 详情端点——网关 `/admin/plugins/` 前缀会把 `/{id}` 一律判成 `plugin:manage`，只读详情会权限错配；改由 `installs/list` + `list` 在前端合并展示。② `ContentService` 新增了 `SetStatusForPlugin`（计划未列），因为 `PluginHost` 的 `comments.setStatus` 宿主 API 需要一个经 service 层的写入口，不能给插件直连 repository 的后门。

**仍未做（明确属于后续提案，非本次最小闭环）**：包签名验签、git clone / npm install 拉代码与依赖清单确认、升级/回滚/版本历史、插件市场、同系统用户隔离、审计 actor 精确化（install/uninstall 仍记 `"system"`）、`fbp dev --site` 连真站点、OpenSpec change 归档与 tasks.md 勾选。

## Global Constraints
- 硬规则：写操作 `POST` + 动词后缀；统一 `pkg.Success/Fail/HandleError` 信封；禁止裸 `ctx.JSON`。
- 权限码单一命名空间 `internal/model/permission.go`；网关 `path→permission` 与路由 `RequirePermission` 必须一致。
- `container.New` 只构造对象图、不拉起进程；`Start` 才订阅/恢复。
- 插件不直连 DB；宿主 API 经 service 适配（接缝接口在 `internal/plugin/api.go`）。
- 新增/修改的 Go 行为必须先有失败测试（TDD red→green）。
- 前端列表页错误态：`useQuery` 保留 `isError`，`Alert variant="destructive"` 显示 `query.error.message`（`app/web/app/admin/AGENTS.md:88-90`）。
- 不改动与本任务无关的文件；`go build ./...` + `go test -race ./...` + `pnpm check` 必须干净。

---

## Task 1（TDD）：Service 生命周期接线（后端核心）

**Files:**
- Modify: `app/server/internal/service/plugin_iface.go`（新增 `PluginRuntime` 窄接口、`SetRuntime`）
- Modify: `app/server/internal/service/plugin.go`（`Install`/`SetEnabled`/`Delete`/`Uninstall` 调 runtime；Install 后置 running；审计 actor）
- Test: `app/server/internal/service/plugin_lifecycle_test.go`（新建）
- Modify: `app/server/internal/service/plugin_install_test.go`（若 `Install` 断言 `installed` 需同步为 `running`）

**Interfaces:**
- Consumes: `plugin.Host.StartPlugin/StopPlugin/Running`（经窄接口，避免 service 依赖具体包）
- Produces:
  ```go
  // service/plugin_iface.go
  type PluginRuntime interface {
      StartPlugin(ctx context.Context, pluginID string, m *plugin.Manifest, caps plugin.CapabilitySet) error
      StopPlugin(pluginID string) error
      Running(pluginID string) bool
  }
  // PluginService 增：SetRuntime(rt PluginRuntime)
  ```
  `Install` 成功返回 `*model.PluginInstall{Status: running}`；`SetEnabled(false)` 调 Stop、`SetEnabled(true)` 调 Start（用存储的 manifest/caps 复原）；`Delete`/`Uninstall` 先 Stop 再删双表。

- [ ] **Step 1 写失败测试**：`plugin_lifecycle_test.go` 用 fake `PluginRuntime` 记录调用次数，断言：
  - `TestInstallStartsRuntime`：合法安装后 `fake.started==1` 且返回 `Status==running`；
  - `TestSetEnabledFalseStopsRuntime` / `TestSetEnabledTrueRestartsRuntime`；
  - `TestUninstallStopsRuntimeAndCleansBothTables`；
  - `TestDeleteStopsRuntime`（并断言 `plugin_installs` 无孤儿）。
  每个测试用 `newPluginTestSvc(t)` 真实 SQLite 铺底。
- [ ] **Step 2 验证红**：`cd app/server && go test ./internal/service/ -run 'TestInstallStarts|TestSetEnabled|TestUninstallStops|TestDeleteStops' -v` → FAIL（方法/行为不存在）。
- [ ] **Step 3 最小实现**：加 `PluginRuntime` 接口 + `SetRuntime`；在四处落库动作后调 runtime；Install 末尾把 install.Status 写 `running`（upsert 前设置）；`s.recordAudit` 的 install/uninstall actor 改为可传入（新增 `Install(ctx, in, actorID)`? —— 不改公共签名，改为在现有 `recordAudit(ctx, currentActor...)`：用 `context` 里取不到 user，故把 actor 作为 `InstallInput` 可选字段或从 handler 透传；**决策：给 `PluginInstallInput`/`Uninstall` 增加非导出参数会破坏接口，采用 `recordAudit` 用 `in.Dir` 无关，actor 仍 `"system"` 但补 `detail` 里带 plugin_id——本任务不改审计 actor，留 Task 5**）。
- [ ] **Step 4 验证绿**：同命令 → PASS；`go test ./internal/service/...` 全绿。
- [ ] **Step 5 Commit**：`git add ... && git commit -m "feat(plugin): wire admin actions to runtime lifecycle"`

## Task 2（TDD）：container 装配真实 runtime + APIDeps

**Files:**
- Modify: `app/server/internal/container/container.go`（`c.PluginSvc.SetRuntime(c.PluginHost)`；`HostOptions.APIDepsFor`）
- Create: `app/server/internal/container/plugin_api_deps.go`（7 个接缝适配器，包装已有 service）
- Test: `app/server/internal/container/plugin_wiring_test.go`（新建，或扩 `container_test.go`）

**Interfaces:**
- Consumes: Task 1 的 `SetRuntime`；`internal/plugin.APIDeps{Posts,Comments,Users,Notifications,KV,Slots,Audit}` 接缝；`c.PostSvc`/`c.ContentSvc`/`c.UserSvc`/`c.NotificationSvc`/`c.KV`/`c.SlotSvc`/`c.AuditLogSvc`。
- Produces: `pluginAPIDeps(pluginID, caps)` 返回 `plugin.APIDeps`，每个字段非空且落到对应 service；`RecordPluginAction` 用 `AuditLogSvc.Record`。

- [ ] **Step 1 写失败测试**：构造 container（复用 `container_test.go` 的 `New(Deps{...})`），断言 `c.PluginHost.opts.APIDepsFor != nil`——需导出一个 `Host.APIDepsConfigured() bool` 或让测试通过 `StartPlugin` 间接验证。**决策：在 `internal/plugin/host.go` 加 `func (h *Host) HasAPIDeps() bool`，测试断言 `c.PluginHost.HasAPIDeps()` 为 true；并断言 `c.PluginSvc` 已 SetRuntime（通过导出 `PluginService` 的 `RuntimeConfigured()` 或行为测试）。**
- [ ] **Step 2 验证红**：`go test ./internal/container/ -run TestPluginWiring -v` → FAIL。
- [ ] **Step 3 最小实现**：适配器文件（每个接缝一个 struct，方法签名对齐 `api.go` 的接口；`Set` 带 TTL 用 0/持久；`Slots.Set` 映射到 `SlotService` 的实际方法——若签名不符则写最小转换）；container 里 `NewHostWithOptions` 补 `APIDepsFor: pluginAPIDeps(...)`，并在构造 PluginSvc 后 `c.PluginSvc.SetRuntime(c.PluginHost)`。
- [ ] **Step 4 验证绿**：`go test ./internal/container/... ./internal/plugin/...` 全绿。
- [ ] **Step 5 Commit**。

## Task 3（TDD）：权限码 + 卸载/详情/运行状态端点

**Files:**
- Modify: `app/server/internal/router/routes_admin.go`（`/{id}/status` 改 `PermPluginManage`；新增 `/{id}/uninstall`、`/{id}` 详情、`/installs/list`）
- Modify: `app/server/internal/gateway/permission.go`（status pattern 改 `PermPluginManage`）
- Modify: `app/server/internal/handler/plugin.go`（`Uninstall`/`Detail`/`ListInstalls`，swagger 注解）
- Modify: `app/server/internal/service/plugin_iface.go` + `plugin.go`（`Get`/`ListInstalls`，返回运行状态）
- Test: `app/server/internal/router/middleware_chain_test.go`（断言 status=manage）、`app/server/internal/handler/plugin_test.go`（新建，测 handler 返回结构）

- [ ] **Step 1 写失败测试**：扩展 `middleware_chain_test.go` 的 `/{id}/status` 行断言为 `PermPluginManage`；新增 `TestAdminPluginRoutes` 断言 uninstall/detail/installs 路由存在。
- [ ] **Step 2 验证红**：`go test ./internal/router/ -run TestAdminPlugin -v` → FAIL。
- [ ] **Step 3 最小实现**：改权限码 + 加 3 个 handler/路由/service 方法；`pkg.Success` 统一信封。
- [ ] **Step 4 验证绿**：`go test ./internal/router/... ./internal/handler/... ./internal/gateway/...` 全绿。
- [ ] **Step 5 Commit**。

## Task 4（TDD）：前端 api-client v2 契约

**Files:**
- Modify: `packages/api-client/src/services/plugin.ts`（补 `PluginSource`/`PluginInstall` 类型 + `listSources/createSource/approveSource/rejectSource/uninstall/detail/listInstalls`；`install` 入参改 `PluginInstallInput`；`update` 带上 `enabled`）
- Modify: `packages/api-client/src/types.ts`（把 Plugin 等类型搬过去，v2 字段）
- Modify: `packages/api-client/src/constants.ts`（`PermissionCode.PLUGIN_MANAGE`/`PLUGIN_READ`）
- Modify: `packages/api-client/src/index.ts`（导出）
- Test: `packages/api-client/src/services/plugin.test.ts`（新建，mock `post`，断言 URL/method/body）

- [ ] **Step 1 写失败测试**：断言 `pluginService.listSources()` 调 `post('/admin/plugins/sources/list', {})`、`install({plugin_id,source_id,manifest})` body 正确、`uninstall(id)` 路径、`PermissionCode.PLUGIN_READ==='plugin:read'`。
- [ ] **Step 2 验证红**：`pnpm --filter @feedchamber/api-client test`（或其 vitest 命令）→ FAIL。
- [ ] **Step 3 最小实现**：补齐类型/常量/方法。
- [ ] **Step 4 验证绿**：测试全绿 + `pnpm --filter @feedchamber/api-client typecheck`。
- [ ] **Step 5 Commit**。

## Task 5（TDD）：`/admin/plugins` 页面按 v2 重做

**Files:**
- Modify: `app/web/app/admin/plugins/page.tsx`（安装表单改 manifest+source 选择；来源审批 Tab/弹窗；运行状态列；编辑带 enabled；`isError` 分支；删除改调 uninstall）
- Modify: `app/web/lib/admin/auth/route-access.ts` / `access.ts`（`plugin:read` 只读映射）
- Test: 无前端单测框架；用 `pnpm check` + `pnpm --filter @feedchamber/web typecheck` 作验证；把「编辑误禁用」用后端测试覆盖（Task 1 已含 `Update` 保留 enabled 的断言）。

- [ ] **Step 1**：先写后端测试 `TestPluginService_UpdateKeepsEnabledWhenOmitted`（改 service `Update`：empty input 时保留原 Enabled，或前端显式带上——**决策：前端显式带 enabled + 后端 Update 对零值 false 不做覆盖，仅当 input 含字段时更新；用 `PluginInput.Enabled *bool` 会破坏接口，采用前端传 `enabled: editing.enabled` 修复**）。
- [ ] **Step 2 验证红**：后端测试红 → 前端改动不可测，故本任务红在后端断言「update 请求带 enabled」由 api-client 测试覆盖。
- [ ] **Step 3 最小实现**：页面重构（保持 326 行量级，拆分来源子组件）；补 `isError` Alert；删除确认改「卸载」并调 uninstall。
- [ ] **Step 4 验证绿**：`pnpm --filter @feedchamber/web typecheck` + `pnpm check` 干净。
- [ ] **Step 5 Commit**。

## Task 6：收口与验证
- [ ] `cd app/server && go build ./... && go test -race ./...`
- [ ] `make swag` 重生成 `openapi.json`/`docs/`（补 4 条 sources + 新端点），`make swag-check`
- [ ] `pnpm check && pnpm build:web`
- [ ] 更新根 `AGENTS.md:322`、`docs/backend-overview.md:209/496` 的过时「无执行扩展点」表述；`docs/design/extension-points.md:7` 原则改写（tasks.md 要求）
- [ ] 勾选 `openspec/changes/2026-11-04-plugin-hooks/tasks.md` 中本次真正完成项（仅勾完成项，不虚报）
- [ ] `git status` 干净 + 最终 diff review

## Self-Review
- 覆盖上一轮缺口：Install→Start ✓、SetEnabled→Start/Stop ✓、Delete/Uninstall→Stop+双表 ✓、APIDepsFor ✓、status 权限 ✓、前端契约 ✓、编辑误禁用 ✓、error 态 ✓、卸载路由 ✓、运行状态可见 ✓。
- 未覆盖（明确留作后续，非本次最小闭环）：包签名验签、npm install/依赖清单、git clone 拉代码、升级/回滚、插件市场、同用户隔离、审计 actor 精确化、`fbp dev --site`。
- 类型一致性：`PluginRuntime.StartPlugin` 签名与 `plugin.Host.StartPlugin` 完全一致；`APIDeps` 字段与 `api.go` 接缝一致。
