# Tasks — XP / 等级 / 门控规则引擎

提案：[proposal.md](proposal.md) · 设计：[design.md](design.md) · 规范 delta：[specs/trust-levels](specs/trust-levels/spec.md)、[specs/access-gating](specs/access-gating/spec.md)、[specs/community-motion](specs/community-motion/spec.md)

> 分期原则：每期独立可上线。P1 不做可见性、P2 不做规则引擎，避免一次性重构。
> 数值已定稿（2026-10-28 讨论），见 [design.md §4.4](design.md)。

## 0. 决策项（已定稿，开工前提）

- [x] 0.1 等级：TL0–TL4 五级，门槛 0/50/200/500/1000，TL4 仅人工；TL0 = 游客 + 新注册
- [x] 0.2 保级：100 天窗口；TL1 无保级；TL2 活跃 ≥10 天、成立举报 ≤5；TL3 活跃 ≥50 天、发帖 ≥10、获赞 ≥20、成立举报 ≤3
- [x] 0.3 XP：发帖 10（上限 25）/ 评论 3（20）/ 被赞 1（30）/ 被收藏 2 / 精华 10 / 签到 2；连续签到里程碑 3/7/15/30/60/100 天 → +3/+10/+20/+40/+80/+150（恰好达到计一次）
- [x] 0.4 默认动作：按能力域（读内容 allow / 发帖 allow / 评论 allow / 外链 deny / 限流 无）
- [x] 0.5 `reputation`：删除列，动议票权改按等级（TL0–1 → 1 / TL2–3 → 2 / TL4 → 3）

## 1. P1 基座：XP 与等级

- [x] 1.1 模型 + AutoMigrate：`xp_rules`、`xp_streak_bonuses`、`trust_levels`、`xp_adjustments`；`posts.featured_at`；`users` 增 `total_xp` / `level_evaluated_at` / `level_metrics`
- [x] 1.2 repository：四表接口 + SQL 实现（含窗口指标与各指标的聚合查询）
- [x] 1.3 `service/trust_level.go`：指标聚合求值（含 `daily_cap` 截断、只计 approved 未删内容）
- [x] 1.4 `service/trust_level.go`：连续签到里程碑"恰好达到"计数
- [x] 1.5 `service/trust_level.go`：等级求值（累计 XP 定级 + 100 天保级 + 逐级下探降级 + `manual_only` 不自动升降）
- [x] 1.6 `service/trust_level.go`：`Recompute(userID)` 单点写入；删除 `points.go` 的 `syncTrustLevel` 与 `checkin.go:97` 直写
- [x] 1.7 行为触发点改调 `Recompute`：发帖/删帖、评论/删评论、被赞/取消赞、被收藏/取消收藏、加精/取消加精、签到
- [x] 1.8 每日重算 ticker（处理不活跃衰减降级），替换 `trust.StartDailyPromotionTicker`
- [x] 1.9 删除硬编码：`PointsPostCreate/CommentCreate/Liked`、`trustLevelForPoints`、`trust.go` 的 `TrustLevel0..4` 与 `evaluatePromotion`
- [x] 1.10 一次性全量重算入口（迁移命令，可重复执行）
- [x] 1.11 `users.total_points` → `total_xp`（API 保留 `total_points` 兼容别名）
- [x] 1.12 后台 `/admin/levels`（等级 CRUD，名称可改）
- [x] 1.13 后台 `/admin/xp-rules`（指标 → XP、日上限、连续签到阶梯）
- [x] 1.14 后台 `/admin/posts` 增加「设为精华」（写 `featured_at`）
- [x] 1.15 后台用户详情：XP / 等级 / 保级指标 / **指标分解** / 人工调整 XP
- [x] 1.16 权限码 `level:manage` seed + 菜单与路由门控登记
- [x] 1.17 测试：XP 聚合（日上限 / 删帖回收 / 取消赞收藏回收 / 改分值重算）、里程碑恰好达到与断签重得、等级求值（升降级 / 边界 / 窗口过期 / manual_only）、单点写入

