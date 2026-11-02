# API 端点参考

> 本文件是 Feed Chamber 后端 HTTP 接口的端点快照，按模块分组罗列。
> **权威来源**为仓库根目录的 `openapi.json` 与运行时的 `/swagger/` 页面，由 handler 上的 swag 注解经 `make swag` 生成。本文件便于离线速查，若与 `openapi.json` 不一致，以 `openapi.json` 为准。
> API 风格约定（全部 RPC 风格 `POST`、动词后缀 URL、body 传参、统一响应）见根目录 [AGENTS.md](AGENTS.md) 的「API 风格约定」。

## API 端点总览 (速查)

> `/metrics` 不在本树的端点里：Prometheus 指标由 `METRICS_PORT`(9091) 的独立 listener 暴露，不经 nginx（见 `app/server/internal/metrics/AGENTS.md`）。

> 本清单仅为常见端点速查，**不完整**（实测约 224 个 POST + 31 个 GET）。其中 `GET` 均为只读查询或协议/爬虫/文件端点——方法规则见 [openspec/specs/api-style](../openspec/specs/api-style/spec.md)（写操作 `POST`，只读查询 MAY `GET`）。完整清单以 `router.go` 与 `openapi.json` 为准。以下模块尚未在此展开：metrics/dashboard、groups、mfa、pm、watches、user-blocks、drafts、revisions、trash、backups、webhooks、plugins、badges、theme、leaderboard、invite、security、sessions、api-keys、realtime、oauth provider、sso。

### Public
- `GET /api/v1/health` — 健康检查
- `POST /api/v1/settings/public` — 获取公开设置

### Auth (`POST /api/v1/auth`)
- `/register` — 注册 (rate-limited)
- `/login` — 登录 (rate-limited)
- `/forgot_password` — 忘记密码 (rate-limited)
- `/reset_password_by_code` — 验证码重置密码 (rate-limited)
- `/mfa/verify` — TOTP 第二步校验 (rate-limited)

> `/auth/refresh`、`/auth/logout`、`/auth/change_password` 等需要凭证，列在下方 Protected 段。曾经的 `/auth/refresh_token` 已随会话化改造更名为 `/auth/refresh`（语义也变为单用轮换）。

### Public 查询 (`POST /api/v1`)
- `/tags/list` — 标签列表
- `/spaces/tree` — 版面树形列表
- `/spaces/{space_id}` — 版面详情
- `/announcements/published` — 已发布公告列表
- `/announcements/{id}` — 公告详情 (GET)

### Post (public, 可选登录 + 动态门控)
- `POST /api/v1/posts/list` — 帖子列表
- `POST /api/v1/posts/search` — 搜索帖子
- `GET /api/v1/posts/{id}` — 获取帖子
- `GET /api/v1/posts/{id}/detail` — 帖子详情
- `POST /api/v1/posts/{postId}/contents/list` — 评论列表
- `POST /api/v1/posts/{postId}/tags` — 帖子标签

### OAuth (`GET /api/v1/oauth`)
- `/providers` — 获取所有 OAuth 配置
- `/{provider}/auth` — 获取授权 URL
- `/{provider}/callback` — OAuth 回调处理
- `/{provider}/config` — 获取单个 OAuth 配置

### Protected (`POST /api/v1` + 会话 sid 或 API Key)
- `/auth/logout` — 登出
- `/auth/refresh` — 会话轮换（签发新 sid 并作废旧 sid；Bearer 客户端必须采纳响应里的新 `access_token`）
- `/auth/change_password` — 修改密码
- `/auth/first_change_password` — 首次修改密码
- `/dashboard` — 首页数据聚合（用户资料+关注数+公告）

### OAuth (Protected) (`POST /api/v1/oauth`)
- `GET /accounts` — 获取绑定账户
- `/{provider}/link` — 绑定账号
- `/{provider}/unlink` — 解绑账号

### Post (Protected) (`POST /api/v1/posts`)
- `/batch` — 批量获取帖子（需登录）
- `/create` (perm:post:create)
- `/{id}/update` (perm:post:create) — service 层所有权检查
- `/{id}/delete` (perm:post:create) — service 层所有权检查
- `/{id}/pin` (perm:post:pin)
- `/{id}/lock` (perm:post:lock)
- `/{postId}/tags/set` (perm:post:create)
- `/{postId}/contents/create` (perm:comment:create)
- `/{postId}/contents/{id}/update` (perm:comment:create) — service 层所有权检查
- `/{postId}/contents/{id}/delete` (perm:comment:create) — service 层所有权检查
- `/{postId}/favorite`
- `/{postId}/favorite/status`
- `/comments/list` (perm:comment:delete) — 全局评论列表

