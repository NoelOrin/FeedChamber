# Dashboard 全量重建：以 admin-template 为新基座

- 日期：2026-10-15
- 状态：已批准（头脑风暴六节均经确认）
- 分支：从 `migrate/react-vinext` 切出 `rebuild/dashboard-template`
- 参照物：`/Users/noelorin/template/apps/web`（下称「模板」）、现有 `app/dashboard`（下称「旧 dashboard」）

## 1. 背景与目标

旧 dashboard 是 React 19 + antd 6 + pro-components + TanStack Router 手写 `router.tsx` 的管理后台。本次以模板为基座**全量重建**，目的：

1. 换掉 pro-layout 壳与手写路由，采用模板的文件路由 + 路由生成菜单 + 自研布局；
2. 统一数据获取模式（现状 39 处手写 `useEffect`+`useState`，Query 挂载但零使用）；
3. 视觉对齐前台「Offset Litho / 档案索引」语言，而非模板默认观感。

**明确不做**：按钮级权限码逐个铺设（机制接通即可，现状页面本就没有）；新增 e2e 套件；图表库引入；mobile/PWA。

## 2. 决策记录

| 决策点 | 结论 | 被否选项及原因 |
|---|---|---|
| 重建形态 | 并行新包 `app/dashboard-v2`，迁完删旧目录并改回 `app/dashboard` | 原地重建（与另一会话的未提交改动冲突） |
| 数据层 | 继续用 `@feedchamber/api-client` | 移植模板 api 层（与 web 端分裂、违背「修接缝不加层」） |
| 迁移节奏 | 一次性全量迁完 | 分批（用户明确选择全量） |
| 视觉 | 对齐前台 Offset Litho 风格 | 直接用模板观感 |
| pro-components | 不带入新包，难页重写为模板 BaseTable | 平移 ProTable/ProDescriptions（两套表格模式并存） |
| URL 形态 | 保持不变（含 `?id=` search 参数的 detail 路由） | 路径参数化 `$id`（会破坏现有链接与肌肉记忆） |

## 3. 架构与落地形态

### 3.1 新包构成

- 复制模板：`src/`、`vite.config.ts`、`tsr.config.json`、biome/vitest 配置，纳入 pnpm workspace。
- **删除**：`src/api`（operations/contracts/legacy/services/adapters 主体）、`src/tenant`、`tokenRefreshUtils`、`pages/(app)/examples`、dev mockBackend 业务部分（`/api` proxy 保留指向 `:8080`）。
- **保留**：layouts、`useMenu` 路由生成菜单、errorStore/settingStore/routeStore、utils/permission、baseTable、design-tokens 骨架、test 基建。
- **重写**三处耦合点：
  - `store/userStore.ts` → api-client 投影（login / getMe / 权限码列表 / 登出）；
  - `auth/bootstrapAuthRuntime.ts` → 去 api/tenant 依赖；
  - `auth/providers/credentialsDriver.ts` → 接 api-client 登录接口。
- 依赖：`@feedchamber/api-client`、`@feedchamber/tokens`、`@feedchamber/shared-utils`。不带入 pro-components、axios、echarts、xlsx、framer-motion 等未用依赖（用到再加）。

### 3.2 与并行 agent 的隔离

改动面 = 新增 `app/dashboard-v2/` 目录 + 收尾阶段的脚本改名，与 `app/dashboard` 的未提交改动零交集。不新开分支之外的协调动作。

## 4. 认证与权限

- 凭证沿用服务端会话 sid：登录 → api-client `authService`；api-client authFetch 自带 401 刷新（sid 单用轮换契约，Bearer 客户端必须采纳返回的 `access_token`）。模板的 axios transport / BroadcastChannel 多 tab 刷新机制整条删除，不重复实现。
- 路由守卫在 `(app)/route.tsx` 的 `beforeLoad` 复刻旧 dashboard 三段逻辑：
  1. 无 token → `/login`；
  2. `mustChangePassword` → `/first-change-password`（新页面，加入模板公开路径白名单 `isPublicAuthPath`）；
  3. role 非 admin/moderator → 登出回登录页。
