# Design

## Context

动机见 [proposal.md](./proposal.md) —— 本文件只记录塑造实现路径的现状约束。以下事实经源码实测确认，是实现决策的依据：

| 事实 | 位置 | 对设计的影响 |
| --- | --- | --- |
| `PublicSettingKeys` 不含 `seo_description` | `internal/model/setting.go:233` | `layout.tsx:33` 读的是一个永不存在的键，永远落到 `site_description` |
| `seo_title` 已 seed 且已公开，但零读取 | `setting.go:85`、`layout.tsx` | 该键是死配置，本轮接线 |
| `post_auth_required` / `space_view_auth_required` 默认 `false` | `setting.go` | 后端语义已正确，问题纯在前端 |
| 后端已用 `AuthGate(deps, key)` 按设置动态放行 | `routes_content.go:32-35` | 后端无需改鉴权，只改 sitemap |
| `proxy.ts` PROTECTED 与 `(app)/layout.tsx` 的 `requireAuth` 是两道独立锁 | `proxy.ts:3`、`(app)/layout.tsx:12` | 必须同时拆，只拆一道无效 |
| `messages/*`、`timeline/create` 没有自己的守卫 | 页面源码 | 它们此前依赖 layout 兜底，拆 layout 后必须补 |
| `user/page.tsx`、`user/follow`、`user/favorites`、`user/notifications` 各自调 `requireAuth` | 页面源码 | 拆 layout 后无需改动 |
| `NotificationBell` 挂载即 `start()`，不判登录 | `notification-bell.tsx:51` | 匿名访客会对受保护 SSE 发起重连风暴 |
| `Tag` 模型**无 slug** | `internal/model/tag.go:9-14` | 标签页只能靠 `/timeline?tag=<name>`，sitemap 条目须 URL 编码 |
| `Space` 有 `slug` 与 `is_active` | `internal/model/space.go:15-31` | 版面 sitemap 可按 `slug` 生成、按 `is_active` 过滤 |
| 帖子 `slug` 在创建时生成（标题 + id 后 6 位） | `service/post.go:249-258` | `/t/{slug}/{id}` 是稳定且语义化的规范地址 |
| `pkg.PageMaxSize = 100` | `internal/pkg/settings.go:63` | 经 `POST /posts/list` 枚举全站帖子成本高，故 sitemap 走 repo 直查 |
| vinext 1.3.1 支持 `app/route.ts`、`sitemap.ts`、`robots.ts`、`generateSitemaps` | `node_modules/vinext/dist/server/metadata-routes.js` | 前端**能**实现爬虫端点，故需明确选择不用 |
| nginx 仅转发 `/api/v1/`、`/swagger/`、`/health` 到后端 | `deploy/nginx/nginx.conf:23-49` | 爬虫路径必须显式加 location |

## Goals / Non-Goals

**Goals:**

- 让公开内容对爬虫返回 200，且索引策略与后端设置项保持单一事实来源。
- 让 sitemap 反映真实可索引的内容集合，而不是固定 200 条重定向。
- 让「SEO 设置」在后台改了就真的生效，消除读不存在的键这类静默失效。
- 把爬虫端点与人类页面的归属划清，避免后续在两个包之间来回搬。

**Non-Goals:**

- 不实现逐帖动态 OG 图。`next/og` + satori 在 vinext 下可用，但每次渲染是几百毫秒到秒级，对论坛这种高频 URL 是性能事故；用站点级 `og_image` 兜底。
- 不引入 sitemap 缓存或 CDN 层。当前 sitemap 直查 DB 且量级在数千条，先正确再谈性能；若后续成为热点再单独加缓存。
- 不改变 `/timeline/post?id=` 的 URL 契约（e2e 锁死），只加 canonical。
- 不做 `sitemap-` 的 `images:` / `videos:` 扩展。帖子正文图片不在本轮解析范围。
- 不改 `AuthGate` 的后端行为——它已经正确。

## Decisions

### D1：爬虫端点留在后端，前端不生成任何爬虫文件

**选择**：`seo.go` 继续由后端提供全部 XML/RSS；nginx 加 6 条精确 location。

**替代方案**：迁到前端 `app/sitemap.ts` + `generateSitemaps` + `app/robots.ts`，删除后端 `seo.go`。

