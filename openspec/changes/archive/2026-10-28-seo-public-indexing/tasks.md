# Tasks

> 提案：[proposal.md](./proposal.md)；设计：[design.md](./design.md)（D1–D8 为决策编号，风险表为验收重点）。
> 验证 Go 需 `export PATH="/usr/local/go/bin:$PATH"` 或登录 shell。

## 1. 设置项与数据来源（后端）

- [x] 1.1 `internal/model/setting.go`：`DefaultSettings` 新增 `seo_description`（默认空）、`og_image`（默认空）、`google_site_verification`（默认空）、`baidu_site_verification`（默认空）、`user_profile_indexable`（默认 `false`）、`sitemap_page_size`（默认 `5000`）
- [x] 1.2 `internal/model/setting.go`：`PublicSettingKeys` 追加 `seo_description`、`og_image`、`google_site_verification`、`baidu_site_verification`、`user_profile_indexable`；**`seo_title` 已在列，确认不动**。验收：`grep -n "seo_description" internal/model/setting.go` 同时命中 `DefaultSettings` 与 `PublicSettingKeys`，消除「读不存在的键」
- [x] 1.3 `internal/model/setting.go`：新增 `SettingUserProfileIndexable`、`SettingSEODescription` 等常量，供 handler/router 引用（避免字面量散落）

## 2. Sitemap 数据查询（后端）

- [x] 2.1 `internal/repository/post.go`：接口新增 `ListSitemapEntries(ctx, offset, limit int) ([]SitemapEntry, error)` 与 `CountSitemapEntries(ctx) (int, error)`；`SitemapEntry` 只含 `ID/Slug/UpdatedAt`（不取正文、不 JOIN users/spaces）
- [x] 2.2 `internal/repository/post_sql.go`（或对应实现文件）：实现上述两方法，`WHERE` 限定 `status IN ('approved','closed')` 且排除软删除，`ORDER BY updated_at DESC`
- [x] 2.3 验收：新增 repo 层测试，断言 `pending` 状态帖子不出现在结果中、`limit/offset` 生效
- [x] 2.4 `internal/model/post.go`：新增复合索引 `idx_posts_sitemap(deleted_at, updated_at, id)`。**原设计漏了索引**：`ORDER BY updated_at DESC, id DESC` 无索引可用，查询计划为 `USE TEMP B-TREE FOR ORDER BY`，等于每次 sitemap 请求都把全部公开帖排序一遍
  - 索引列序经过三次实测才定：`(status, deleted_at, updated_at)` 仍有 TEMP B-TREE（`status IN (...)` 是范围条件，会阻断后续列的排序用途）；`(deleted_at, updated_at)` 降为 `LAST TERM OF ORDER BY`；补上 `id` 后 TEMP B-TREE **完全消失**
  - 5 万帖实测：offset=0 33.6→21.6ms、**offset=5000 78.3→4.3ms（18×）**、offset=50000 35.7→12.3ms；中段分片收益最大
  - 排序结果与直接查 DB 一致（最新在前），功能无回归
- [x] 2.5 跨库实测（上一轮只验了 SQLite 就断言「SQLite/MySQL 都能顺索引扫描」，属未验证的推断）：
  - **PostgreSQL**：用真实 PG 16 容器 + 5 万行验证，优化器**不会**选 `idx_posts_sitemap`（`deleted_at` 选择性太差），强制启用与默认顺序扫描同为 ~8ms，**无收益**，只增加写入成本。保留该索引的前提是「默认部署是 SQLite，PG 侧代价可忽略」，已写进代码注释
  - **MySQL**：**未能验证**——`AutoMigrate` 在既有缺陷处失败（`Space.Name` / `Tag.Name` 等无长度的 `string` + `uniqueIndex` 会生成 TEXT 列，报 `Error 1170: BLOB/TEXT column used in key specification without a key length`）。这是 2026 年早期就存在的跨库问题，与本次索引无关，但**意味着 MySQL 后端当前无法启动**
  - 代码注释已从断言改为实测结论（分 SQLite / PostgreSQL / MySQL 三段），避免后来者按直觉假设「加索引就一定更快」
- [x] 2.6 附带结论：`CountSitemapEntries` 的 `COUNT(*)` 在 5 万帖下 62ms，爬虫端点每天数次请求，**不值得优化**（上一轮曾列为候选，实测后否决）

