# Web 前端

用户端论坛，React 19 + vinext（Next App Router 的 Vite 实现）+ TanStack Query + shadcn 风格原语 + Tailwind v4。

> 📱 **移动端构建指南**：[MOBILE.md](MOBILE.md) — 本期（vinext 迁移）相关流程**已挂起**，恢复移动端时需重新校订。
> 🧭 **vinext 迁移已完成并合入主干**（Solid 树已删除，本目录即 React 19 + vinext 实现）。迁移决策记录已归档，视觉规范以 `app/web/app/globals.css`（shadcn token）与 `components/ui/` 原语为准（`documents/designs/current-ui_design_system.md` 是 2026-10-18 重构前的旧基线，已废弃，仅作历史参考）。
> ⚠️ 旧 Solid 树已随迁移收尾删除，需查阅原始实现时从 git 历史读取：`git show 890815b^:app/web/src/<path>`。

## 技术栈

- React 19 + vinext 1.1 — App Router（`app/` 目录、RSC、Server Actions），`proxy.ts` 取代 `middleware.ts`
- TanStack Query 5 — 取数/缓存（**不再使用 TanStack Router**，路由由 vinext 承担；`TanStack` 在本包只指 Query）
- shadcn/ui 基准的原语 + `app/globals.css` 自带的 shadcn token（中性色 + 冷紫主色）。**Web 不再依赖 `@feedchamber/tokens`**（该包只服务 Dashboard）；daisyUI 已移除
- `@feedchamber/api-client` — 唯一请求层（`authFetch` + 401 单飞刷新），SSR cookie 经 `next/headers` 转发
- `@feedchamber/sse-client` — 实时未读数
- nuqs — URL 查询参数校验（取代 TanStack 的 `validateSearch`）
- **容器运行时是 Bun，本地开发仍是 Node**：`Dockerfile` Stage 2 用 `oven/bun:1.4.4-alpine` + `bun node_modules/vinext/dist/cli.js start`，Stage 1 构建仍走 `node:22-alpine` + pnpm（产物是纯 JS，换运行时不碰锁文件）。**不要降到 Bun 1.3.x**：那版 `node:http` 对 HTTP/1.0 请求返回 200 后不关连接，`ab`/`curl --http1.0` 会挂死，1.4.4 已修。

## 目录

```
app/                    # App Router 路由（注意与 monorepo 根的 app/ 重名但不同层）
├── layout.tsx          # 根文档 + 首绘前主题脚本 + Providers（Query/Nuqs/Tooltip/Toast）
├── (app)/              # 公开内容 + 私有面共用的外壳（AppShell；登录判定下沉到各页面）
├── (auth)/             # 免登录组（layout 内 requireGuest）：login / register / oauth
├── admin/              # 管理后台（/admin/**，纯 CSR，见下节）
components/
├── ui/                 # shadcn 原语（button card badge tabs dialog … prose）
├── layout/             # Nav / Sidebar / Footer / AppShell / 通知铃 / 用户卡
├── pagination/         # Pager · Paginator · ScrollPaginator
├── error/              # ErrorFallback · SectionErrorBoundary
├── admin/              # 后台壳层（AdminShell）、准入组件、后台专用 hooks
└── <domain>/           # post / content / user / markdown-editor / …
lib/
├── auth/               # zustand 会话状态机 + 服务端 guards + 跨标签同步
├── query/              # client.ts（RSC 每请求一份 / 浏览器单例）+ keys.ts（全站唯一 key 来源）
├── queries/            # server.ts（RSC `cache()` 数据接缝）+ shell.ts
├── api/adapter.ts      # 向 api-client 注入 readRequestCookie
├── admin/              # 后台准入（access / route-access / return-to）与导航适配
└── theme.ts · toast.ts · i18n.ts · pagination.ts · icons.ts
```

## 路由

| 路由 | 页面 | 说明 |
|------|------|------|
| `/` | 重定向 | → `/timeline` |
| `/timeline` | 时间线 | 帖子列表，`?tag= &sort_by= &list_view=`（nuqs） |
| `/timeline/post` | 帖子详情 | **`?id=` 查询参数，不是路径参数** |
| `/timeline/create` `/timeline/tag` | 发帖 / 标签 | |
| `/spaces` `/spaces/{slug}` | 版面总览 / 单版首页 | |
| `/spaces/{slug}/post` | 版内发帖 | |
| `/user` | 我的主页 | `/user/posts` 重定向到 `/user` |
| `/user/{posts,notifications,follow,favorites}` | 用户中心子页 | |
| `/user/{userId}` | 他人主页 | 静态段优先于 `[userId]` |
| `/messages` `/messages/{id}` | 私信 | 私有面，页面内 `requireAuth` |
| `/home` `/leaderboard` `/search` `/t/{slug}/{id}` | 聚合 / 排行 / 搜索 / 帖子规范地址 | `/t/` 直接渲染（200）；`/timeline/post?id=` 的 canonical 指回它 |
| `/login` `/register` `/oauth` | 认证页 | `?redirect=` 仅接受站内相对路径 |

