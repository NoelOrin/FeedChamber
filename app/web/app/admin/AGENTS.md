# 管理后台（`app/admin/`）

内嵌在主站里的 `/admin/**` 路由段。**不是独立应用**：与前台同一个构建、同一个 origin、同一套部署、同一套 shadcn 原语（antd / Pro Components 已全部移除）。

> 上级：[app/web/AGENTS.md](../AGENTS.md) · 路由总览：[app/AGENTS.md](../AGENTS.md) · 壳层组件：[components/admin](../../components/admin/AGENTS.md) · 准入逻辑：[lib/admin](../../lib/AGENTS.md)

实测规模：41 个路由文件（页面 + `layout.tsx` + `admin.css` + `error.tsx` + `not-found.tsx` + `[...rest]` 兜底）。

## 纯 CSR：服务端只吐骨架

`app/admin/layout.tsx` 只做两件服务端能做的事：声明 noindex 元信息、引入 `admin.css`。真正的判定与渲染全在客户端：

```
app/admin/layout.tsx          →  <AdminShell>（'use client'）
  └─ components/admin/admin-shell.tsx      准入 + 壳层 + 菜单
      └─ components/admin/admin-root.tsx   客户端挂载门（挂载前只渲 BootSplash）
```

**页面组件一律 `'use client'`。** 不要给后台页面加 `generateMetadata` 之外的 server 逻辑，也不要引入只在服务端可用的模块。

## 准入链（三道，顺序不可换）

| 道 | 位置 | 语义 |
|----|------|------|
| 1. 结构性私有 | `../proxy.ts` | `/admin` 不在 `PROTECTED` 里——未登录由第 2 道处理，**不要在 proxy 加** |
| 2. 进后台 | `lib/admin/auth/access.ts` | 拉 `/user/profile`（不信任 localStorage 的角色投影）→ 角色 ∈ `DASHBOARD_ROLES`（admin/moderator）或命中 `DASHBOARD_PERMISSION_CODES` 任一 → 放行。未登录跳 `/login?redirect=/admin/...`；已登录但无资格 → **原路返回**；无回跳目标才整页 403 |
| 3. 模块级 | `lib/admin/auth/route-access.ts` | 路径前缀 → `anyOf` 权限码（**数组语义是 anyOf**）。菜单过滤与壳层内 403 共用同一张表；页面内用 `useCan([...])` |

第 3 道只是 UX 门控（隐藏菜单、拦下必然 403 的页面）。**真正的安全边界在后端网关**（`app/server/internal/gateway/permission.go`）——权限码与那边 `/api/v1/**` 的映射对齐，取的是该页面主数据读接口所需的权限。

## 新增一个后台模块（四步，缺一步就出问题）

1. `app/web/app/admin/<route>/page.tsx` — 新建目录即新增 `/admin/<route>` 路由
2. `components/admin/layout/admin-shell.tsx` — 在 `menuItems` 里登记菜单项（`path` / `name` / `icon`）；详情页加 `hideInMenu: true`
3. `lib/admin/auth/route-access.ts` — 在 `ROUTE_PERMISSIONS` 登记权限码；**详情页必须排在列表页之前**（顺序即优先级，`/admin` 只做精确匹配）。`/admin/levels` 与 `/admin/xp-rules` 登记为 `level:manage`、`/admin/gates` 登记为 `gate:manage`（后端 `routes_trust.go` / `routes_gate.go` 同一门）
4. 取数用 `@feedchamber/api-client` 的 service + `useQuery`（key 取 `~/lib/query/keys.ts` 的 `qk`），表格用 `components/ui/data-table.tsx` 的 `DataTable`，详情用 `components/ui/descriptions.tsx`

> 注：根 `AGENTS.md` 曾写「菜单在 `components/admin/layout/pro-shell.tsx` 登记」——该文件不存在，实际是 `admin-shell.tsx` 的 `menuItems`。以本文件为准。

## 路由分组（菜单结构）

