# Discourse 能力基线文档（Feed Chamber 对齐版）

> 基准：Discourse 3.3/3.4 开源版核心能力（以官方 docs、meta.discourse.org 与源码插件能力为基线，排除 Enterprise/AI 付费插件）。用途：作为 Feed Chamber“基本能力对齐”的唯一权威基线，明确哪些能力必须对齐、哪些为可选增强。

日期：2026-08-21  
维护者：Feed Chamber  
前置审查：历轮对齐审查整合（48+ agent 任务，结论已沉淀进本基线文档与 `reviews/phase4-discourse-alignment-retro.md`）

## 1. 判定口径

- **必须对齐（MUST）**：Discourse 单站开箱即用的核心论坛能力；缺失则不能称为“论坛基本能力对齐”。
- **建议对齐（SHOULD）**：Discourse 通过官方插件/主题组件或配置实现的高频能力；按优先级逐步对齐。
- **可选（MAY）**：Discourse 生态扩展或重型能力（如独立全文检索引擎、Onebox 全量、Polls 全特性），不阻断基本对齐验收。
- **不对齐**：与 Feed Chamber 定位无关或需重架构的能力，明确不纳入本基线。

## 2. 能力基线总览（按域）

### A. 内容与结构

| 能力 | Discourse 基线 | Feed Chamber 现状 | 结论 |
|---|---|---|---|
| Topic/Post 二级模型 | Topic（标题+分类+标签+状态）+ Post 流（分页/小窗）| Post（标题/slug/space_id/状态机）+ Content（楼层流，支持 parentId 嵌套） | MUST 已对齐（术语不同：Post≈Topic，Content≈Post） |
| 分类（Category） | 树形分类、权限、排序、颜色/图标、子分类 | Space 版面（树形 `tree`、`slug`、排序、启禁）+ Category 兼容层 | MUST 已对齐（以 Space 为主） |
| 标签（Tag） | 标签、标签组、帖子多对多、标签权限 | Tag 多对多、`SetPostTags`、按名称批量建标签 | MUST 已对齐 |
| 状态机 | pinned / closed / archived / unlisted / visible | pinned（含 `PinOrder`/`PinnedAt`/`PinnedExpireAt`）、locked、status=`approved/pending/rejected/closed` | MUST 已对齐（archived/unlisted 归为 moderated 扩展） |
| 草稿（Draft） | 服务端草稿、自动保存、按用户/类型去重 | `drafts` 表 + `/drafts/save|list|{id}/delete`（SQL/Mongo）| MUST 已对齐 |
| 版本历史（Revision） | 帖子编辑历史、diff、恢复 | `revisions` 快照 + 帖子/评论列表与恢复接口（SQL/Mongo） | MUST 已对齐（diff 为 SHOULD） |
| 引用/提及 | 引用回复、@提及通知 | @提及解析与通知（`mention.go`） | MUST 已对齐 |
| 投票（Poll） | 官方 poll 插件 | 无 | SHOULD 未对齐（不阻断验收） |
| Onebox/链接预览 | 链接一键展开卡片 | 无 | SHOULD 未对齐 |

### B. 浏览与发现

| 能力 | Discourse 基线 | Feed Chamber 现状 | 结论 |
|---|---|---|---|
| 列表视图 | Latest / New / Unread / Top / Categories | `/posts/list` 支持 `list_view`（latest/new/unread/read/posted）+ `last_active_at` 排序 | MUST 已对齐（语义：posted=我发布，参与话题为增强） |
| 已读追踪 | TopicUser/TopicView + PostTiming 精确到楼层 | `topic_read_states`（`LastReadAt` + `LastContentID`）+ `MarkRead` 携带 `last_content_id`，列表返回 `unread_content_count` | MUST 已对齐（分页跳转仍需加固） |
| Suggested Topics | 同分类/标签相关推荐 | `POST /posts/{id}/suggestions`（同版面最近活跃，排除当前帖） | MUST 已对齐 |
| 搜索 | 全文检索（Postgres tsvector）、高级筛选、排序 | SQL 扩展 `contents.body` 命中、Mongo `$lookup` 评论命中、Postgres 时 `to_tsvector`、关键词高亮、`/search` 完整搜索页 | MUST 已对齐（GIN/FTS 索引为性能增强） |
| SEO | canonical、robots、sitemap、RSS、结构化数据 | `/robots.txt`、`/latest.rss`、sitemap index、`/sitemap-posts.xml`、canonical 收敛到 `/t/{slug}/{id}` | MUST 已对齐（OG/JSON-LD 为 SHOULD） |

