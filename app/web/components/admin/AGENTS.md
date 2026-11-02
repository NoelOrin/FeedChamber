# 后台壳层与准入组件（`components/admin/`）

`/admin/**` 的客户端壳层。**后台不走 SSR**：这里的一切只在浏览器里跑。

> 上级：[components/AGENTS.md](../AGENTS.md) · 路由：[app/admin](../../app/admin/AGENTS.md) · 准入逻辑：[lib/admin](../../lib/AGENTS.md)

实测规模：10 个文件 / 4 个子目录。

## 文件表

| 文件 | 职责 |
|------|------|
| `admin-root.tsx` | **客户端挂载门**：服务端与首次水合只渲 `BootSplash`，`useEffect` 里 `captureReturnTarget()`（记下无权限时的回跳目标）后置 `mounted=true`。整段 `/admin` 靠它挡在 SSR 之外 |
| `admin-shell.tsx` | **分区总壳**：准入判定（`useDashboardAccess`）、未登录跳主站登录、无资格原路返回/403、`AccessProvider` 下发权限码、模块级 403（`ModuleForbidden`）。`AdminLayout` 只包这一层 |
| `layout/admin-shell.tsx` | **视觉壳层**：顶栏 + 侧栏菜单（`menuItems`）、菜单过滤（`requiredPermissionsFor`）、折叠状态持久化（localStorage `menu_open_keys`）、主题/语言切换 |
| `access/access-context.tsx` | `AccessProvider` + `useCan(anyOf?)` + 原始权限码读取 |
| `access/access-denied.tsx` | 整页 403（拿不到回跳目标时） |
| `access/module-forbidden.tsx` | 壳层内模块级 403 |
| `error/boundaries.tsx` | 后台错误边界 |
| `error/ErrorFallback.tsx` | 后台错误回退 UI（PascalCase 命名是历史遗留，新文件跟 kebab-case） |
| `error/index.ts` | 边界与回退的 barrel |
| `SpaceIcon.tsx` | 版面图标（PascalCase，历史遗留） |

## 命名易混点

`components/admin/admin-shell.tsx`（**分区总壳，含准入**）与 `components/admin/layout/admin-shell.tsx`（**视觉壳层，`AdminChrome`**）同名不同层。`app/admin/layout.tsx` 引入的是前者；前者再引入后者并渲染成 `<AdminChrome>`。改壳层前先确认改的是哪一层。

## 菜单登记（新增后台模块的第 2 步）

`layout/admin-shell.tsx` 的 `menuItems` 是**菜单唯一登记点**：

- `path` / `name` / `icon`（图标从 `~/lib/icons`）
- `hideInMenu: true` 给详情页（`/admin/posts/detail` 等），它们不进菜单但路由仍可直达
- 菜单过滤走 `requiredPermissionsFor(path)`（`lib/admin/auth/route-access.ts`）——**权限表与菜单是两处登记，新增模块必须同时改**

> 根 `AGENTS.md` 曾写「菜单在 `components/admin/layout/pro-shell.tsx` 登记」——该文件不存在。以本文件为准。

### 菜单开合与高亮的语义（改之前先读）

- **滚动**：壳层是**固定高度的单页布局**（根容器 `h-screen overflow-hidden`），只有 `<main>` 是滚动容器（`overflow-y-auto`），侧栏 `<nav>` 单独滚动。**不要退回 `min-h-screen` 文档流布局**：那样页面一长就把侧栏和顶栏一起滚走，用户抱怨的「左侧 side 跟着页面滚」就是它。后台页面因此不能依赖 `window.scrollY`（历史上也没有页面依赖）。
- **分组开合**：`openGroups` 持久化到 localStorage `menu_open_keys`。自动展开只在**路径变化**时发生（用 `useRef` 记住上次自动展开的路径），同一路径下用户的手动收起/展开不会被 effect 撤销。早前 effect 依赖里带了 `openGroups`，于是「当前组」永远收不起来——这是「子选中时父级收不起」的根因。
- **高亮**：`isActive` 是**最长前缀胜出**，且只考虑菜单里真的渲染出来的项。`/admin` 是前缀一切的根，裸 `startsWith` 会让「运营概览」在每个后台页都亮着；把 `hideInMenu` 的详情页算进「更具体」则会让列表项和详情项都不亮。当前实现：路径命中且没有更具体的**可见**兄弟项时才高亮。


## 准入分工（不要在这层重写判定）

- **判定**（要不要放行、跳哪里）在 `lib/admin/auth/access.ts` 的 `useDashboardAccess` / `canAccess`
- **模块级权限表**在 `lib/admin/auth/route-access.ts` 的 `ROUTE_PERMISSIONS`
- **回跳目标**在 `lib/admin/auth/return-to.ts` 的 `captureReturnTarget` / `resolveReturnTarget` / `rememberSiteUrl`
- 本目录只负责**把判定结果渲染成 UI**（403 页、403 模块、菜单过滤）

真正的安全边界在后端网关，这里是 UX 门控。

## 改这里要注意

- **不要给这里的文件加服务端逻辑**：`admin-root.tsx` 的挂载门一旦失效，整段后台会尝试 SSR，antd 时代的样式注入/主题判定问题会立刻复现（虽然 antd 已移除，`globals.css` 的 token 与 `admin.css` 分块仍按 CSR 设计）。
- 新增菜单项必须同时登记 `route-access.ts`，否则菜单会出现但点进去 403。
- `useCan()` 返回 `boolean`，数组语义是 **anyOf**。需要 allOf 的场景自行用原始权限码组合。
- 后台错误回退不要直接抛到前台 `components/error/`：两者边界与文案不同（前台是分区降级，后台是整页失败）。

## 本文件何时更新

- 壳层分层（挂载门 / 分区总壳 / 视觉壳层）变化
- `menuItems` 结构或菜单过滤机制变化
- 壳层滚动模型（`h-screen` + `<main>` 滚动）、菜单开合/高亮语义变化
- 准入组件（403 页 / 模块 403 / AccessProvider）的对外契约变化

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。