## 3. 爬虫端点重写（后端）

- [x] 3.1 `internal/handler/seo.go`：`SitemapIndex` 改为输出完整索引——`sitemap-posts.xml?page=1..N`（N 由 `CountSitemapEntries` 与 `sitemap_page_size` 算出）、`sitemap-spaces.xml`、`sitemap-tags.xml`
- [x] 3.2 `internal/handler/seo.go`：`Sitemap` 支持 `?page=N`（默认 1，非法值归 1），超出范围返回**合法空 `<urlset>`**而非 404
- [x] 3.3 `internal/handler/seo.go`：新增 `SitemapSpaces`，遍历启用版面输出 `/spaces/{slug}`，`lastmod` 取 `UpdatedAt`
- [x] 3.4 `internal/handler/seo.go`：新增 `SitemapTags`，输出 `/timeline?tag={urlencoded}`（`Tag` 无 slug，见 design Context）
- [x] 3.5 `internal/handler/seo.go`：`Robots` 补 `Disallow: /messages`、`/timeline/create`、`/user/`；**当 `user_profile_indexable=true` 时 MUST NOT 输出 `Disallow: /user/`**（design 风险表已单列此联动）
- [x] 3.6 `internal/handler/seo.go`：`LatestRSS` 补 `<language>`、`<lastBuildDate>`、`atom:link rel=self`、`<description>`
- [x] 3.7 `internal/handler/seo.go`：抽出 `postURL(siteURL, slug, id)` 单点函数，sitemap/RSS/索引三处共用，防止 URL 形态再次分叉
- [x] 3.8 `internal/router/router.go`：注册 `app.Get("/sitemap-spaces.xml", h.SEO.SitemapSpaces)` 与 `app.Get("/sitemap-tags.xml", h.SEO.SitemapTags)`
- [x] 3.9 `internal/handler/handler.go` 与 `server/wiring.go`：`SEOHandler` 若新增依赖（如 `SpaceService`）则补接线，漏接线须编译报错

## 4. 部署层（nginx）

- [x] 4.1 `deploy/nginx/nginx.conf`：在 `location /` **之前**插入 6 条精确 location（`= /sitemap.xml`、`= /sitemap-posts.xml`、`= /sitemap-spaces.xml`、`= /sitemap-tags.xml`、`= /robots.txt`、`= /latest.rss`）转 `http://backend:8080`
- [x] 4.2 验收：`nginx -t` 通过；起容器后 `curl -I http://<host>/sitemap.xml` 返回 `200` 与 `application/xml`
- [x] 4.3 端到端验收（此前只做了 `nginx -t` 语法检查，未验证**路由分工**）：用 compose 里真实的挂载路径 `conf.d/default.conf` 起 nginx 容器，6 个爬虫路径逐条请求，全部命中后端（返回 XML / `User-agent`）而非前端；非爬虫路径（`/timeline`）落到前端。`Content-Type` 分别为 `application/xml; charset=utf-8`、`text/plain; charset=utf-8`、`application/rss+xml`，内容完整无截断

## 5. 拆登录墙（前端）

- [x] 5.1 `app/web/app/(app)/layout.tsx`：删除 `requireAuth()` 调用（保留 `loadPublicSettings()`），保留 AppShell
- [x] 5.2 `app/web/proxy.ts`：PROTECTED 收窄为结构性私有路径——`/messages`、`/timeline/create`、`/home`，以及**精确**的 `/user`（`^/user/?$`，不含 `/user/{id}`）；同步更新 `matcher`
- [x] 5.3 `app/web/app/(app)/messages/page.tsx` 与 `messages/[id]/page.tsx`：补 `requireAuth`
- [x] 5.4 `app/web/app/(app)/timeline/create/page.tsx`：补 `requireAuth`
- [x] 5.5 `app/web/app/(app)/user/[userId]/page.tsx`：他人主页改为按 `user_profile_auth_required` 判定（设置 `false` 时匿名可读），并输出 `robots` 依 `user_profile_indexable`
- [x] 5.6 `app/web/components/layout/notification-bell.tsx`：`start()` 加登录态门（匿名不建连），消除对受保护 SSE 的重连风暴
- [x] 5.7 验收（逐条匿名 `curl`）：`/timeline`、`/spaces`、`/spaces/{slug}`、`/t/{slug}/{id}`、`/timeline/post?id=`、`/search`、`/leaderboard` → `200`；`/messages`、`/timeline/create`、`/user`、`/home` → `307 /login`
- [x] 5.8 验收：`user/page.tsx`、`user/follow`、`user/favorites`、`user/notifications` 无需改动（已有自守卫），但仍须匿名验证返回 `307`