**放弃理由**：
1. **枚举成本**。前端要拿全站帖子只能走 `POST /posts/list`，而 `PageMaxSize=100`。5000 帖 = 50 次后端调用，每次都要过完整中间件链。后端直查 DB 用一条 `LIMIT/OFFSET` 只取 `id, slug, updated_at` 就够。
2. **请求上下文风险**。vinext 的元数据路由能否在 RSC 之外拿到 `headers()` 转发 cookie、能否复用 `loadPublicSettings()`，未验证；赌错的代价是 sitemap 整个 500。
3. **归属清晰**。sitemap 的语义是「后端知道哪些内容公开」，`is_active` / `status` 的判定权在后端。

**边界约束（写进 spec）**：XML/RSS 只由后端产出，前端 MUST NOT 生成任何爬虫文件。这条边界防止后续「顺手在前端也加一个」。

### D2：sitemap 分片用 `?page=` 而非路径段

**选择**：`/sitemap-posts.xml?page=2`。

**替代方案**：`/sitemap-posts-2.xml`。

**理由**：`api-style` 已允许 GET 用 query 承载分页；单路由 + 参数比 N 条路由更易注册与测试，且索引里 `?page=` 也是标准写法。已在 `api-style` delta 中把它写成显式场景，避免后人按「POST 参数走 body」把它改掉。

### D3：`/t/{slug}/{id}` 直接渲染，`/timeline/post?id=` 保留

**选择**：两个入口都返回 200，`/timeline/post?id=` 的 canonical 指向 `/t/` 形态。

**替代方案**：让 `/t/` 保持 307，把 sitemap 改成指向 `/timeline/post?id=`。

**放弃理由**：sitemap 里放 `?id=` 这种无语义查询参数，对搜索引擎是弱信号；且 `/t/{slug}/{id}` 的 slug 本来就是为了 SEO 生成的（见 `service/post.go:249`），弃之不用等于白付了生成成本。而 `/timeline/post?id=` 是 e2e 硬契约不能删，故用 canonical 收敛重复内容——这正是 canonical 的标准用途。

### D4：登录墙改为读设置，而非删除

**选择**：`(app)/layout.tsx` 去掉 `requireAuth`；`proxy.ts` PROTECTED 收窄；需要守卫的页面各自 `requireAuth`。

**关键取舍**：`proxy.ts` 跑在边缘、只能读 cookie，**读不到后端设置**（设置是服务端 API）。因此 proxy 只能做「路径级」判断，无法做「设置级」判断。这意味着：

- proxy 保留的只是**永远私有**的路径（`/messages`、`/timeline/create`、`/user` 自身、`/home`）——这些不依赖任何设置。
- 内容页的**设置级**判定必须发生在 RSC 层（`(app)` 子布局或页面内），因为那里能 `await loadPublicSettings()`。

这是两层职责的重新划分：proxy 管「结构性私有」，RSC 管「配置性公开」。不在 proxy 里读设置是刻意的——proxy 每个请求都跑，加一次后端调用会放大延迟。

### D5：`/user/{id}` 默认 `noindex`，开关放设置

**选择**：新增设置 `user_profile_indexable`（默认 `false`），控制 `robots` 与 sitemap 收录。

**理由**：用户主页是 UGC 薄内容，默认索引会稀释站点质量；但这是运营决策不是技术决策，所以做成开关而不是硬编码。默认 `noindex` 同时满足隐私优先与质量优先。

### D6：JSON-LD 走独立组件 + 转义

**选择**：`components/seo/json-ld.tsx`，内容 `JSON.stringify` 后把 `<` 替换为 `\u003c`，再经 `dangerouslySetInnerHTML` 注入。

**为什么需要例外**：`app/web/AGENTS.md` 规定「不要新增 `dangerouslySetInnerHTML` 路径」，唯一出口是 `markdown-renderer.tsx`。JSON-LD 必须是内联 `<script type="application/ld+json">`，React 无法用普通 props 表达脚本内容。因此这是**受控例外**：封装在单一组件内、输入是结构化对象而非字符串拼接、且强制转义 `</` 序列防止提前闭合脚本块。spec 里已把「正文含 `<` 时仍为合法 JSON」写成可测场景。

### D7：`lastmod` 直接用 `updated_at`