### User (`POST /api/v1/user`)
- `/profile` (GET) — 获取/更新资料
- `/profile/update` — 更新资料
- `/search` (perm:user:read)
- `/list` (perm:user:read)
- `/{id}` (GET, perm:user:read)
- `/{id}/delete` (perm:user:delete)
- `/{id}/role/update` (perm:user:update)
- `/{id}/banned` (perm:user:update) — 禁言
- `/{id}/unbanned` (perm:user:update) — 解禁
- `/{id}/banned/status` — 禁言状态
- `/{id}/banned/overview` — 禁言历史
- `/{id}/space/{space_id}/banned` (perm:user:update) — 版面禁言
- `/{id}/space/{space_id}/unbanned` (perm:user:update) — 版面解禁
- `/{id}/space/{space_id}/banned/status` — 版面禁言状态

**注意**：用户标识统一使用 `User.ID`（ULID）。前端路由参数 `userId` 直接映射到 `User.ID`。Dashboard 的 `uuid` 查询参数已迁移为 `id`。

### Role (`POST /api/v1/role`)
- `/list` (perm:role:read)
- `/create` (perm:role:manage)
- `/{id}/delete` (perm:role:manage)

### Permission (`POST /api/v1/permission`)
- `/list` (perm:role:read)
- `/role` (perm:role:read) — 获取角色权限
- `/sync` (perm:role:manage) — 同步角色权限

### Tag (`POST /api/v1/tags`)
- `/create` (perm:post:create)
- `/{id}/update` (perm:post:create)
- `/{id}/delete` (perm:post:delete)

### Content (`POST /api/v1/contents/{contentId}`)
- `/like` — 点赞/取消点赞
- `/like/status` — 查询点赞状态
- `/replies` — 获取回复列表

### Favorites (`POST /api/v1/favorites`)
- `/list` — 用户收藏列表

### Follow (`POST /api/v1/users/{userId}`)
- `/follow` — 关注
- `/unfollow` — 取消关注
- `/follow/status` — 查询关注状态
- `/followers` — 获取粉丝列表
- `/following` — 获取关注列表
- `/counts` — 获取关注数/粉丝数

### Upload (`POST /api/v1/upload`)
- `/` — 上传文件
- `/presign` — 获取预签名签名
- `/confirm` — 确认上传
- `/list` — 文件列表
- `/{id}` — 获取文件 (GET)
- `/{id}/delete` — 删除文件

### Notification (`POST /api/v1/notifications`)
- `/list` — 通知列表
- `/unread_count` — 未读数
- `/{id}/read` — 标记已读
- `/read_all` — 全部标记已读
- `/{id}/delete` — 删除通知
- `/stream` — SSE 实时推送未读数

### Report (`POST /api/v1/reports`)
- `/create` (perm:report:create)
- `/list` (perm:report:manage)
- `/{id}/status` (perm:report:manage)
- `/{id}/delete` (perm:report:manage)

### Motion — 社区自治动议 (`POST /api/v1/motions`)

与举报并行、互不共享状态机：举报=违规上行→管理员；动议=灰区→社区信誉加权集体裁判。
结果全部柔性（fold 折叠降权 / watch 观察 / escalate 生成 Report / exile 暗区软隔离 / reverse 翻案），
硬封禁与封号仍是版主/管理员的裁量升级，不由动议状态机自动执行。

- `/create` (perm:motion:create) — 发起 fold/watch/escalate/exile/reverse
- `/list` (perm:motion:create) — 分页列表，body 带 `status` / `target_type`
- `GET /{id}` (perm:motion:create) — 详情
- `/{id}/vote` (perm:motion:create) — `{side: approve|reject}`，票权 `1+min(reputation/1000,1)` 上限 ×2，一人一票
- `/{id}/resolve` (perm:motion:manage) — 开放期满/达法定人数后结算
- `/{id}/status` (perm:motion:manage) — 翻案 `status=overturned` / 标记滥诉 `abuse` / 升级硬封禁 `hard_ban`（用户类）
- `/{id}/delete` (perm:motion:manage) — 发起者撤回（仅 pending，service 校验发起者）

暗区：系统版面 `slug=dark-zone`（`system=true`、`recommendable=false`），seed 幂等确保；被 exile 的帖子
`posts.exiled=true` 并路由到该版面，主推荐流据此过滤，版面内仍可访问、可翻案。

### Check-in (`POST /api/v1/checkin`)
- `/` — 每日签到
- `GET /status` — 签到状态
- `GET /history` — 签到历史

