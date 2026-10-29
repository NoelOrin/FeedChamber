# Web App Router（`app/`）

vinext（Next App Router 的 Vite 实现）路由树。**只放路由骨架与页面装配**，业务组件在 `../components/<domain>/`，取数与状态在 `../lib/`。

> 上级：[app/web/AGENTS.md](../AGENTS.md) · 相邻：[components](../components/AGENTS.md)、[lib](../lib/AGENTS.md)、[admin](admin/AGENTS.md)

实测规模：76 个文件（含页面组件与 `admin.css`）。**URL 是硬契约**（站外链接 + SEO + sitemap），路由一经发布不得改动。

## 路由组

| 组 | 目录 | 登录语义 | 说明 |
|----|------|----------|------|
| 公开内容 + 私有面共用外壳 | `(app)/` | **布局不兜底鉴权**，由页面各自守 | `AppShell` + 侧栏 + 通知铃；`layout.tsx` 只加载公开设置 |
| 免登录组 | `(auth)/` | `requireGuest`（已登录直接弹回） | `login` / `register` / `oauth` |
| 管理后台 | `admin/` | 纯 CSR，见 [admin/AGENTS.md](admin/AGENTS.md) | `/admin/**`，服务端只吐骨架 |
| 根 | `layout.tsx` / `error.tsx` / `not-found.tsx` / `globals.css` | — | 根文档、首绘前主题脚本、Providers、JSON-LD |

`(app)` 与 `(auth)` 是**路由组**（括号不进 URL）。注意目录 `app/web/app` 与 monorepo 根的 `app/` 同名但不同层。

`(auth)/login` 与 `(auth)/register` 共用 `components/auth/auth-split-shell.tsx` 的**左右分栏**外壳：左栏是品牌紫装饰面（字标 / 标语 / `site_description` / 三条特性），右栏是表单。分栏从 `md`（768px）起生效，手机竖屏退回单栏并在顶部显示字标。`(auth)/oauth` 仍用居中的 `auth-shell.tsx`——它是回调页，不需要装饰面。

> 装饰面只用纯色 + 线稿圆环（叠加一层 `bg-black/20` 压暗保证小字对比度），不引入渐变/噪点/网格——与 `globals.css` 的视觉基线一致。左栏标语是装饰性文案，用 `<p>` 而非标题元素，保证页面唯一的 `<h1>` 是右栏表单标题。

## 页面清单（`(app)/`）

| 路由 | 文件 | 登录判定 |
|------|------|----------|
| `/` | `page.tsx` | 重定向 → `/timeline` |
| `/timeline` | `timeline/page.tsx` + `timeline-list.tsx` | `requireAuthBySetting('post_auth_required')` |
| `/timeline/post` | `timeline/post/page.tsx` | 同上；**`?id=` 查询参数**，不是路径参数 |
| `/timeline/create` | `timeline/create/page.tsx` | 结构性私有（`proxy.ts` 的 `PROTECTED`） |
| `/timeline/tag` | `timeline/tag/page.tsx` + `tag-list.tsx` | 公开 |
| `/t/{slug}/{id}` | `t/[slug]/[id]/page.tsx` | **帖子详情的规范地址**，直接 200 渲染 |
| `/spaces` `/spaces/{slug}` | `spaces/page.tsx`、`spaces/[slug]/page.tsx` + `space-home.tsx` | `requireAuthBySetting('space_view_auth_required')`；单版首页=面包屑 + 头部卡 + **直接子版面胶囊**（有子版时）+ 帖子列表，无「关注版面」（2026-10-28 移除）。帖子列表按 `space_id` + `include_sub` **递归聚合子版面**（父版显示子版内容），不再按 `space_slug` 过滤；子版胶囊来自版面树的 `node.children`，可再下钻到孙版 |
| `/spaces/{slug}/post` | `spaces/[slug]/post/page.tsx` | 发帖，结构性私有 |
| `/user` + `{posts,notifications,follow,favorites}` | `user/*` | 自身主页，结构性私有 |
| `/user/{userId}` | `user/[userId]/page.tsx` | 他人主页，`requireAuthBySetting('user_profile_auth_required')` |
| `/messages` `/messages/{id}` | `messages/*` | 结构性私有 |
| `/home` | `home/page.tsx` | 结构性私有（聚合首页） |
| `/search` `/leaderboard` `/motions` | 同名目录 | 公开 |

## 登录墙分两层（不要合并）

1. **`proxy.ts`（边缘，只能读 cookie）** 只挡**结构性私有**路径：`/messages`、`/home`、`/timeline/create`、精确的 `/user`。它**读不到后端设置**，所以不参与内容页判定。
2. **RSC 层（页面内）** 用 `requireAuthBySetting(settingKey, pathname)` 判定内容页是否要登录。设置非 `'true'` 放行，为 `'true'` 时等价 `requireAuth`。

映射：`/timeline` `/timeline/post` `/t/{slug}/{id}` → `post_auth_required`；`/spaces` `/spaces/{slug}` → `space_view_auth_required`；`/user/{id}` → `user_profile_auth_required`。三者**互相独立**。