**选择**：不改数据模型，不引入「内容实质修改时间」列。

**理由**：当前 `updated_at` 会因计数器变动（浏览/点赞）而更新，理论上会让 `lastmod` 抖动。但：① 抖动只降低爬虫重抓效率，不影响正确性；② 引入新列需要三库迁移 + 所有写路径埋点，成本远超收益；③ Google 已明确表示 `lastmod` 只是弱信号。YAGNI。

### D8：`metadataBase` 取设置而非仅环境变量

**选择**：`settings.site_url` 优先，`process.env.SITE_URL` 回退。

**理由**：后台已有 `site_url` 设置项且已公开，管理员改它却对 OG 绝对 URL 无效是反直觉的。环境变量作为回退保留给未配置设置的部署。

## Risks / Trade-offs

| 风险 | 缓解 |
| --- | --- |
| **匿名公开是访问控制边界变更**：原本必须登录才能看的内容，升级后匿名可读。若运营方以为「默认私有」，会是一次意外的信息暴露 | 这是让前端对齐后端既有语义（`post_auth_required` 默认 `false` 且 `AuthGate` 早已放行），不是新增能力。proposal 已标为破坏性变更；上线前须确认 `post_auth_required` / `space_view_auth_required` 的实际值符合预期。若需保持私有，把设置改 `true` 即可，无需回滚 |
| **拆 layout 守卫后漏补页面**：某页面此前靠 `(app)/layout` 兜底，拆掉后变成匿名可访问的私有页 | 已逐一清点：`messages/*` 与 `timeline/create` 必须补；`user/page.tsx`、`user/follow`、`user/favorites`、`user/notifications` 已有自守卫无需动。tasks 里列为独立验收项，并用匿名请求逐条验证 |
| **`/t/{slug}/{id}` 改为直接渲染后 slug 不匹配仍返回 200**：爬虫可用任意 slug 访问同一帖子，产生重复 URL | canonical 指向正确 slug 的地址，收敛重复。不做 301 重定向到规范 slug——那会让 sitemap 之外的链接多一跳，且 slug 变更时产生重定向链 |
| **sitemap 分片在帖子数变化时页码漂移**：索引声明的分片数与实际内容随时增长 | 索引每次请求实时计算分片数，不缓存。代价是每次索引请求一次 `COUNT(*)`，量级可接受 |
| **`?page=` 超出范围返回空 urlset**：爬虫可能请求不存在的分片 | 返回合法空 `<urlset>` 而非 404——404 会让爬虫把整个 sitemap 判为失效 |
| **JSON-LD 转义遗漏导致 XSS** | 转义发生在单一组件内且强制替换 `<`；spec 有对应场景；代码评审时该文件是唯一需要看的点 |
| **nginx 精确 location 与前端路由冲突**：若未来前端也加了 `/sitemap.xml` 路由，nginx 会遮蔽它 | 这正是 spec 里「爬虫端点由后端独占」要防的；该约束使冲突成为违规而非事故 |
| **`robots.txt` 加 `Disallow: /user/` 会连带屏蔽 `/user/{id}`**，即使管理员开启了索引 | 开关开启时 `robots.txt` MUST 不输出该条 Disallow。这条联动容易漏，tasks 里单列 |

## Migration Plan

1. **后端先行**：新 sitemap 路由与设置键可独立上线，对现网无影响（旧 sitemap 端点仍在，只是内容变正确）。
2. **部署层**：nginx 加 location。此步单独可回滚——移除 6 条 location 即回到现状（404），不影响其他流量。
3. **前端**：拆登录墙 + `/t/` 直接渲染 + 元数据。这是唯一有用户可见行为的步骤。
4. **回滚策略**：若匿名公开引发问题，改 `post_auth_required=true` / `space_view_auth_required=true` 即恢复登录墙，**无需回滚代码**——这是把策略做成设置项的直接收益。
5. **上线前检查**：确认 `site_url` 已配置（否则 canonical 与 sitemap 会输出 `localhost`）；确认 `seo_description` 已填（否则回退 `site_description`）。

## Open Questions

无。影响 spec、方案与任务拆分的未知项已在设计阶段全部解析；剩余项（sitemap 缓存、动态 OG 图）已明确列为 Non-Goals，可在后续独立提案中处理。