### C. 身份与社交

| 能力 | Discourse 基线 | Feed Chamber 现状 | 结论 |
|---|---|---|---|
| 用户体系 | 注册/登录、邮件验证、用户卡片、公开资料 | User（ULID）、`TrustLevel`（basic/member/regular/leader）、公开资料 `PublicUser`、个人中心统计 | MUST 已对齐 |
| 信任等级 | TL0~TL4 自动晋升、权限联动 | 信任等级基于积分显示（`total_points`），权限联动为 SHOULD | MUST 部分对齐（展示已对齐，规则联动后续） |
| 徽章（Badge） | 系统/自定义徽章、授予与展示 | `badges` + `user_badges` + 排行榜/个人中心展示，签到触发评估 | MUST 已对齐 |
| 关注/屏蔽 | Follow/Block、列表与计数 | `/users/{userId}/follow*` + `/blocks/*`，关注计数 | MUST 已对齐 |
| 私信（PM） | 私信会话、已读、通知 | `/messages/conversations/*`（SQL/Mongo 双实现）| MUST 已对齐 |
| 通知 | 站内通知、未读数、实时推送 | `/notifications/*` + SSE `/stream` + KV 未读缓存 + 通用 `/realtime/stream` | MUST 已对齐 |
| 签到/积分 | 插件化签到、积分/等级 | `/checkin`（日签、连续天数、积分）+ 积分可见化 | MUST 已对齐（Discourse 原生无签到，属对齐增强） |

### D. 治理与审核

| 能力 | Discourse 基线 | Feed Chamber 现状 | 结论 |
|---|---|---|---|
| Review Queue | 统一审核队列（待审内容、举报、用户） | `content_moderations` + `/admin/moderation/*` | MUST 已对齐 |
| 举报（Flag） | 结构化举报类型、去重、处置 | `reports`（`flag_type`/快照/聚合计数/分页/处置审计） | MUST 已对齐 |
| 禁言/封禁 | silence / suspend / shadowban 分级 | 全局/版面两级禁言（`banned` 表）、`role=ban` 封号、网关 banCheck 拦截写操作、版面禁言 | MUST 已对齐（shadowban 为 SHOULD，规划过但未实现，详见历史计划） |
| 权限/角色 | 组（Group）+ 权限点 + 版面级权限 | Role/Permission 模型、20+ 权限码、版面级 `space_permissions`、`PermissionService` 缓存 + Pub/Sub 同步、Gateway 权限映射 | MUST 已对齐 |
| 审计 | Staff Action Log | `audit_logs`（用户/举报/批量/注册审批/回收站恢复等） | MUST 已对齐（全覆盖为 SHOULD 持续补齐） |
| 慢速模式/限流 | 慢速、限流、IP 封禁 | `RateLimit`（滑动窗口，KV）、登录/注册限流、安全设置（brute force/IP 黑名单） | MUST 已对齐 |

### E. 平台与开放

| 能力 | Discourse 基线 | Feed Chamber 现状 | 结论 |
|---|---|---|---|
| 认证 | 本地、邮件验证、MFA | JWT（access 24h/refresh 7d）、JTI 黑名单、MFA TOTP/备份码、邮件验证 | MUST 已对齐 |
| OAuth/SSO | 多 OAuth 提供商、OAuth2 Provider、OIDC SSO | OAuth（6 提供商）+ OAuth Provider（`token/revoke/authorize/clients`）+ SSO OIDC（`/.well-known/*`、JWKS RS256） | MUST 已对齐 |
| API Key | 作用域、限流、审计 | `/api-keys`（scope 校验、限流、撤销）| MUST 已对齐 |
| Webhook | 事件订阅、投递、重试、日志 | `/admin/webhooks/*`（ping/retry/退避/delivery 日志/TLS）| MUST 已对齐 |
| 备份/恢复 | 站点备份与恢复、保留策略 | `/admin/backups/*`（快照归档到 storage、checksum、恢复校验、保留 `backup_retention_days`）| MUST 已对齐 |
| 回收站 | 软删除与恢复 | `/admin/trash/*`（帖子/评论列表与恢复，SQL/Mongo）| MUST 已对齐 |
| 主题/插件 | 主题、主题组件、插件 API | `/themes/*`（声明式注册 `orinlight/orindark` + 用户偏好）、`/admin/plugins/*`（声明式元数据管理，不执行任意代码）| MUST 已对齐（Discourse 主题/插件全量为 MAY） |
| 存储 | 本地/S3、上传、白名单 | `storage`（local/S3）、预签名/确认/列表/删除 | MUST 已对齐 |

