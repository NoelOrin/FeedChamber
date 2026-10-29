# Web Lib（`lib/`）

Web 前端的跨域基础设施：会话状态、取数接缝、API 适配、主题/语言/设置、SEO、图标与工具。

> 上级：[app/web/AGENTS.md](../AGENTS.md) · 相邻：[app](../app/AGENTS.md)、[components](../components/AGENTS.md) · 子域：[auth](auth/AGENTS.md)

实测规模：25 个文件 / 5 个子目录（`admin/`、`api/`、`auth/`、`hooks/`、`queries/`、`query/`）。

## 分层

```
lib/auth/        会话状态机 + 服务端 guards + 跨标签同步
lib/query/       QueryClient（RSC 每请求一份 / 浏览器单例）+ 全站唯一 key 来源
lib/queries/     RSC 取数接缝（cache() 包裹）+ 外壳查询契约
lib/api/         向 @feedchamber/api-client 注入 SSR cookie / request id
lib/admin/       后台准入（access / route-access / return-to）与导航适配
lib/hooks/       SSE 订阅、公告等跨域 hook
lib/*.ts         theme / i18n / settings / seo / pagination / toast / icons / utils
```

**依赖方向**：`lib/*` 不 import `components/` 与 `app/`；`lib/auth` 不 import `lib/queries` 之外的域代码。

## 文件表

| 文件 | 职责 |
|------|------|
| `query/client.ts` | `getQueryClient()`（RSC 用 `cache()`，**每请求一份**）/ `getBrowserQueryClient()`（浏览器单例）。`DEFAULT_STALE_TIME` 60s、`FEED_STALE_TIME` 10s；`shouldRetry` 只对传输错误与 5xx 重试（4xx 重试必然同样失败） |
| `query/keys.ts` | `qk` —— **全站 query key 唯一来源**。页面与组件不得本地拼数组，否则 `invalidateQueries` 会漏掉对方写入的缓存 |
| `queries/server.ts` | RSC 接缝：`loadPublicSettings` / `loadProfile` / `loadSpaceTree` / `loadShellData`，全部 `cache()` 包裹，**失败降级成空值**（页面骨架照常 SSR，不整树抛错） |
| `queries/shell.ts` | 外壳级查询（版面树、标签列表）：RSC `prefetchQuery` 与组件 `useQuery` 共用同一个 `queryOptions`，`queryFn` 必须无参 |
| `api/adapter.ts` | `registerWebAdapter({ readRequestCookie, readRequestId })` + 进程级 `setApiBaseUrl`。**任何 SSR 取数的页面都要先 import 它** |
| `auth/session.ts` / `auth/client.ts` / `auth/sync.ts` / `auth/index.ts` / `auth/guards.ts` | 见 [auth/AGENTS.md](auth/AGENTS.md) |
| `admin/auth/access.ts` | 后台准入门（`useDashboardAccess` / `canAccess` / `DASHBOARD_ROLES` / `DASHBOARD_PERMISSION_CODES`） |
| `admin/auth/route-access.ts` | `ROUTE_PERMISSIONS`：路径 → `anyOf` 权限码，菜单过滤与壳层 403 共用 |
| `admin/auth/return-to.ts` | `captureReturnTarget` / `resolveReturnTarget` / `rememberSiteUrl` |
| `admin/navigation.ts` | `useNavigate` / `useLocation` / 查询参数 hook（页面不要自己拼 `window.location`） |
| `admin/permissions.ts` | 权限码前缀 → 分组标签（`permissionGroup` / `groupPermissions`） |
| `hooks/use-notification-sse.ts` | 通知未读数 SSE：走 `@feedchamber/sse-client`，**浏览器侧必须是相对路径** `/api/v1/notifications/stream` |
| `hooks/use-announcements.ts` | 公告取数与展示状态 |
| `hooks/use-trust-levels.ts` | 等级名称的唯一来源：`useTrustLevels()` 读公开接口 `GET /levels`（staleTime 5 分钟），`trustLevelName()` 渲染；P2.9 起不再有硬编码中文标签（旧的 `lib/trust.ts` 已删除） |
| `theme.ts` | `useTheme`（light/dark/system + localStorage + 系统偏好），首绘前脚本在根布局 |
| `i18n.ts` | `useLocale` / `t()`，zh-CN 词条表；前台与后台共用 |
| `settings.ts` | `usePublicSettings` / `useSiteName`（`qk.settings`），前后台共用一份缓存 |
| `seo.ts` | `listPageMetadataFromSettings` / `siteImage`：列表页元数据 + 自指 canonical（**刻意不带**筛选与分页参数） |
| `pagination.ts` | 分页尺寸 fallback（与服务端 `model.DefaultSettings` 一致） |
| `toast.ts` | 命令式 toast（zustand store + 视口组件在 `components/ui/toast.tsx`） |
| `icons.ts` | 图标唯一出口；`biome.json` 禁止直接 import `lucide-react` |
| `utils.ts` | `cn()` = `@feedchamber/shared-utils` 的 `cx` |

## 三条最容易踩的规则

1. **`lib/auth/guards.ts` 不能被客户端 import**：它 import `next/headers`。`lib/auth/index.ts` **刻意不 re-export 它**——一旦 re-export，`next/headers` 会进浏览器包，构建期就炸。服务端组件请直接 `import { requireAuth } from '~/lib/auth/guards'`。
2. **QueryClient 不能跨请求复用**：SSR 必须 `getQueryClient()`（`cache()` 的每请求实例），浏览器才用单例。用错会把上一位访客的信息流渲进这一位的 HTML。
3. **query key 只在 `qk` 里定义**：域内自拼 key 是本仓库最常见的缓存失效 bug 来源。

## 改这里要注意

- 改 `qk` 的结构等于改缓存身份：旧的 key 会变成孤儿，`invalidateQueries` 命中不到。改名时全仓 grep 一次。
- `queries/server.ts` 的「失败降级成空值」是刻意设计（SSR 不整树失败）。不要让某个 loader 改成抛错——那会让页面在数据库抖动时整页 500。
- `seo.ts` 的 canonical 归一规则与后端 sitemap（`handler/seo.go`）必须一致，否则 sitemap 指向的地址和页面声明的规范地址对不上。
- `hooks/use-notification-sse.ts` 用相对路径是刻意的：SSR 侧 `setApiBaseUrl` 会把 base 改成绝对地址，而这条流只在浏览器建连。

## 本文件何时更新

- 新增/删除 `lib/` 下的模块或子目录
- `qk` 顶层结构变化
- `query/client.ts` 的 staleTime / retry 策略变化
- RSC 接缝（`queries/server.ts`）的降级语义变化
