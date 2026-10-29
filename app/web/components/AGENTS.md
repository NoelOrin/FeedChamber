# Web Components（`components/`）

Web 前端的全部 UI 实现。按域分目录，**每个域一个目录**；路由树在 `../app/`，取数与状态在 `../lib/`。

> 上级：[app/web/AGENTS.md](../AGENTS.md) · 相邻：[app](../app/AGENTS.md)、[lib](../lib/AGENTS.md) · 子域：[ui](ui/AGENTS.md)、[admin](admin/AGENTS.md)、[markdown-editor](markdown-editor/AGENTS.md)

实测规模：138 个文件 / 21 个目录（含 3 个顶层文件：`providers.tsx`、`query-hydration.tsx`、`aside/sidebar*.tsx` 所属的 `aside/`）。

## 分层：谁可以依赖谁

```
app/（路由页面）          只装配，不实现交互细节
  └─ components/<domain>/  业务组件：列表、卡片、表单、面板
       └─ components/ui/   shadcn 原语：无业务、无取数
lib/                      跨域基础设施（auth / query / api adapter / seo / i18n / toast）
```

- **`ui/` 不 import 任何 `<domain>/`**，也不 import `lib/auth` / `lib/queries`。原语只接收 props。
- **`<domain>/` 可以 import `ui/` 与 `lib/`**，但**不得 import 另一个 `<domain>/`**。跨域复用要把东西下沉到 `ui/` 或 `lib/`，而不是横向 import。
- **`app/` 可以 import 任何 `components/` 与 `lib/`**，反向不允许（组件不 import 路由页面）。
- 已知的两个受控例外：`components/admin/` 与 `components/layout/` 会互相用少量壳层组件（后台复用主站的 `lib/theme.ts` / `lib/i18n.ts`），这是「后台内嵌在主站」的直接后果，改动时保持窄。

## 子域一览

| 目录 | 文件 | 职责 | 文档 |
|------|------|------|------|
| `ui/` | 33 | shadcn 原语 + 后台表格/表单/树选择等复合原语 | [ui/AGENTS.md](ui/AGENTS.md) |
| `user/` | 23 | 用户主页、资料、关注/收藏/通知列表、统计卡、骨架 | 本文件 |
| `post/` | 11 | 帖子卡片、详情、列表页、composer、相关主题、骨架、`queries.ts` | 本文件 |
| `markdown-editor/` | 11 | tiptap 编辑器 + sanitize 管线（安全边界） | [markdown-editor/AGENTS.md](markdown-editor/AGENTS.md) |
| `layout/` | 10 | Nav / Footer / AppShell / 通知铃 / 公告浮层 / 主题切换 / 搜索浮层 | 本文件 |
| `admin/` | 10 | 后台壳层、准入组件、错误边界 | [admin/AGENTS.md](admin/AGENTS.md) |
| `content/` | 9 | 评论列表、评论项、Markdown 渲染器、代码块与高亮 | 本文件 |
| `auth/` | 7 | 登录/注册表单、OAuth 按钮与回调、认证外壳（`auth-shell` 居中 / `auth-split-shell` 左右分栏） | 本文件 |
| `motion/` | 4 | 社区动议卡片 / 大厅 / 面板 + `queries.ts` | 本文件 |
| `pm/` | 3 | 会话列表、消息线程 + `queries.ts` | 本文件 |
| `search/` | 3 | 搜索面板、结果卡 + `queries.ts` | 本文件 |
| `pagination/` | 2 | `Pager` / `ScrollPaginator` | 本文件 |
| `aside/` | 2 | 侧栏与侧栏内容 | 本文件 |
| `space/` | 2 | 版面卡片、版面图标 | 本文件 |
| `leaderboard/` | 2 | 排行榜面板 + `queries.ts` | 本文件 |
| `checkin/` | 1 | 签到面板 | 本文件 |
| `home/` | 1 | 聚合首页 feed | 本文件 |
| `error/` | 1 | 前台 `ErrorFallback` / 分区错误边界 | 本文件 |
| `seo/` | 1 | `JsonLd`（受控 `dangerouslySetInnerHTML`） | 本文件 |
| `providers.tsx` | 1 | Query / Nuqs / Tooltip / Toast 提供者 + 跨标签会话同步启动点 | 本文件 |
| `query-hydration.tsx` | 1 | `'use client'` 边界，转出 `HydrationBoundary` | 本文件 |

## 取数约定