> URL 是硬契约（站外链接 + SEO + sitemap），路由一经发布不得改动。
> 未实现：`/first-change-password`（R-05 强制改密，后端已支持，Solid 版也没有这个页面）。

**哪些页面匿名可读**：`/timeline`、`/spaces`、`/spaces/{slug}`、`/t/{slug}/{id}`、`/timeline/post?id=`、`/search`、`/leaderboard`，以及 `user_profile_auth_required=false` 时的 `/user/{id}`。这个判定**由服务端设置决定**（`post_auth_required` / `space_view_auth_required` / `user_profile_auth_required`），前端不得硬编码——见 `openspec/changes/2026-10-28-seo-public-indexing`。

## 后台（`/admin`）

后台不再是独立应用，而是本应用里的一段路由（`app/admin/**`），与前台同一个构建、origin 与部署；组件与前台共用 `components/ui/` 下的 shadcn 原语（antd / Pro Components 已全部移除，见下方「后台去 antd」）。

- **不走 SSR**：`app/admin/layout.tsx` 用客户端挂载门 `components/admin/admin-root.tsx` 把整段路由挡在服务端渲染之外；页面组件全部 `'use client'`。
- **准入**：`lib/admin/auth/access.ts`（拉 `/user/profile`，按角色/权限码判定）；未登录跳 `/login?redirect=/admin/...`，普通用户原路返回，无回跳目标才 403。
- **模块门控**：`lib/admin/auth/route-access.ts` 维护「路径 → 权限码」，菜单过滤与壳层内 403 共用；页面内可用 `useCan([...])`。
- **登录**：复用主站 `/login`（带 `redirect`），后台不再有独立登录页；首改密在 `/admin/first-change-password`。
- **外壳取数旁路**：`proxy.ts` 会透传 `x-pathname`，根布局与 `generateMetadata` 检测到 `/admin/**` 时跳过 spaces/tags 的外壳取数，`app/admin/admin.css` 只打进 `/admin` 的 CSS 分块，不污染前台。
- **复用优先**：主题/语言/站点名/设置都直接用主站的 `lib/theme.ts`、`lib/i18n.ts`、`lib/settings.ts`（`useTheme`/`useLocale`/`useSiteName`），后台不再自留一套 hook；后台自研的只有权限相关（`lib/admin/auth/*` + `components/admin/access/*`）。

## 关键结构

- `lib/auth/session.ts` — 认证状态机（unknown/anonymous/loading/authenticated/refreshing/expired），zustand + `authSession` 命令式门面
- `lib/auth/guards.ts` — 服务端 `requireAuth` / `requireGuest`（`headers()` + `redirect()`）；**刻意不从 `lib/auth/index.ts` 再导出**，否则 `next/headers` 会被打进浏览器包
- `lib/auth/sync.ts` — BroadcastChannel + localStorage 跨标签页同步，启动点在 `components/providers.tsx`
- `components/ui/prose.tsx` — 正文排版原语（Markdown 渲染唯一出口，shadcn 不覆盖）
- 原语外观验证：`app/ui-showcase/` 对照台**已不存在**，改完 `components/ui/*` 用 `pnpm dev:web` 走查 + `node scripts/axe-audit.mjs`
- `lib/icons.ts` — 图标唯一出口，`biome.json` 已把直接 import `lucide-react` 列为 error
- `Makefile` / `capacitor.config.ts` — 移动端构建入口，本期挂起

## Mobile Build (iOS / Android H5)

> **本期挂起**：移动端构建已暂停。以下描述的是旧流程，恢复移动端时必须按 vinext 产物重新校订（尤其是 `webDir` 不再是 Nitro 的 `.output/public`）。

