# 重构积分/等级为「XP + 等级 + 门控规则引擎」

## Why

`users.trust_level` 与积分体系是分三次长出来的，现在互相打架且大面积硬编码：

- **三方抢写同一个字段**：积分阈值（`service/points.go:46` `trustLevelForPoints`，50/200/500）、每日 ticker 的行为指标晋升（`service/trust.go:114-135`，天数/帖数/赞数/回复数）、管理员手工设置（`service/trust.go:70`）。管理员把某人设成 `leader`，对方下次发帖触发 `syncTrustLevel` 就被按积分重置回 `basic`。
- **等级门只覆盖 TL0**：`service/trust.go:169` 与 `:179` 对 `trustLevel > 0` 直接 `return false`，TL1+ 不受任何配置约束；`trustIntToString` 把 3/4 都映射成 `leader` 且晋升链只到 TL2 → **TL3/TL4 实际不可达**。
- **规则全硬编码**：XP 分值（`service/points.go:11-14`）、等级阈值（`points.go:24-34`、`trust.go:21-24`）、14 处限流预算与窗口（`routes_*.go`）、请求体上限与超时（`router.go:65-70`）。后台设置页（声明式 81 key）里一个都改不了。
- **无账本、无解释、有漂移**：积分只是一个汇总列 `users.total_points`，改分值不会重算、删帖不回收；`IncrPoints`（原子自增）与签到的 `user.TotalPoints += points; Update(user)`（`Save` 全行覆盖，`repository/user_sql.go`）两条路径混用会丢更新。
- **可见性只有两态**：`middleware.AuthGate`（`middleware/auth.go:97`）只能表达"登录/不登录"，**无法表达"等级 ≥N 才能读"**；版面隔离靠 `Space.AuthRequired` bool + 全局 `space_view_auth_required`，与 `posts` 状态过滤（`StatusIn=[approved,closed]`）各写一套，六个内容出口（列表/详情/搜索/sitemap/RSS/batch）各自硬编码。
- **死字段与死配置**：`reputation` 无任何写入点却驱动动议票权（`service/motion.go:124`，导致票权恒为 1）；11 个 settings key 表里有、代码从不读（`upload_auth_required`、`user_profile_auth_required`、`security_ip_blacklist`、`passkey_*`、`mfa_sms/email/backup_codes_enabled` 等）。

## What Changes

- **XP 是经验值，权威来源是行为聚合**：`total_xp(u) = Σ(指标计数 × 当前 XP 规则) + Σ(连续签到里程碑) + Σ(人工调整)`。不建事件账本 → 天然免疫重复计分与计数漂移；改分值立即对全员生效；删除内容、取消点赞/收藏、取消精华都**自动回收**。人工/活动加减分单独落 `xp_adjustments`。
- **指标注册表**：XP 规则与保级条件共用同一批固定枚举指标（帖子/评论/获赞/被收藏/精华/签到/连续签到里程碑/活跃天数/成立举报）。
- **连续签到按里程碑一次性奖励**：连续天数**恰好达到** 3/7/15/30/60/100 天各给一次（断签重来可再得），阶梯可配。
- **等级 = 累计 XP 定级 + 100 天滚动窗口保级**（可升可降）；**TL0 = 公共游客 + 刚注册用户**（未登录访客在等级比较中一律视为等级 0）；仅人工授予的等级（TL4）不参与自动升降。
- **等级是派生缓存**：`users.trust_level` / `total_xp` 只是快照，**只能经统一 evaluator 写入**，消灭"三方抢写"。等级名称可改，前端不再硬编码中文标签。
- **门控规则引擎**：`gate_rules(priority, domain, conditions, action)`，条件限定为固定谓词集，动作限定为固定枚举（`allow` / `deny` / `require_review` / `rate_limit`）。求值语义 = **按优先级首个匹配生效 + 每个能力域的显式默认动作**。
- **可见性统一成策略枚举**：`spaces.visibility ∈ {public, login, level}` + `spaces.min_level`，`posts.min_level` 可覆盖。**列表/搜索/sitemap/RSS 不出现，直连详情返回 403 并提示所需等级**。可见性不进规则动作集，只由数据属性决定。
- **出口一致性**：六个内容出口 MUST 共用同一个可见性谓词函数。
- **新增精华机制**：`posts.featured_at` + 后台"设为精华"（否则"加精 +10 XP"无数据来源）。
- **删掉 `reputation`**：动议票权改为按信任等级（TL0–1 → 1 票 / TL2–3 → 2 票 / TL4 → 3 票，可配），带 MODIFIED delta 到 `community-motion`。
- **全部后台可配**：新增 `/admin/levels`、`/admin/xp-rules`、`/admin/gates` 三页 + 版面可见性配置 + 帖子加精 + 用户详情（XP/等级/保级指标/**指标分解**/人工调整）。

## Non-goals

- **不改权限体系**：路径→权限码映射（`gateway/permission.go:193-444`）与角色权限表不动；门控规则引擎管的是"等级/状态相关的业务能力"，不是"有没有这个权限码"。
- **不接管与等级无关的硬编码**：请求体上限（1MB/10MB）、请求超时（30s）、维护模式放行清单、权限映射表本身——另案处理。
- **不建完整 XP 事件账本**：行为 XP 走聚合（前端现在也没有"积分明细"页面，账本属 YAGNI）；只有人工/活动调整有记录。
- **不动内容审核引擎**：`moderation_rules` / `moderation_configs` 维持现状，门控引擎的 `require_review` 动作只决定"要不要送审"，不改变审核规则本身。
- **不动暗区/软隔离机制**：`ExiledUntil` 与列表的 `ExcludeExiled` 保持现状，不并入可见性策略——两者语义不同（隔离是处罚，等级门槛是权限）。
- **不改动议的其它机制**：法定人数、公示期、结算与结果执行保持现状，本次只把票权来源从 `reputation` 换成等级。
- **不引入规则动作 `hide` 与 `notify`**：前者与可见性策略、软隔离语义重叠，后者的观察需求由 trace 与审计日志承担。