### Space (`POST /api/v1/spaces`)
- `POST /tree` `/flat` `/roots` `/{space_id}` 等 — 公开读（可选登录 + gate）
- `/create` — 创建（protected，无路由级权限码）
- `/reorder` — 排序
- `/{space_id}/update`
- `/{space_id}/delete`
- `/{space_id}/status` — 启用/禁用
- `/{space_id}/move` — 移动

### Comments (admin) (`POST /api/v1/comments`)
- `/{id}/delete` (perm:comment:delete) — 管理员删除评论

### Admin (`POST /api/v1/admin`) — all perm:role:manage
- `/stats` — 系统统计
- `/dashboard` — 运营概览（聚合统计 + 待处理举报 + 近期动态）
- `/search` — 全局搜索（支持各实体独立分页）
- `/user/{id}/activities` — 用户活动记录
- `/batch/users` — 批量用户操作（delete / set_role）
- `/batch/posts` — 批量帖子操作（delete / pin / unpin / lock / unlock）
- `/batch/comments` — 批量评论删除
- `/reconcile/counters` — 计数器修复
- `/banned/list` — 禁言记录列表
- `/announcements/list` — 公告列表（含草稿）
- `/announcements/create` — 创建公告
- `/announcements/{id}/update` — 更新公告
- `/announcements/{id}/delete` — 删除公告
- `/announcements/{id}/publish` — 发布公告
- `/announcements/{id}/unpublish` — 撤回公告
- `/settings` — 获取所有设置
- `/settings/batch` — 批量设置
- `/settings/{key}` — 获取设置
- `/settings/{key}/update` — 更新设置
- `/settings/{key}/delete` — 删除设置

### SEO / 爬虫（app 根，**不在** `/api/v1` 下）

> 这 6 个端点由后端独占，部署层用精确 `location =` 转发（见 `deploy/nginx/nginx.conf`）。
> 前端不生成任何爬虫文件。索引策略由 settings 控制，规范见 `openspec/specs/seo`。

- `GET /sitemap.xml` — sitemap 索引（声明帖子分片 + 版面 + 标签）
- `GET /sitemap-posts.xml?page=N` — 帖子 sitemap 分片（单片上限由 `sitemap_page_size` 控制，协议上限 50000）
- `GET /sitemap-spaces.xml` — 启用版面的 sitemap
- `GET /sitemap-tags.xml` — 标签页 sitemap（Tag 无 slug，用 `/timeline?tag=<name>`）
- `GET /robots.txt` — 允许抓取公开内容，屏蔽私有面与 API；`user_profile_indexable=false` 时额外屏蔽 `/user/`
- `GET /latest.rss` — 最近 20 条公开帖子的 RSS 2.0

### SSE
- `POST /api/v1/notifications/stream` — 通知未读数实时流

## 成长体系（XP / 等级）

- `GET /api/v1/levels` — 公开等级名称表（前端渲染等级名 / 升级进度；无需登录）
- `/api/v1/admin/levels` — `POST /list`、`/save`、`/{level}/delete`（`level:manage`）
- `/api/v1/admin/xp-rules` — `POST /list`、`/save`、`/streak/list`、`/streak/save`、`/streak/{days}/delete`（`level:manage`）
- `/api/v1/admin/trust/recompute` — 全量重算 XP / 等级（`level:manage`）
- `/api/v1/admin/users/{id}/xp` — `GET` 指标分解（`level:manage`）
- `/api/v1/admin/users/{id}/xp/adjust`、`/xp/adjustments/list`、`/api/v1/admin/xp-adjustments/{id}/delete` — 人工调整（`level:manage`）
- `POST /api/v1/user/{id}/trust_level/update` — 手工设级 / 解除锁定（body `{level, manual?}`）
- `POST /api/v1/posts/{id}/feature` — 设为 / 取消精华（body `{featured}`，`report:manage`）

## 门控规则引擎

- `/api/v1/admin/gates` — `POST /meta`、`/list`、`/save`、`/{id}/delete`、`/defaults/list`、`/defaults/save`、`/simulate`（`gate:manage`）

## 可见性

- `spaces.visibility`（public/login/level）+ `spaces.min_level`，`posts.min_level` 单帖覆盖；列表 / 搜索 / sitemap / RSS / 批量统一过滤，详情不足返回 403「需要等级 N，当前等级 M」。
- `POST /api/v1/spaces/{space_id}`（详情）在不可见时同样返回 403；`POST /spaces/create`、`/spaces/{space_id}/update` 接受 `visibility` / `min_level`。