基于 [Capacitor](https://capacitorjs.com/) 将 Web 打包为 iOS / Android 原生壳。

基于 [Capacitor](https://capacitorjs.com/) 将 Web 打包为 iOS / Android 原生壳。

### 关键文件

| 文件 | 说明 |
|------|------|
| `Makefile` | 所有移动端命令入口（init / add-* / sync / dev / build-* / icon / clean） |
| `capacitor.config.ts` | Capacitor 配置，通过 `dotenv` 加载 `.env.mobile` |
| `.env.mobile.example` | 配置模板（提交到 git） |
| `.env.mobile` | 本地配置（gitignored，含开发服务器地址、App ID、签名信息） |
| `public/pwa-192x192.png` / `pwa-512x512.png` | PWA + 原生图标源文件 |
| `ios/` / `android/` | 原生项目（gitignored，`make add-*` 生成） |

### 开发模式 vs 生产模式

`capacitor.config.ts` 根据 `MOBILE_SERVER_URL` 是否设置切换模式：

- **开发模式**（`MOBILE_SERVER_URL=http://<IP>:3000`）：原生壳加载 Vite dev server，支持 HMR 热重载。`make dev-ios` / `make dev-android` 启动模拟器。
- **生产模式**（`MOBILE_SERVER_URL` 留空）：原生壳加载打包进 app 的静态资源（`.output/public/`）。`make build-ios` / `make build-android` 出包。

### 典型工作流

```bash
# 1. 首次设置
cp .env.mobile.example .env.mobile   # 编辑 MOBILE_SERVER_URL、CAPACITOR_APP_ID
make init                            # 安装 Capacitor 依赖
make add                             # 同时初始化 iOS + Android 原生项目
make icon                            # 从 pwa-512x512.png 生成全套图标

# 2. 日常开发（HMR）
make dev                             # 一键启动 Vite + iOS（PLATFORM=android 切 Android）
# 或两终端：终端 A `make dev-vite`，终端 B `make dev-ios`

# 3. 出包
make sync && make open-ios           # 在 Xcode 配置签名
make build-ios                       # 生成 .ipa
make build-android                   # 生成 .apk（调试）
make release-android                 # 生成 .aab（Google Play）
```

### 前置条件

- **iOS**: Xcode 15+、CocoaPods（`pod --version`）
- **Android**: Android Studio、JDK 17、Gradle
- **通用**: Node 20+、pnpm

### 约定

- `MOBILE_SERVER_URL` 支持 `http://`（开发，`cleartext: true`）或 `https://`（生产）
- `webDir` 固定指向 `.output/public`（Nitro 产物）
- `.env.mobile` 中的 `IOS_DEV_TEAM` 仅在 `make build-ios` 时自动签名使用，也可在 Xcode 手动配置
- `make clean` 保留原生项目配置；`make clean-all` 不可逆删除 ios/ 和 android/

## 安全

- **登录墙分两层，职责不同**：`proxy.ts` 只管**结构性私有**路径（`/messages`、`/timeline/create`、`/home`、精确的 `/user`）——它跑在边缘只读得到 cookie，**读不到后端设置**，所以内容页要不要登录不归它判。公开内容面的判定发生在 RSC 层：页面调 `requireAuthBySetting(settingKey, pathname)`（`lib/auth/guards.ts`），设置非 `'true'` 放行、为 `'true'` 时等价 `requireAuth`。映射是 `/timeline` `/timeline/post` `/t/{slug}/{id}` → `post_auth_required`，`/spaces` `/spaces/{slug}` → `space_view_auth_required`，两者**互相独立**。`(app)/layout.tsx` **不再兜底 `requireAuth`**，私有页各自在页面内守。
- `(auth)/layout.tsx` 的 `requireGuest` 阻止已登录用户停留在登录/注册页
- 登录后的 `?redirect=` 只接受站内相对路径，拒绝 `//host` 与 `/\host`（防开放重定向）
- 会话凭据是 HttpOnly cookie（`master_session`），JS 只读非 HttpOnly 的 `master_auth` 标志位
- 公告弹窗的 `iframe` 带 `sandbox`，且 `content` 为空时不渲染（空 `src` 会让浏览器重复下载整页）
- 正文渲染统一走 `rehype-sanitize`（`components/content/markdown-renderer.tsx`）。`dangerouslySetInnerHTML` 的**受控例外**共五处：根布局的首绘主题脚本、`components/seo/json-ld.tsx` 的 JSON-LD（输入是结构化对象、序列化后强制转义 `<`），以及**站点自定义层的三处注入**（主题 token 覆盖 / 自定义 CSS / 自定义 JS，见下条）。新增第六处前先扩展已有组件，不要就地再写一次。
- **站点自定义注入是产品决定的「有意允许任意内容」**：`app/layout.tsx` 的 `fc-theme-overrides` / `fc-custom-css` / `fc-custom-js` / `fc-custom-body` 四处，内容来自后台「站点自定义」。风险由**写入侧**控制（仅 admin 可写、机器凭证写 JS/HTML 强制草稿、全量审计），**渲染侧刻意不做过滤**——过滤会让站长写不了复杂选择器。`/admin/**` 不注入（`isAdmin` 分支返回空），避免任意脚本在后台上下文执行。

## 站点自定义注入（2026-10-30 修订）

注入的容器选择统一在 `lib/customization/emit.ts`（`emitSnippets(snippets, slot)`），`app/layout.tsx` 只渲染结果，不按 kind 临时判断：

| kind | 容器 | 说明 |
|------|------|------|
| css | `<style>` | 会中和 `</style` |
| js | `<script>` | 会中和 `</script>` |
| html | 容器 `<div>` | 塞进 `<script>` 只会变成语法错误；html 一律 body_end |

服务端写入时已拒绝含 `</style` / `</script` 的内容（`service.validateSnippetContent`），emit 层再做一次中和——这条链路的失效模式是全站脚本执行，两处都独立成立。

因此本节的 `dangerouslySetInnerHTML` 受控例外清单以 `emit.ts` 为准：`fc-theme-overrides`（token）、`fc-custom-css-*`、`fc-custom-js-*`、`fc-custom-body-*`（容器 div）、首绘主题脚本、JSON-LD。