### F. 实时与可靠性

| 能力 | Discourse 基线 | Feed Chamber 现状 | 结论 |
|---|---|---|---|
| 实时 | MessageBus 长轮询/SSE、通知/帖子实时 | SSE `/notifications/stream` + 通用 `/realtime/stream`（channel/envelope/心跳/断线恢复）+ NATS 事件总线 | MUST 已对齐 |
| 性能 | 索引、分页、缓存 | 高频复合索引（posts/contents/notifications/topic_reads 等）、KV 缓存（未读数/限流/JTI） | MUST 已对齐（全文索引持续加固） |
| 可访问性 | 键盘、焦点、ARIA | skip link、`dialog`/`aria-modal`/Escape、分页 `aria-current`、焦点轮廓、快捷键（`/`/`?`/`Esc`） | MUST 已对齐 |

## 3. API 风格与约定

本项目强制 RPC 风格（`POST /{resource}/{id}/<verb>`、body 传参、统一响应），与 Discourse REST 风格不同。基线对齐判定以“能力等价”为口径，不以 URL/方法一致为口径；新增接口必须遵守本项目 RPC 约定（见 `AGENTS.md`）。

## 4. 差距分级与验收口径

### P0（若存在则不算对齐）
- 公开可见性：`pending/rejected` 不得出现在公开列表/详情/搜索/批量/置顶/Suggested Topics（普通用户显式传参亦强制公开状态）。
- 权限映射：Gateway 权限表覆盖全部 protected 路由，禁止“有路由无映射导致 403/越权”。
- 事务与双后端一致性：SQL/Mongo 同语义；关键写路径错误不得静默吞掉。
- 安全基线：OAuth `redirect_uri` 白名单与授权码绑定、 scope 越权拒绝、实时 channel 授权、JWKS/ID token 校验。

> 依据第 7 轮审查，上述 P0 在当前工作树已按批次修复并进入校验阶段；本基线文档将 P0 作为持续回归门禁。

### P1（基本对齐验收前必须闭环）
- 已读精确到评论级、搜索正文/评论命中与屏蔽作者过滤、通知未读一致性、Monitored 索引覆盖。

### P2/SHOULD（逐步对齐）
Poll、Onebox、wiki、慢速模式细化、archived/unlisted、OG/JSON-LD、GIN/FTS 专库等。

## 5. 对齐路线（与现有 Batch 映射）

- Batch 1：安全与可见性基线（已落地，持续回归）
- Batch 2：浏览与创作体验（列表视图/已读/推荐/搜索/SEO/Composer/草稿/版本）
- Batch 3：身份与治理闭环（资料/徽章/关注/私信/审核/举报/禁言/审计）
- Batch 4：平台化（Webhook/API Key/SSO-OAuth/备份/回收站/主题-插件）
- Batch 5：实时/性能/可访问性/i18n/可靠性

详见对齐计划整合结论（已归档）与 `reviews/phase4-discourse-alignment-retro.md`。

## 6. 验证门禁（MUST）

```bash
cd app/server
go test ./internal/service ./internal/model ./internal/repository ./pkg/gateway -count=1
# Dashboard/Web 类型与构建（若涉及前端改动）
pnpm --filter @feedchamber/dashboard-vite exec tsc -p tsconfig.app.json --noEmit
pnpm build:web
```

接口与浏览器：
- 全路由权限测试覆盖 Webhook、API Key、Sessions、实时通道
- `feedchamber_test.sh` 覆盖搜索/审核/禁言/草稿/版本
- Mongo 双后端覆盖版面过滤、列表可见性、私信

## 7. 结论

按本基线度量，Feed Chamber 已完成 Discourse 基本能力（MUST）的结构性对齐：内容结构、浏览发现、身份社交、治理审核、平台开放、实时可靠性六大域的开箱能力均已具备真实实现与双后端覆盖。剩余工作为 P1 加固与 SHOULD 增强，不影响“基本能力对齐”验收。下阶段以 P0 回归 + P1 闭环为唯一优先级，按 Batch 顺序推进。

