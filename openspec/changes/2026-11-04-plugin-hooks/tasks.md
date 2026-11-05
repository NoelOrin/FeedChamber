# Tasks — 插件钩子

提案：[proposal.md](proposal.md)

> 顺序：1 → 2 可并行；3 依赖 1；4 依赖 3；5 依赖 1；6 最后。

## 0. 前置决策（已定）

- [x] 插件系统用户：**与后端同用户**（v1）→「不直连数据库」为约定而非强制；唯一可执行防线是环境变量白名单
- [x] `cron.daily`：**复用 `TrustService` 的 ticker**，不新建独立 ticker
- [x] `npm install`：**允许**，在插件目录内隔离执行
- [ ] 钩子超时默认值与是否按钩子可配（未定）

## 1. 事件广播（先做，纯增量）

- [x] `internal/pkg/eventbus/eventbus.go`：新增 `SubscribeBroadcast`（不带 queue group 的 core pub/sub）
- [x] 补测试：广播送达全部订阅者；现有队列订阅语义不变
- [ ] `internal/pkg/eventbus/AGENTS.md`（若有）更新说明两种订阅语义

## 2. 子进程宿主骨架

- [x] `internal/plugin/host.go`：`Host` 接口（Install/Start/Stop/Reload/Status）
- [x] `internal/plugin/supervisor.go`：`os/exec` 拉起 `node`，进程组管理与优雅终止
- [x] `internal/plugin/supervisor.go`：退避重启（1s→2s→4s→8s→16s，上限 5 次）
- [x] `internal/plugin/ipc.go`：stdio JSON-RPC 编解码（请求/响应/通知三种帧）
- [x] 环境变量白名单：`cmd.Env` 由白名单**重建**（MUST NOT 基于 `os.Environ()`），只下发 `NODE_ENV`、`PATH`、插件专属配置
- [x] 单测：断言子进程环境变量中不含 `DB_DSN` / `ADMIN_PASSWORD` / `*_SECRET` / `*_KEY`
- [x] 单测：进程崩溃后宿主存活、退避序列正确、超限停止

## 3. 钩子分发

- [ ] `internal/plugin/hooks.go`：钩子名常量表（9 个）
- [x] 钩子注册：读 `plugin.json` 的 `hooks` 字段
- [x] 分发：广播事件 → 本机 fan-out → 按插件声明过滤 → IPC 调用
- [x] 每钩子超时（默认值待定）+ 超时后终止该次调用
- [x] 失败策略：记日志 + 有限重试（指数退避，最多 3 次）
- [x] 上下文失效：钩子返回后 `ctx` 作废，后续调用被拒绝
- [x] `cron.daily`：复用 `TrustService` 的每日 ticker 触发，不新建 ticker
- [x] 单测：多插件收到同一事件、钩子失败不影响主流程、上下文失效

## 4. 宿主 API

- [x] `internal/plugin/api.go`：API 注册与 capability 校验
- [x] `posts.*`：`create` / `find` / `update` / `setStatus`（走 `PostService`）
- [x] `comments.*`：`create` / `find`（走 `ContentService`）
- [x] `users.*`：`get` / `findByEmail`（走 `UserService`）
- [x] `notifications.send`（走 `NotificationService`）
- [x] `kv.*`：`get` / `set`，前缀 `plugin:<id>:`
- [x] `slots.set` / `slots.delete`（写插槽条目）
- [x] `log.*`
- [x] 写类 API 全部落 `audit_logs`
- [x] 单测：未声明 capability 被拒、写操作留审计

## 5. 模型与生命周期

- [x] `internal/model/plugin.go`：扩 `Manifest` / `Capabilities` / `ProtocolVersion` / `SourceID` / `Status`
- [x] `internal/model/plugin_source.go`：来源白名单与审批状态
- [x] `internal/model/plugin_install.go`：安装记录与状态机
- [x] `internal/model/slot_entry.go`：插槽条目 + 枚举 + 5 种 Kind
- [x] `db/migrations/`：三张新表
- [x] `server/iris.go`：AutoMigrate 登记
- [x] `internal/container/container.go`：装配 `Host`
- [x] `internal/container/runtime.go`：`Start` 恢复 `running` 插件；`Shutdown` 优雅停止
- [x] 权限码：`plugin:read` / `plugin:manage`；`/plugins` Party 补 `RequirePermission`（当前为零）
- [x] `server/seed.go`：权限码 seed 与角色映射

## 6. 安装与来源审批

- [x] 来源白名单 CRUD + 审批流
- [x] 安装前能力清单与依赖清单展示
- [x] `npm install` 在插件目录内执行（`--prefix` 限定），失败回滚
- [x] 依赖安装前把依赖清单展示给管理员确认
- [x] 安装/审批/卸载全部落审计

## 7. 壳层插槽

### 后端