- **`queries.ts` 只出现在域目录里**（`post/` `user/` `pm/` `search/` `motion/` `leaderboard/`），导出 `queryOptions(...)` / `infiniteQueryOptions(...)`。RSC 预取与客户端 `useQuery` **必须调用同一个工厂**，否则水合后首帧立刻重发。
- **query key 一律取自 `~/lib/query/keys.ts` 的 `qk`**，不在域内拼数组。
- `post/queries.ts` 还负责信息流的**参数归一**（`normalizeListView` / `normalizeSortBy` / `normalizeTag` / `stableFeedKey`）：服务端读 `searchParams` 与客户端读 nuqs 都走同一组函数，非法值当缺省、不改写 URL。
- **单版信息流按 `space_id` + `include_sub` 递归聚合**：`PostFeedParams` 带 `space_id`（不是 `space_slug`）且 `include_sub: true` 时，后端把过滤展开成「本版 + 全部后代版」。`feedParamsOf` 对两者做互斥处理（有 `spaceId` 就不写 `space_slug`，后端 space_id 优先），父版面首页因此能看到子版帖子，叶子版面等价于只看本版。
- 域组件不直接调 `authFetch`；一律经 `@orinforum/api-client` 的 service。

## 组件写法

- 文件命名：组件用 kebab-case（`post-card.tsx`、`user-card.tsx`），只有后台遗留的两个用 PascalCase（`components/admin/SpaceIcon.tsx`、`components/admin/error/ErrorFallback.tsx`）——新文件跟 kebab-case。
- 样式：Tailwind + `cn()`（`~/lib/utils`，即 `@orinforum/shared-utils` 的 `cx`）。**不要写 CSS module**（`markdown-editor/editor.css` 是唯一例外，tiptap 必须）。
- 图标：**一律从 `~/lib/icons` 引入**。`biome.json` 的 `noRestrictedImports` 把直接 import `lucide-react` / `lucide-*` 列为 **error**。
- `'use client'` 只在需要 hooks / 浏览器 API / 事件时加；能被服务端渲染的纯展示组件不要加。
- 正文排版只用 `components/ui/prose.tsx`（Markdown 渲染的唯一出口），不要就地写 typography 类。

## 安全边界

- **Markdown 渲染**：`content/markdown-renderer.tsx` 是唯一出口，统一走 `rehype-sanitize`。不要新增第二套渲染器。
- **`dangerouslySetInnerHTML` 的受控例外只有两处**：`app/layout.tsx` 的首绘主题脚本、`components/seo/json-ld.tsx` 的 JSON-LD（输入是结构化对象、序列化后强制转义 `<`）。新增第三处前先扩展已有组件。
- **公告浮层**：`layout/announcement-overlay.tsx` 的 `iframe` 必须带 `sandbox`，且 `content` 为空时不渲染（空 `src` 会让浏览器重复下载整页）。

## 改这里要注意

- 新增一个域目录 → 本文件「子域一览」加一行；若该域足够复杂（≥10 文件且有独立约定），再建 `components/<domain>/AGENTS.md`。
- 改 `ui/` 下的原语先看外观：历史上有一个 `app/ui-showcase/` 原语对照台，**该目录已不存在**（`app/web/AGENTS.md` 仍在引用它，属文档漂移）。目前验证外观靠 `pnpm dev:web` 手动走查 + `scripts/axe-audit.mjs` 可访问性审计。
- 删除/重命名域内组件时检查 `app/` 的引用；组件没有 index barrel，引用都是显式路径。
- **发帖结果按 `status` 分流**：`post/create` 返回 `status === 'pending'` 时帖子不会进任何公开列表，`post/composer.tsx` 因此提示「已提交审核，通过后将公开展示」并跳到 `/timeline/post?id=<id>`（而不是跳列表，否则作者以为发丢了）；`approved` 才走原来的「发布成功」+ 回列表。详情页 `post/post-detail.tsx` 对 `pending` 渲染一条 `Alert variant="warning"` 的「审核中」说明（后端只对作者本人放行非公开帖，故无需再判身份）。
- **前台外壳宽度是「占满窗口」，不设居中上限**：`layout/` 的 Nav / AppShell / Footer / AnnouncementBanner 四处容器统一 `w-full px-4`，**不要再加 `max-w-*`**（历史上有过 `mx-auto max-w-6xl`，在 ≥1700px 窗口里两侧各留 ~300px 空白）。横向留白只由 `px-4` 提供，四处必须一致，否则页头 / 主列 / 页脚会错位。后台 `admin/layout/admin-shell.tsx` 是另一条线（`max-w-[1280px]`），不受此约定约束。

## 本文件何时更新

- 新增/删除一个域目录
- 分层规则（谁能 import 谁）或取数约定变化
- 新增一处 `dangerouslySetInnerHTML` 或新渲染管线


## 等级文案（2026-10-29）

`components/user/profile-header.tsx` 与 `components/user/visitor-profile-page.tsx` 都通过 `~/lib/trust` 的 `trustLabel()` 渲染等级，不再各自维护 `TRUST_LABELS`。后端 `trust_level` 已是数值（0-4 可扩展），旧的字符串键保留在映射里做兼容；P2.9 改为读取后端配置后删除该映射。