## 2. P2 可见性

- [x] 2.1 `spaces` 增 `visibility` 枚举 + `min_level`；`posts` 增 `min_level`；数据迁移（`auth_required=true` → `login`）
- [x] 2.2 统一可见性谓词函数（状态 + 等级 + 版面策略 + 作者例外 + 游客=TL0），放 repository 层供 SQL 复用
- [x] 2.3 六个出口改走该谓词：列表、详情、搜索、sitemap、RSS、批量
- [x] 2.4 详情对受限内容返回 403 + 所需等级/当前等级（区别于封禁的 404）
- [x] 2.5 版面祖先继承语义迁移到 `visibility`
- [x] 2.6 `middleware.AuthGate` 退化为站点级登录墙；`SpaceAuthGate` 由可见性谓词取代
- [x] 2.7 后台 `/admin/spaces` 增加可见性策略配置（public / login / level + min_level）
- [x] 2.8 前端：受限内容详情页「需要等级 N」提示 + 升级进度
- [x] 2.9 前端：等级名称改读后端配置（去掉 `TRUST_LABELS` 硬编码，修 `visitor-profile-page` 显示原文的 bug）
- [x] 2.10 测试：**六出口可见性一致性矩阵**（同一受限帖 × 匿名/低等级/达标等级）

## 3. P3 门控规则引擎

- [x] 3.1 模型 + repository + AutoMigrate：`gate_rules`、`gate_domain_defaults`
- [x] 3.2 `service/gate.go`：谓词求值器（固定谓词集）+ 动作执行器（`allow` / `deny` / `require_review` / `rate_limit`）
- [x] 3.3 求值语义：优先级降序首个匹配 + 能力域默认动作
- [x] 3.4 规则校验：非法谓词/动作、未知字段、死规则、优先级冲突
- [x] 3.5 内存快照 + KV Pub/Sub 失效（沿用 `perm:sync` 模式）
- [x] 3.6 命中记录进 trace/日志（命中哪条规则 / 域默认动作）
- [x] 3.7 seed 规则：`external_link` + `level = 0` + 含外链 → `deny`；不 seed 发帖强制待审
- [x] 3.8 接管旧门：`trust_tl0_block_links` → 规则；`trust_tl0_require_approval` 标废弃
- [x] 3.9 限流增加按用户/等级维度（注意键形状变更 = 清零在计数的窗口）
- [x] 3.10 后台 `/admin/gates`：规则 CRUD + 域默认动作 + 校验 + 「以用户试算」
- [x] 3.11 权限码 `gate:manage` seed + 菜单与路由门控登记
- [x] 3.12 测试：首匹配、域默认、优先级、非法规则、试算与线上一致

## 4. P4 收尾

- [x] 4.1 删除 `users.reputation` 列；动议票权改按等级（`service/motion.go`）
- [x] 4.2 榜单「发帖榜显示积分」错位修复（后端已返回 `post_count`，前端类型缺字段）
- [x] 4.3 清理 11 个死配置 key（`upload_auth_required`、`user_profile_auth_required`、`security_ip_blacklist`、`security_brute_force_enabled`、`passkey_*`、`mfa_sms/email/backup_codes_enabled`）
- [x] 4.4 `space.post_count` 死字段处置（`UpdatePostCount` 无调用方）
- [x] 4.5 前端迁移到 `total_xp`，下线 `total_points` 兼容别名
- [x] 4.6 清理旧 `trust_level` 字符串字段与 `check_ins.points_earned` 的历史语义（不再参与 XP 求值）

## 5. 文档与规范

- [x] 5.1 各模块 `AGENTS.md` 同步（service / repository / handler / router / gateway / model / middleware / app/web 相关域 / packages）
- [x] 5.2 `docs/api-reference.md` 与 `make swag` 重新生成
- [x] 5.3 本变更归档（`openspec archive`），delta 合入 `openspec/specs/`
- [x] 5.4 根 `AGENTS.md` 模块文档索引重写（`node scripts/check-agents-docs.mjs --write-index`）