- [x] `internal/repository/slot.go`：接口 + SQL
- [x] `internal/service/slot.go`：CRUD、可见性过滤、审计
- [x] `internal/handler/slot.go`：管理端点 + 公开读取端点
- [x] `internal/router/`：管理路由挂权限码；公开路由

### 前端

- [x] `app/web/components/slot/slot-host.tsx`：读配置 + 可见性过滤 + 排序
- [x] `app/web/components/slot/renderers/`：`list` / `stats` / `markdown` + 注册表
- [x] `components/aside/sidebar-content.tsx`：拆 6 个插槽
- [x] `components/layout/nav.tsx`：拆 3 个插槽
- [x] `components/layout/footer.tsx`：拆 2 个插槽
- [ ] `app/web/app/admin/customize/slots/`：插槽配置页
- [x] `packages/api-client/src/services/slot.ts` + 类型 + 常量
- [ ] 测试：空配置回退、未知插槽忽略、等级可见性

## 8. CLI

- [x] `packages/plugin-cli/`：包骨架、`fbp` 入口、tsup 构建
- [x] `fbp init [--lang js]`：生成 `plugin.json` + `index.js` + 示例钩子
- [x] `fbp dev`：拉起本地 mock 宿主（与真宿主共用 IPC 编解码）
- [x] `fbp build`：打包插件目录
- [ ] 模板仓库

## 9. 文档

- [x] `docs/design/extension-points.md`：改写原则（从「不支持插件」到「支持子进程钩子插件」）
- [ ] 新增 `docs/design/plugin-hooks.md`：钩子清单、IPC 协议、生命周期
- [ ] 新增 `docs/design/plugin-security.md`：同机风险、npm 供应链、站长须知
- [x] `app/server/internal/plugin/AGENTS.md`
- [x] `app/web/components/slot/AGENTS.md`
- [x] `packages/plugin-cli/AGENTS.md`
- [x] 根 `AGENTS.md` 模块说明
- [x] `node scripts/check-agents-docs.mjs --write-index`

## 10. 验收

- [x] `openspec validate --all --strict`
- [x] `cd app/server && go build ./... && go test -race ./...`
- [ ] `pnpm check && pnpm build`
- [ ] `bash feedchamber_test.sh -all`
- [ ] 端到端：装一个示例插件 → 发帖 → 插件收到 `post.created` → 调宿主 API 写插槽 → 前台侧边栏显示
- [ ] 故障演练：`kill -9` 插件进程 → 宿主不受影响 → 自动重启 → 状态恢复

## 11. 后台管理闭环与分发（2026-11-05 追加，已完成）

- [x] `PluginRuntime` 窄接口 + `SetRuntime`/`HasRuntime`：Install→StartPlugin（失败回滚）、SetEnabled→Start/Stop、Delete/Uninstall→停进程+双表清理、ListInstalls
- [x] `HostOptions.APIDepsFor` 真实七接缝适配（`container/plugin_api_deps.go`）+ `hostRuntime`（固定 `context.Background()`，避免请求 ctx 杀子进程）
- [x] `/{id}/status` 权限码 `plugin:read`→`plugin:manage`（只读凭证此前可停全站插件）
- [x] 审计记录真实操作人（`InstallAs`/`UninstallAs`/`SetEnabledAs`/`UpdateAs`/`DeleteAs`）
- [x] `Update` 与 `enabled` 解耦（元数据编辑不再静默禁用插件）
- [x] `PluginFetcher`：`git clone --depth 1 --single-branch` + `npm install --omit=dev --ignore-scripts`，带超时、失败清理目录
- [x] `ValidateSourceURL`：只放行 http/https/git/git@，拒绝 `file://`、壳元字符、`-` 开头
- [x] `HashPluginDir` 内容校验和（排除 node_modules/.git），落 `plugin_installs.checksum`
- [x] `POST /admin/plugins/preview`：能力/依赖清单预览，不落库、不起进程
- [x] `plugin_versions` 表 + `ListVersions` / `RollbackAs`（`POST /admin/plugins/versions/list`、`/{id}/rollback`）
- [x] 前端 `/admin/plugins` 支持来源审批、manifest 安装、预览、运行状态、卸载、版本历史与回滚
- [x] `make swag` 重生成（插件端点 5 → 14）

## 12. 仍未交付（明确遗留，非本次范围）

- [ ] 包签名验签（proposal 为 SHOULD）
- [ ] 插件市场 / 一键安装（Non-goal，刻意不做）
- [ ] 同系统用户强制隔离（Non-goal，仅 env 白名单为可执行防线）
- [ ] `docs/design/plugin-hooks.md` / `plugin-security.md` 两份设计文档
- [ ] 仓库内示例插件 + 端到端「装插件→发帖→收到钩子→写插槽」验收
- [ ] `kill -9` 故障演练验收
- [ ] `pnpm check && pnpm build` 全仓门禁（web 侧存在与本次无关的 `nav.tsx` import 顺序既有失败）
- [ ] `bash feedchamber_test.sh -all` 全量 API 测试