> 5.9–5.11 是**回填**：首轮实现只拆了登录墙，前端从未读取 `post_auth_required` / `space_view_auth_required`，
> 于是「管理员把设置改回 true」是个改不动前端行为的死配置。补上门控后 spec 该场景才真正成立。

- [x] 5.9 `app/web/lib/auth/guards.ts`：新增 `requireAuthBySetting(settingKey, pathname)`——设置非 `'true'` 时放行（返回可能为 null 的用户），为 `'true'` 时等价 `requireAuth`（跳登录 + 回跳）
- [x] 5.10 把门控接到 5 个公开内容页：`/timeline` 与 `/timeline/post` 与 `/t/{slug}/{id}` 用 `post_auth_required`；`/spaces` 与 `/spaces/{slug}` 用 `space_view_auth_required`
- [x] 5.11 验收（实测双向）：`post_auth_required=true` → `/timeline`、`/timeline/post?id=`、`/t/…` 均 `307 /login` 且回跳保留原路径，而 `/spaces` 仍 `200`（证明两设置独立）；改回 `false` → 全部恢复 `200`

## 6. 规范地址与元数据（前端）

- [x] 6.1 `app/web/app/(app)/t/[slug]/[id]/page.tsx`：从 `redirect()` 改为**直接渲染帖子详情**（复用 `postDetailOptions` + `PostDetail`），返回 200；保留 `id` 缺失时 `redirect('/timeline')`
- [x] 6.2 `app/web/app/(app)/t/[slug]/[id]/page.tsx`：`generateMetadata` 输出自指 canonical 与完整 OG/Twitter
- [x] 6.3 `app/web/app/(app)/timeline/post/page.tsx`：canonical 指向 `/t/{slug}/{id}`（slug 取详情响应），补 `article:published_time` / `article:modified_time`
- [x] 6.4 `app/web/app/(app)/timeline/page.tsx`、`spaces/page.tsx`、`spaces/[slug]/page.tsx`、`search/page.tsx`、`leaderboard/page.tsx`：补 canonical；`search` 页输出 `noindex`
- [x] 6.5 `app/web/app/layout.tsx`：`metadataBase` 改为 `settings.site_url` 优先、`process.env.SITE_URL` 回退；`title.default` 用 `seo_title || site_name`；`description` 用 `seo_description || site_description`；补 `openGraph.siteName`、`og_image`（配置时）、`verification.google` / `other: baiduspider`
- [x] 6.6 验收：匿名抓 `/` 与某帖子页，`curl -s | grep` 确认 `og:title`、`og:image`、`canonical`、`twitter:card` 均存在且非默认值

> 6.7–6.9 是**回填**：首轮把列表页 canonical 写成了「`base` 非空才输出」，于是未配置
> `site_url` 的部署里列表页一条 canonical 都没有（与帖子页同一个错误，帖子页当时已修、
> 列表页漏了）。同时「他人主页开启索引」的 canonical 也依赖 `base`，同样会消失。

- [x] 6.7 `app/web/lib/seo.ts`：`listPageMetadata` 改为**无条件**输出 `alternates.canonical`（`base` 为空时退回相对路径），与帖子页处理一致
- [x] 6.8 `app/web/app/(app)/user/[userId]/page.tsx`：开启索引时无条件输出自指 canonical（不再要求 `base` 非空）
- [x] 6.9 验收（实测）：`site_url` 为空时 `/timeline`、`/spaces`、`/spaces/tech`、`/search`、`/leaderboard` 均输出 canonical；`?tag=go&sort_by=hot`、`?list_view=new`、`?sort_by=most_commented` 三个变体的 canonical 均归一为不带参数的路径；`user_profile_indexable=true` 时他人主页输出 canonical 且无 `noindex`