- 不复用模板 AuthGuard 的 authorization 引擎做页面守卫；模板 `PermissionGate` 机制保留备用。
- `login`（150 行）、`first-change-password`（118 行）按旧 dashboard 行为重写，无 pro 依赖，平移成本低。

## 5. 数据层

- QueryClient 挂载沿用模板；业务请求全部经 api-client services。
- query key 约定 `[resource, params]`；`staleTime: 60s`（沿 dashboard 约定，区别于 web 的 10s）。
- 列表页统一模板 `baseTable`：分页/筛选/表头持久化走 baseTable + tableStore；tableStore 的 `#/api` 类型引用改为 api-client 类型；ProTable 的 valueEnum/status 映射改写为普通 render 函数。
- mutations 用 `useMutation` + 失效对应 query key；操作反馈走模板 sonner。
- detail 路由保留 `?id=` search 参数（`validateSearch`）。

## 6. 视觉主题（Offset Litho 对齐）

- 改造锁定 design-tokens 三文件：`token/base.ts`（圆角/间距/字体）、`token/light.ts`、`token/dark.ts`，加 `antdTheme.ts` 组件级微调。骨架 antd-style `createStyles` 自动跟随。
- 设计值来源：`app/web/app/globals.css` archive 体系（Eyebrow/IndexNum/Rule/DisplayTitle 的字号/字重/间距）+ `@feedchamber/tokens/tokens.css` CSS 变量。`@feedchamber/tokens` 包继续引入作 CSS 层，antd ThemeConfig 与之取同值，保证双轨一致；该包同时被 web 消费，不删不并。
- 主题切换沿用模板 settingStore（persist + 跨 tab 同步），保持 light/dark/system 三态。

## 7. 页面迁移映射

菜单 4 分组结构沿用旧 ProShell，落到文件路由 `staticData`（title/icon/order/hideInMenu），由 useMenu 自动生成菜单。

三档执行：

| 档 | 页面 | 做法 |
|---|---|---|
| 易（约 20 页） | dashboard 首页、tags、groups、invites、webhooks、api-keys、backups、oauth-clients、plugins、security、sessions、trash、audit-logs、search、login、first-change-password、settings/modules 等 | 平移 + 手写 fetch 机械替换为 useQuery |
| 中（约 8 页） | users、users/detail、reports、reports/detail、spaces、spaces/permissions、moderation、settings | 手写分页/批量操作改 baseTable + useMutation；settings 的 `settingsConfig.ts` 声明式体系原样保留，tab 走 `?tab=` search 参数 |
| 难（约 7 页） | posts、posts/detail、comments、comments/detail、banned、announcement、roles/detail | ProTable → baseTable；ProDescriptions（39 处）→ antd Descriptions 轻封装；roles/detail（348 行权限矩阵）保留原逻辑仅换容器 |

顺序：骨架跑通（登录→首页→菜单）→ 易档 → 中档 → 难档 → 删旧目录改名收尾。

## 8. 工程与验收

- 工具链：模板 vite 8 + biome 2 + vitest；`pnpm dev:dashboard`、Dockerfile、nginx 配置收尾时切到新包。
- 验证门（每档迁移后）：`tsc --noEmit` + `biome check` + `vitest run` + 构建通过。
- UI 验收：全部路由 × light/dark 截图，汇总为单个自包含 HTML gallery；浮层用真实点击关闭后截图。
- 行为基准：以旧 dashboard 页面为准逐页对照。
- **完成定义**：30 条路由全部可用；旧 `app/dashboard` 删除；包名改回 `@feedchamber/dashboard`；`pnpm dev:dashboard`/Dockerfile/nginx 指向新包；验证门全绿；截图 gallery 交付。

## 9. 风险

- **并行 agent**：仓库有另一会话在写 `app/dashboard` 与后端，本设计对旧目录零写入；若收尾删目录时对方仍有未提交改动，删除动作须再次确认。
- **模板认证层的耦合深度**：`auth` 目录是骨架中轴线，重写 driver 时若牵出隐藏耦合（authProjection、AuthProvider），以「保骨架、换数据源」为原则，不为对齐模板原实现而保留多余机制。
- **query 化的手写请求改写**：39 处机械替换可能引入时序差异（如重复提交防抖缺失），易档迁移时逐页对照旧行为。