| 分组 | 路由 |
|------|------|
| 首页 | `/admin`（运营概览） |
| 论坛管理 | `/admin/banned`、`/admin/posts`（+`/detail?id=`）、`/admin/comments`（+`/detail?id=`）、`/admin/tags`、`/admin/trash` |
| 运营管理 | `/admin/announcements`、`/admin/reports`（+`/detail?id=`）、`/admin/moderation`（待审队列 + 顶部「文章审核模式」四选一卡片，写系统设置 `moderation_mode`，与「设置 → 内容」同一个 key）、`/admin/spaces`（含版面权限弹窗 + 可见性策略 public/login/level + min_level）、`/admin/search`（`hideInMenu`） |
| 用户与权限 | `/admin/users`（+`/detail?id=`，含「成长」页：XP / 等级 / 保级指标 / 指标分解 / 人工调整）、`/admin/invites`、`/admin/roles`（+`/detail?id=` 权限矩阵）、`/admin/groups`、`/admin/levels`（等级定义）、`/admin/xp-rules`（XP 规则 + 连续签到阶梯）、`/admin/gates`（门控规则 + 域默认动作 + 以用户试算） |
| 系统设置 | `/admin/api-keys`、`/admin/webhooks`、`/admin/backups`、`/admin/oauth-clients`、`/admin/plugins`、`/admin/security`、`/admin/sessions`、`/admin/settings?tab=`、`/admin/audit-logs` |
| 认证 | `/admin/first-change-password`（首改密在后台，登录复用主站 `/login`） |

`/admin/settings` 的 `?tab=` 是硬契约：`/admin/settings/dashboard` 重定向到 `?tab=dashboard`；Tab 定义与设置项清单在 `app/admin/settings/settingsConfig.ts`（`serviceTabs` / `SettingsGroup`）。

## 详情页的固定形状

详情页不是独立路由段，而是 `?id=` 查询参数（`/admin/posts/detail?id=`、`/admin/users/detail?id=` …）。原因：这些页面共享列表的权限码与菜单项，`hideInMenu: true` 让它们不进菜单，但 `route-access.ts` 仍要单独登记前缀。

## 复用优先

主题 / 语言 / 站点名 / 设置直接用主站的 `lib/theme.ts`、`lib/i18n.ts`、`lib/settings.ts`（`useTheme` / `useLocale` / `useSiteName`）。**后台不自留第二套 hook**；后台自研的只有权限相关（`lib/admin/auth/*` + `components/admin/access/*`）。

`proxy.ts` 会透传 `x-pathname`；根布局与 `generateMetadata` 检测到 `/admin/**` 时**跳过** spaces/tags 的外壳取数，`app/admin/admin.css` 只打进 `/admin` 的 CSS 分块，不污染前台。这条旁路是性能约定，新增后台页面不要绕过它。

## 改这里要注意

- 新增路由忘了登记 `route-access.ts` → 页面在壳层内被判 403；忘了登记菜单 → 用户找不到入口。**两处必须同时改。**
- 权限码写错不会编译报错（是字符串），对照 `packages/api-client/src/constants.ts` 的 `PermissionCode` 与后端 `initRouteMap`。
- 不要在页面里读 `window.location` 拼跳转：用 `lib/admin/navigation.ts` 的 `useNavigate` / `useLocation` / 查询参数 hook。
- 删除确认用 `components/ui/confirm-dialog.tsx` 的 `useConfirm`，不要 `window.confirm`。
- 后台页面不允许被搜索引擎索引：`admin/layout.tsx` 的 metadata 已声明 `robots: { index: false, follow: false }`，新增路由段不要覆盖它。
- **状态筛选的「全部状态」哨兵统一取 `@feedchamber/api-client` 的 `STATUS_FILTER_ALL`（`'__all__'`）**，不要在页面里各自定义 `const ALL`（`/admin/posts` 与 `/admin/reports` 共用）。后端只对 `report:manage` 持有者把该值当「不按状态过滤」，所以 **pending 帖只会出现在 `/admin/posts` 的「全部状态」下**——存量待审帖靠该列表操作列的「通过 / 拒绝」（`postService.setStatus` → `POST /posts/{id}/status`）救回，`/admin/posts` 因此是该状态唯一的运营入口。

## 本文件何时更新

- 新增/删除/改名后台路由
- 准入链（`DASHBOARD_ROLES` / `DASHBOARD_PERMISSION_CODES` / `ROUTE_PERMISSIONS`）语义变化
- 菜单结构或分组调整
- 后台壳层或挂载门机制变化