> 6.10 是**回填**：Next 的 metadata 合并是**浅层**的——页面级 `openGraph` 一旦出现就整体替换根布局那份，
> 于是所有带自己 `openGraph` 的页面都丢了 `og:image` 与 `og:site_name`。实测首页有 `twitter:image` 却没有
> `og:image` 才暴露出来。

- [x] 6.10 `app/web/lib/seo.ts`：`listPageMetadata` 接收并转发 `siteName` / `image`；新增 `siteImage(settings)`（`og_image` → `site_logo` → 空）与 `listPageMetadataFromSettings(settings, {title, path, …})` 便捷入口，5 个列表页改为调用它
- [x] 6.11 `app/web/app/(app)/t/[slug]/[id]/page.tsx` 与 `app/(app)/timeline/post/page.tsx`：`openGraph` 显式转发站点分享图，`twitter.card` 按有无图切换 `summary_large_image` / `summary`
- [x] 6.12 验收（实测）：配 `og_image` 后 `/timeline`、`/t/{slug}/{id}`、`/timeline/post?id=` 三类页面均输出 `og:image` 与 `og:site_name`；清空 `og_image` 而设 `site_logo` → 回退到 logo；两者都空 → 不输出 `og:image`（不产生空标签）

> 6.13 是**回填**：`title.template` 把后缀硬编码成「Feed Chamber」，而每个页面都带自己的 title，
> `title.default` 永远轮不到——于是管理员改 `seo_title` 对首页**毫无效果**，直接违反 spec 的
> 「首页 `<title>` MUST 包含 `seo_title`」场景。

- [x] 6.13 `app/web/app/layout.tsx`：`title.template` 后缀改用配置标题（`seo_title || site_name`），不再硬编码 `Feed Chamber`；`title.default` 去掉重复后缀
- [x] 6.14 `app/web/lib/seo.ts` + `app/(app)/timeline/page.tsx`：新增 `absoluteTitle` 选项（走 Next 的 `title.absolute` 跳过模板）；`/timeline` 作为站点首页时用 `seo_title` 且不叠后缀
- [x] 6.15 验收（实测）：`seo_title=示例论坛` → 首页 `<title>示例论坛</title>`（含设置值且**不重复**）、`/spaces` 为「版面 — 示例论坛」；`seo_title` 清空 → 首页回退「时间线」、其他页后缀回退 `site_name`

## 7. 结构化数据

- [x] 7.1 新建 `app/web/components/seo/json-ld.tsx`：接收对象，`JSON.stringify` 后把 `<` 替换为 `\u003c`，经 `dangerouslySetInnerHTML` 注入；**单文件内注释说明这是 `app/web/AGENTS.md` 的受控例外**（design D6）
- [x] 7.2 `app/web/app/layout.tsx`（或首页）：输出 `WebSite` + `SearchAction`（`/search?q={query}`）
- [x] 7.3 帖子详情（两个入口共用）：输出 `DiscussionForumPosting`（`headline`/`author`/`datePublished`/`dateModified`/`interactionStatistic`/`commentCount`）
- [x] 7.4 版面页与帖子页：输出 `BreadcrumbList`
- [x] 7.5 验收：造一个标题含 `<script>` 的帖子，抓页面确认 JSON-LD 仍可被 `JSON.parse` 解析（转义生效）

> 7.6 是**回填**：`SearchAction` 原本整块 gate 在 `site_url` 非空上，未配站点时首页没有搜索入口——
> 而 spec 要求「首页 MUST 声明站内搜索入口」是无条件的。

- [x] 7.6 `app/web/app/layout.tsx`：`SearchAction` 改为无条件输出；`urlTemplate` 在 `site_url` 为空时退回相对路径（规范偏好绝对 URL，但缺结构化数据的代价更大）。实测：`site_url` 空 → `potentialAction` 存在、`urlTemplate=/search?q=…`；设为 `https://forum.example.com` → `url` 与 `urlTemplate` 均为绝对地址

## 8. 后台可控

