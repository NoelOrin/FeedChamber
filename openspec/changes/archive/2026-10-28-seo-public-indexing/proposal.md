## Why

Feed Chamber 的 SEO 能力目前**整体失效**，不是「优化不足」而是「一条都抓不到」。四个缺陷叠加，任何一个单独存在都足以让索引为零：

1. **所有内容页对爬虫返回 307 到 `/login`。** 前端 `proxy.ts` 把 `/timeline|spaces|t|user|...` 硬编码为 PROTECTED，`(app)/layout.tsx` 再用 `requireAuth()` 二次拦截。而后端 `post_auth_required` / `space_view_auth_required` 默认 `false` 且 `AuthGate` 已按设置动态放行——**后端本来就是对的，是前端单方面加的登录墙**。这意味着设置项在后台改了也不生效，前端不读它。
2. **`/sitemap.xml`、`/robots.txt` 在生产环境返回 404。** 这两个端点只注册在后端根 mux（`app.Get`），而 `deploy/nginx/nginx.conf` 只把 `/api/v1/`、`/swagger/`、`/health` 转给后端，其余全部落到前端，前端没有对应路由。
3. **sitemap 内容是错的。** 它只输出 `/t/{slug}/{id}`，而该路由现在只是 `redirect()` 到 `/timeline/post?id=`——sitemap 里全是 307，且固定 200 条无分页，无版面页、无标签页。
4. **元数据接线断了。** `seo_title` 已 seed 且已公开，但**没有任何代码读它**；`seo_description` 被 `layout.tsx` 读，却既不在 `PublicSettingKeys` 也不在 `DefaultSettings`——读一个不存在的键，永远落到 `site_description`。

## What Changes

- **索引策略改为「按后端设置项决定」，前端不再单方面加登录墙。** 拆掉 `(app)/layout.tsx` 的 `requireAuth()`，把 `proxy.ts` 的 PROTECTED 收窄为真正私有的路径（`/messages`、`/timeline/create`、`/user` 精确自身主页、`/home`）。已有自守卫的页面（`user/page.tsx`、`user/follow`、`user/favorites`、`user/notifications`）保持不动；`messages/*`、`timeline/create` 补自己的 `requireAuth`，因为它们此前只靠 layout 兜底。他人主页 `/user/{id}` 改为按 `user_profile_auth_required` 判定。
- **匿名访客不再触发受保护 SSE。** `NotificationBell` 在匿名时仍会挂载即 `start()`，对受保护端点发起重连风暴；加登录态门。
- **`/t/{slug}/{id}` 从重定向改为直接渲染帖子详情**，返回 200。`/timeline/post?id=` 保留（e2e 锁死），但两处 canonical 统一指向 `/t/` 形态。
- **sitemap 重写为分片 + 分页**：`sitemap.xml` 为索引，`sitemap-posts.xml?page=N` 每片 5000 条，新增 `sitemap-spaces.xml`（启用版面）、`sitemap-tags.xml`（标签）。条目 URL 一律指向最终 200 地址。
- **`robots.txt` 补齐**：新增 `Disallow: /messages`、`/timeline/create`、`/user/`，保留 sitemap 声明。
- **nginx 增加 6 条精确 location**，把 4 个 sitemap 分片、`robots.txt`、`latest.rss` 转给后端。
- **元数据补全**：`seo_description` / `og_image` / `google_site_verification` / `baidu_site_verification` 进入 `DefaultSettings` 与 `PublicSettingKeys`；`metadataBase` 优先取 `site_url` 设置；`title.default` 用 `seo_title`；补 `og:site_name`、`og:image`、`article:published_time/modified_time`、canonical、`verification`。
- **结构化数据**：全站 `WebSite` + `SearchAction`，帖子页 `DiscussionForumPosting`，版面/帖子页 `BreadcrumbList`。
- **后台可控**：SEO 设置面板补 `seo_description`、`og_image`、两个 verification 键，并新增「他人主页是否允许索引」开关（默认关闭）。

## Capabilities

### New Capabilities

- `seo`: 搜索引擎可见性的唯一权威规范——索引策略与登录墙的边界、爬虫端点（sitemap/robots/RSS）的形状与归属、canonical URL、结构化数据、页面元数据来源、以及部署层爬虫路由。

### Modified Capabilities

- `api-style`: 系统端点清单新增 `sitemap-spaces.xml`、`sitemap-tags.xml`，并明确 sitemap 分页参数 `?page=` 属于「GET 允许 query 承载过滤与分页」的既有例外。

## Impact

- **后端**：`internal/handler/seo.go`（重写）、`internal/repository/post.go`（新增 sitemap 专用轻量查询）、`internal/model/setting.go`（`DefaultSettings` + `PublicSettingKeys` + 新设置常量）、`internal/router/router.go`（新增两条 sitemap 路由）。
- **前端**：`proxy.ts`、`app/(app)/layout.tsx`、`app/(app)/t/[slug]/[id]/page.tsx`、`app/(app)/timeline/post/page.tsx`、`app/(app)/messages/*`、`app/(app)/timeline/create/page.tsx`、`app/(app)/user/[userId]/page.tsx`、`app/layout.tsx`、`components/layout/notification-bell.tsx`、`components/seo/json-ld.tsx`（新）、`app/admin/settings/settingsConfig.ts`。
- **部署**：`deploy/nginx/nginx.conf` 新增 6 条 location。
- **破坏性变更（对外可见）**：匿名访客首次可以读取 `/timeline`、`/spaces`、`/t/**`。这**改变的是访问控制边界**，不是渲染逻辑——后端 `AuthGate` 与 `post_auth_required` 一直是这个语义，本轮只是让前端停止单方面加锁。管理员若想恢复登录墙，改设置即可，无需回滚代码。
- **兼容性**：`/timeline/post?id=` 与所有既有 URL 不变（e2e 锁死）；`/t/{slug}/{id}` 的 URL 不变，仅响应从 307 变 200。
- **不在本轮范围**：逐帖动态 OG 图（`next/og` + satori 渲染成本）、Core Web Vitals、站内搜索质量、多语言 hreflang、`/user/{id}` 之外的 UGC 页面索引策略。
