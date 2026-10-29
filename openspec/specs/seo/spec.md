# seo Specification

## Purpose
定义 OrinForum 面向搜索引擎的可见性契约：哪些内容页对匿名访客与爬虫开放（以及该策略如何由服务端设置驱动）、爬虫端点（sitemap / robots / RSS）的归属与形状、规范地址（canonical）的归一规则、页面元数据的取值来源、结构化数据的输出要求，以及他人主页等 UGC 页面的索引开关。目的是让「站点内容可被正确收录」成为可测试、可配置的行为，而不是依赖前端硬编码。
## Requirements
### Requirement: 内容页索引策略由服务端设置决定

公开内容页（时间线、版面总览、单版面、帖子详情）是否要求登录 MUST 由服务端设置决定，MUST NOT 由前端硬编码。页面级设置项为 `post_auth_required`（时间线与帖子详情）与 `space_view_auth_required`（版面总览与单版面），取值 `true` 时要求登录、`false`（默认）时允许匿名读取。

`comment_auth_required` 是**评论接口级**设置（作用于 `POST /posts/{postId}/contents/list` 的 `AuthGate`），不由页面级登录门消费；页面级守卫 MUST NOT 因为该设置而拦截整页。

后端与前端 MUST 对同一设置得出一致结论：设置项为 `false` 时，匿名访客访问公开内容页 MUST 返回 `200` 且渲染完整内容，MUST NOT 返回重定向或 `401`/`403`。设置项为 `true` 时，匿名访客 MUST 被重定向到登录页并携带原路径回跳参数。

两个设置项 MUST 相互独立：只把 `post_auth_required` 置 `true` 时，版面页 MUST 仍对匿名访客可读，反之亦然。

页面级登录守卫 MUST 只覆盖真正的私有面：私信、发帖入口、用户中心（我的主页 / 关注 / 收藏 / 通知）与聚合首页。公开内容面 MUST NOT 额外叠加页面级守卫。

#### Scenario: 匿名访问公开内容页

- WHEN `post_auth_required=false` 且匿名访客请求 `/timeline`
- THEN 系统 MUST 返回 `200` 并渲染帖子列表，MUST NOT 重定向到 `/login`

#### Scenario: 匿名访问帖子详情

- WHEN `post_auth_required=false` 且匿名访客请求 `/t/{slug}/{id}`
- THEN 系统 MUST 返回 `200` 并渲染帖子正文，MUST NOT 重定向

#### Scenario: 管理员恢复登录墙

- WHEN 管理员将 `post_auth_required` 改为 `true`
- THEN 匿名访客请求 `/timeline` MUST 被重定向到 `/login`，且回跳参数 MUST 保留原路径

#### Scenario: 两个页面级设置相互独立

- WHEN 仅 `post_auth_required=true` 而 `space_view_auth_required=false`
- THEN 匿名访客请求 `/timeline` MUST 被重定向，而 `/spaces` 与 `/spaces/{slug}` MUST 仍返回 `200`

#### Scenario: 设置恢复为 false 后重新公开

- WHEN 管理员把 `post_auth_required` 从 `true` 改回 `false`
- THEN 匿名访客请求 `/timeline` MUST 恢复 `200`，MUST NOT 继续被重定向

#### Scenario: 私有面始终要求登录

- WHEN 匿名访客请求 `/messages`、`/timeline/create`、`/user`（自身主页）或 `/user/notifications`
- THEN 系统 MUST 重定向到 `/login`，与任何内容设置无关

### Requirement: 爬虫端点由后端独占提供

`sitemap.xml`（索引）、`sitemap-posts.xml`、`sitemap-spaces.xml`、`sitemap-tags.xml`、`robots.txt`、`latest.rss` MUST 由后端提供，MUST NOT 由前端生成任何爬虫文件。部署层 MUST 把这 6 条路径路由到后端，MUST NOT 落到前端。

所有爬虫端点 MUST 可在未认证状态下访问，MUST NOT 因 `post_auth_required` 等设置而失效——爬虫不携带会话。

#### Scenario: 生产环境可取得 sitemap

- WHEN 未认证客户端经部署层请求 `/sitemap.xml` 与 `/robots.txt`
- THEN 系统 MUST 返回 `200` 且 `Content-Type` 分别为 `application/xml` 与 `text/plain`

#### Scenario: 登录墙不影响爬虫端点

- WHEN `post_auth_required=true`
- THEN `/sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt` 与 `/latest.rss` MUST 仍返回 `200`

### Requirement: Sitemap 结构与分页

`sitemap.xml` MUST 是 sitemap 索引，声明全部子 sitemap。帖子 sitemap MUST 支持 `?page=N` 分片，单片条目数 MUST NOT 超过 5000（协议上限）。索引 MUST 声明的分片数与实际可访问分片一致。

每条 `<url>` MUST 包含 `<loc>` 与 `<lastmod>`。`<loc>` MUST 指向返回 `200` 的最终地址，MUST NOT 指向任何重定向地址。`<lastmod>` MUST 为 RFC 3339 的 UTC 时间。