- [x] 8.1 `app/web/app/admin/settings/settingsConfig.ts`：SEO 面板补 `seo_description`、`og_image`、`google_site_verification`、`baidu_site_verification`；`seo_title` 的 description 改为「SEO 标题（站点默认标题，帖子页以帖子标题为准）」
- [x] 8.2 `app/web/app/admin/settings/settingsConfig.ts`：新增「他人主页允许搜索引擎索引」布尔项（`user_profile_indexable`，默认关闭）
- [x] 8.3 验收：后台改 `seo_title` → 前台 `<title>` 立即变化；改 `user_profile_indexable` → `/user/{id}` 的 `robots` 与 `robots.txt` 联动变化

## 9. 文档同步

- [x] 9.1 根 `AGENTS.md`：环境变量/模块说明中补充爬虫端点归属（后端独占）与 `user_profile_indexable`
- [x] 9.2 `app/web/AGENTS.md`：路由表标注哪些路径匿名可读；「安全」节补充 JSON-LD 是受控例外
- [x] 9.3 `app/server/internal/handler/AGENTS.md`：`seo.go` 一行更新为 6 个方法（Sitemap/SitemapIndex/SitemapSpaces/SitemapTags/Robots/LatestRSS）
- [x] 9.4 `openapi.json`：`make swag` 后同步（6 条爬虫 GET 端点全部进入 paths）
- [x] 9.5 `docs/api-reference.md`：补「SEO / 爬虫」节（6 个端点 + 归属说明），并从「尚未展开」清单里移除 `seo/realtime`。**此前这条被误标为已完成，实际该文件把 seo 列为未展开、一条都没写**——核对文档与 `router.go` 后补齐

## 10. 全量验证

- [x] 10.1 `cd app/server && go build ./... && go test ./...` 全绿
- [x] 10.2 `cd app/web && pnpm typecheck && pnpm lint` 全绿
- [x] 10.3 `bash feedchamber_test.sh -all` 全绿（后端未改业务逻辑，须无回归）
- [x] 10.4 `node e2e-test.js`：`/timeline/post?id=` 用例 MUST 仍通过（URL 契约未变）
- [x] 10.5 手工验收清单：sitemap 索引 → 各分片 → 抽 3 条 `<loc>` 逐个 `curl -I` 确认 `200`（**不得有 3xx**，这是本轮核心缺陷的回归项）
- [x] 10.6 `openspec validate 2026-10-28-seo-public-indexing --strict` 通过
- [x] 10.7 场景补测：`sitemap_page_size=3` + 10 篇已发布帖 → 索引声明 4 片，逐片取回 3/3/3/1 条、合计 10 条**无重复无遗漏**，第 5 片为合法空 urlset
- [x] 10.8 场景补测：`post_auth_required=true` 且 `space_view_auth_required=true` 时，6 个爬虫端点仍全部 `200`（登录墙不影响爬虫）
- [x] 10.9 场景补测：前端读取的 12 个 SEO 设置键逐一核对，全部同时存在于 `DefaultSettings` 与 `PublicSettingKeys`（消除「读未声明的键」）
- [x] 10.10 场景补测（帖子页）：`/t/{slug}/{id}` 直接 `200` 且无重定向；页面 `<title>`/`description` 取自帖子自身而非站点默认；canonical 自指；`/timeline/post?id=` 与 `/t/…` 两个入口的 canonical **完全相同**；sitemap 中该帖的 `<loc>` 与 canonical 一致且返回 `200`

## 11. 仍需人工确认

> 这三项**无法由实现方完成**：它们需要生产域名、运营决策与站长平台账号。
> 代码侧能做的兜底已做（见 11.1 的启动告警），本节保留为上线检查清单，故不勾选。
> 归档前须由负责人确认或转交。

- [ ] 11.1 上线前确认生产 `site_url` 已配置（否则 canonical 与 sitemap 输出 `localhost:8080`）
  - 已加代码兜底：`server/iris.go` 的 `warnIfSiteURLMissing()` 在 settings 与 `SITE_URL` 都为空时于启动日志告警（只告警不阻断，本地开发不需要该值）。实测：空 → 告警出现；配了 → 无告警
- [ ] 11.2 上线前确认 `post_auth_required` / `space_view_auth_required` 的实际值符合运营预期——**这是访问控制边界变更**，匿名公开后无法靠代码回滚，只能改设置
- [ ] 11.3 在 Google Search Console / Bing Webmaster 提交新 sitemap 索引；`google_site_verification` / `baidu_site_verification` 填真实值
