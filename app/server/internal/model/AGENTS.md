# Model 层

纯数据结构体定义，无业务逻辑。

## 约定

- 每个文件定义一个或多个相关的结构体
- SQL 标签用 `gorm:"..."`
- JSON 序列化用 `json:"..."`，敏感字段加 `json:"-"` (如 Password)
- 自定义表名通过 `TableName() string` 方法
- 种子数据用包级变量 (如 `DefaultRoles`, `DefaultPermissions`)
- 缓存用包级 map + `LoadXxxCache()` 函数 (如 roleCache)
- DTO/ListItem 结构体用于列表查询的扁平化输出

## 文件列表

| 文件 | 模型 | 说明 |
|------|------|------|
| user.go | User, UserResponse, PublicUser | 用户，含 ID Hook、`ToResponse` / `ToPublic`。等级与 XP 是派生缓存：`TrustLevel` 为数值、`TotalXP`（列/JSON 均为 `total_xp`；`total_points` 兼容别名已在 P4 下线）、`LevelEvaluatedAt` / `LevelMetrics` / `LevelManual`。只有 `TrustService.Recompute` 可以写这些字段 |
| post.go | Post, PostListItem, PostDetail | 帖子实体 + 列表 DTO + 详情 DTO (含作者/版面名/统计/正文)。`FeaturedAt` 非空即精华，是 XP 指标 `featured_post_count` 的唯一来源；`MinLevel` 是单帖可见性覆盖（0=继承版面） |
| content.go | Content, ContentListItem | 内容实体 + 内容列表 DTO |
| space.go | Space | 版面（图标/slug/排序/启禁）+ 可见性策略 `Visibility`(public/login/level) / `MinLevel`；旧的 `AuthRequired` 仅兼容旧库，启动时迁移后清零 |
| tag.go | Tag, PostTag | 标签 + 帖子-标签多对多 |
| role.go | Role | 角色 + 进程级缓存（`LoadRoleCache`, `IsValidRole`, `HasBanRole`） |
| permission.go | Permission, RolePermission | 权限定义 + 角色-权限关联 + 默认映射（admin/moderator/user/ban） |
| like.go | ContentLike | 内容点赞记录 |
| favorite.go | PostFavorite | 收藏记录 |
| follow.go | UserFollow | 关注关系 |
| upload.go | Upload | 上传文件记录 |
| notification.go | Notification | 通知 |
| report.go | Report | 举报 |
| setting.go | Setting | 系统设置 (key-value) + `DefaultSettings` + setting key 常量。`DefaultSettings` 是「默认值」的唯一落库来源：只写代码不写这里，读取会永远落回各处的硬默认（`trust_tl0_require_approval` 曾因此默认 true，把 TL0 用户的新帖静默改成待审） |
| credential.go | Credential | 密码凭证表（独立于 User） |
| banned.go | Banned | 禁言记录（全局/版面级，含过期时间自动解禁） |
| checkin.go | CheckIn | 每日签到。`PointsEarned` 是旧版签到积分的遗留列，自 2026-10-29 起恒为 0，不参与 XP 求值 |
| announcement.go | Announcement | 公告（含显示类型/置顶/过期时间） |
| oauth.go | OAuthUser, OAuthProvider | 社交登录绑定 + 提供商配置 |
| moderation.go | ContentModeration, ModerationRule, ModerationConfig | 内容审核模型（H-02 已迁移未启用）。另含文章审核模式常量：`ModerationMode{Off,Async,External,Manual}`、`DefaultModerationMode=async`、`ParseModerationMode`（读取侧回落默认值）与 `ValidModerationMode`（写入侧校验），模式本身存系统设置 `moderation_mode`，不在这三张表里 |
| meta.go | Meta | 键值元数据表（R-05 强制改密标记等） |
| gate.go | GateRule, GateDomainDefault, GateCondition | 门控规则引擎：5 个能力域（`GateDomain*`）、固定谓词字段（`GateField*` + `GateFieldOps` / `GateWindowMetrics`）、4 个动作（`GateAction*`）、首启 seed（`DefaultGateRules` / `DefaultGateDomainDefaults`）。条件与动作参数都是 JSON 字符串，谓词集固定、不提供自由表达式 |
| trust.go | XPRule, XPStreakBonus, TrustLevel, XPAdjustment, LevelMetrics | XP / 等级体系：指标注册表（`Metric*` 常量 + `XPMetricLabels`）、指标→XP 规则、连续签到阶梯、等级定义、人工调整账本。首启 seed 用 `DefaultXPRules` / `DefaultXPStreakBonuses` / `DefaultTrustLevels` |

> **关注目标类型只有 `post` / `tag`**：`watch.go` 的 `WatchTargetSpace` 已于 2026-10-28 删除（产品决定，与「关注话题」语义重叠），`validTargetType` 不再接受 `"space"`，存量行见 `db/migrations/cleanup_space_watches.sql`。
>
> 上表为早期模块，未覆盖全部（现共 49 个模型文件 / 68 个 struct）。另含：`space.go`(SpaceRolePermission)、`group.go`(Group/Member/Permission)、`session.go`、`apikey.go`、`mfa.go`、`oauth_client.go`/`oauth_authorize.go`/`oauth_token.go`、`conversation.go`/`message.go`、`webhook.go`、`plugin.go`、`backup.go`、`draft.go`、`revision.go`、`badge.go`、`theme.go`/`user_preference.go`、`invite.go`、`watch.go`、`user_block.go`、`security_event.go`/`login_history.go`、`upload.go`、`import_map.go` 等。

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。

## 主题 token 与 meta 标记（2026-10-30）

- `ThemeToken` 的身份是 `(token, mode)`（`idx_theme_token_mode` 复合唯一索引）：同一 token 可分别覆盖明亮与暗色，规范要求「同时作用于两种模式或明确指定其一」。
- `MetaSeededRolePermissionCodes`（`seeded_role_permission_codes`）记录「已应用过的默认权限码」：升级补齐只补新码，管理员撤销的默认权限不会在重启时复活（R-04）。