**前端不得硬编码「哪个页面匿名可读」**——这是管理员可改的服务端设置。规范见 `openspec/specs/seo`。

## 页面写法（本仓库的固定形状）

```tsx
// 1. 服务端页面：只做「预取 + 水合 + 传数据」，不写业务交互
export async function generateMetadata(): Promise<Metadata> { /* lib/seo.ts 的 helper */ }

export default async function Page() {
  await requireAuthBySetting('post_auth_required', '/timeline');   // 或 requireAuth()
  const qc = getQueryClient();
  await qc.prefetchQuery(...);                                     // 用 lib/query/keys.ts 的 qk
  return <HydrationBoundary state={dehydrate(qc)}><ClientList /></HydrationBoundary>;
}
```

- **import `~/lib/api/adapter`**：任何会经 `@orinforum/api-client` 取数的页面都要先导入它，SSR 才能带上请求 cookie。
- **`HydrationBoundary` 必须从 `~/components/query-hydration` 导入**，不能直接从 `@tanstack/react-query` 导——后者在服务端组件里会调 `useQueryClient`（此时无 Provider）。
- **query key 一律取自 `~/lib/query/keys.ts` 的 `qk`**，页面与组件不得本地拼数组，否则 `invalidateQueries` 会漏。
- **取数走 RSC 接缝 `~/lib/queries/server.ts`（`loadPublicSettings` / `loadProfile` / `loadSpaceTree`）**，它们都用 `cache()` 包过，同一次请求内 `generateMetadata → layout → page` 只发一次后端调用。
- 元数据统一用 `~/lib/seo.ts` 的 helper（`listPageMetadataFromSettings` / `siteImage`），保证自指 canonical 与筛选参数归一。
- `'use client'` 只出现在真正需要交互的页面组件上；服务端页面不要加。

## 文件表要点

| 文件 | 说明 |
|------|------|
| `layout.tsx` | 根文档：`viewport.themeColor`、首绘前主题脚本、`Providers`（Query/Nuqs/Tooltip/Toast）、`JsonLd`；检测 `/admin/**` 时**跳过**外壳取数并返回 noindex 元信息 |
| `globals.css` | shadcn token 的**唯一载体**（Web 不依赖 `@orinforum/tokens`） |
| `error.tsx` / `global-error.tsx` / `not-found.tsx` | 错误边界与 404 |
| `admin/admin.css` | 只打进 `/admin` 的 CSS 分块，不污染前台 |

## 改这里要注意

- **不要在 `(app)/layout.tsx` 里加 `requireAuth()`**。2026-10-28 之前这里无条件鉴权，与后端 `AuthGate` 的默认放行相反，匿名访客连帖子详情都被 307 到 `/login`，爬虫一条都抓不到。
- **`/timeline/post?id=` 与 `/t/{slug}/{id}` 是同一内容的两个入口**：前者 canonical 指回后者。改任一都要同步 sitemap（后端 `handler/seo.go` 独占）与 `docs/api-reference.md`。
- **爬虫文件由后端独占**：`/sitemap.xml`、`/sitemap-posts.xml`、`/sitemap-spaces.xml`、`/sitemap-tags.xml`、`/robots.txt`、`/latest.rss` 全部挂在后端 app 根，前端**不生成任何爬虫文件**。nginx 用 6 条精确 `location =` 转给后端（见 [deploy/AGENTS.md](../../../deploy/AGENTS.md)）。
- **`?redirect=` 只接受站内相对路径**，拒绝 `//host` 与 `/\host`（防开放重定向）。
- **`dangerouslySetInnerHTML` 的受控例外只有两处**：根布局的首绘主题脚本、`components/seo/json-ld.tsx`。新增第三处前先扩展已有组件。
- **`Card` 基类是 `flex flex-col`，调用点想横向排布必须显式给 `flex-row`**：tailwind-merge 只合并同一属性组的类，`flex-wrap` 与 `flex-col` 不同组，所以 `<Card className="flex flex-wrap gap-2 p-4">` 合并后方向仍是 column —— 子元素被拉成整行（`/timeline/tag` 曾因此把 28 个标签渲染成 28 条 1412px 的整行，见 `timeline/tag/tag-list.tsx`）。要么写 `flex flex-row flex-wrap`，要么像标签页那样把横向流放进 Card 内部单独一层 `div`。
- **`<button>` 的 `cursor: pointer` 由 `globals.css` 的 base 层统一提供**（Tailwind v4 删掉了这个默认值）。写裸 `<button>` 时**不必**再手写 `cursor-pointer`，也别指望浏览器默认；需要别的指针用 `.cursor-default` / `.cursor-not-allowed`（utilities 层优先级更高）。

## 本文件何时更新

- 新增/删除/改名一个路由（URL 是硬契约，改名等于破坏站外链接）
- 登录墙分层或设置项映射变化
- 根布局的取数旁路、Providers 组成、元数据策略变化