sitemap MUST 只包含公开可见内容：帖子 MUST 限于 `approved` 与 `closed` 状态，版面 MUST 限于启用状态。

#### Scenario: sitemap 不含重定向 URL

- WHEN 爬虫读取 `sitemap-posts.xml` 的任一 `<loc>`
- THEN 该 URL MUST 返回 `200`，MUST NOT 返回 `3xx`

#### Scenario: sitemap 分片

- WHEN 帖子数超过单片上限
- THEN 索引 MUST 声明多个分片，且每个 `?page=N` MUST 可独立取回该片条目

#### Scenario: sitemap 排除非公开内容

- WHEN 存在 `pending` 状态的帖子或被禁用的版面
- THEN 它们 MUST NOT 出现在任何 sitemap 中

### Requirement: Canonical URL 与 URL 归一

每个可索引页面 MUST 输出唯一 canonical URL，指向该内容的规范地址。同一内容的多个入口 MUST 共用同一个 canonical。

帖子详情的规范地址 MUST 为 `/t/{slug}/{id}`。以查询参数形式访问的同一内容 MUST 通过 canonical 指向该规范地址。规范地址本身 MUST 返回 `200`，MUST NOT 通过重定向跳转到其他地址。

#### Scenario: 查询参数入口的 canonical

- WHEN 爬虫请求 `/timeline/post?id={id}`
- THEN 页面 MUST 输出指向 `/t/{slug}/{id}` 的 canonical

#### Scenario: 规范地址直接渲染

- WHEN 爬虫请求 `/t/{slug}/{id}`
- THEN 系统 MUST 返回 `200` 并渲染正文，canonical MUST 指向自身

#### Scenario: 列表页 canonical

- WHEN 爬虫请求带分页或筛选参数的时间线
- THEN 页面 MUST 输出 canonical，且 MUST NOT 因参数组合产生重复的规范地址

### Requirement: 页面元数据来源

站点级元数据 MUST 取自 settings 表：标题取 `seo_title`（缺失回退 `site_name`）、描述取 `seo_description`（缺失回退 `site_description`）、关键词取 `seo_keywords`。这些键 MUST 可被未认证客户端读取，且 MUST 在默认设置中预先存在，MUST NOT 出现「代码读取了未声明的设置键」的情况。

站点基准 URL MUST 优先取自 `site_url` 设置，缺失时回退部署环境变量。

帖子详情页 MUST 输出内容自身的标题、摘要与规范化 URL，MUST NOT 使用站点级标题覆盖。

页面 MUST 输出 Open Graph 与 Twitter 卡片元数据，包含标题、描述、类型与站点名；站点级配置了分享图时 MUST 输出 `og:image`。

#### Scenario: 站点级元数据取自设置

- WHEN 管理员将 `seo_title` 设为「示例论坛」
- THEN 首页 `<title>` MUST 包含「示例论坛」，MUST NOT 使用硬编码的默认名

#### Scenario: 未声明的设置键不存在

- WHEN 系统以默认设置启动
- THEN 所有被代码读取的 SEO 设置键 MUST 在默认设置中存在，MUST NOT 存在读取即落空回退的键

#### Scenario: 帖子页使用自身元数据

- WHEN 爬虫请求某帖子详情页
- THEN 标题与描述 MUST 来自该帖子，MUST NOT 为站点级默认值

### Requirement: 结构化数据

站点首页 MUST 输出 `WebSite` 结构化数据并声明站内搜索入口。帖子详情页 MUST 输出 `DiscussionForumPosting` 结构化数据，包含标题、作者、发布时间、更新时间、互动计数与评论数。版面页与帖子页 MUST 输出 `BreadcrumbList`。

结构化数据 MUST 以 JSON 内嵌，且序列化结果 MUST 转义 `<` 字符，MUST NOT 允许正文内容中断脚本块。

#### Scenario: 帖子页结构化数据

- WHEN 爬虫请求帖子详情页
- THEN 页面 MUST 包含 `DiscussionForumPosting`，且 `datePublished` 与帖子创建时间一致

#### Scenario: 正文不得逃逸脚本块

- WHEN 帖子标题或正文包含 `<` 字符
- THEN 结构化数据 MUST 仍为合法 JSON，MUST NOT 提前闭合脚本块

### Requirement: 他人主页索引开关

他人主页 `/user/{id}` MUST 默认不允许被搜索引擎索引。管理员 MUST 可通过设置项开启索引。

允许索引时，该页面 MUST 输出自指 canonical 并可出现在 sitemap 中；不允许索引时，页面 MUST 输出 `noindex` 指令且 MUST NOT 出现在任何 sitemap 中。

#### Scenario: 默认不索引

- WHEN 系统以默认设置运行且爬虫请求 `/user/{id}`
- THEN 页面 MUST 输出 `noindex`，且该 URL MUST NOT 出现在 sitemap 中

#### Scenario: 管理员开启索引

- WHEN 管理员开启他人主页索引
- THEN 该页面 MUST 输出可索引指令与自指 canonical
