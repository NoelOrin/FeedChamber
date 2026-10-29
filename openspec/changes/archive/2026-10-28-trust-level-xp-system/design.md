# Design — XP / 等级 / 门控规则引擎

提案：[proposal.md](proposal.md) · 规范 delta：[specs/trust-levels/spec.md](specs/trust-levels/spec.md)、[specs/access-gating/spec.md](specs/access-gating/spec.md)、[specs/community-motion/spec.md](specs/community-motion/spec.md)

> 本文档的数值均为**已讨论定稿**（2026-10-28 与运营逐项确认），不再是建议值。

---

## 1. 领域模型与数据模型

四层单向依赖：

```
行为（已有表：posts / contents / likes / favorites / check_ins / login_history…）
  ↓  按「XP 规则」（可配：哪个指标给多少经验）
XP（经验值）
  ↓  按「等级定义」（可配：XP 门槛 + 滚动窗口保级条件）
等级（TL0…TL4）
  ↓  按「门控规则」（可配：谓词 → 动作）
能力（读内容 / 发帖 / 评论 / 外链 / 频率）
```

### 1.1 指标注册表（XP 规则与保级条件共用）

指标 MUST 是**固定枚举**（可 SQL 聚合），只定义一次，XP 规则与保级条件都引用它：

| 指标 | 含义 |
|---|---|
| `post_approved_count` | 已通过审核的帖子数 |
| `comment_approved_count` | 已通过审核的评论数 |
| `likes_received_count` | 收到的赞数（按点赞行） |
| `favorite_received_count` | 被收藏数（按收藏行） |
| `featured_post_count` | 被设为精华的帖子数 |
| `checkin_count` | 签到天数 |
| `checkin_streak_hit(3/7/15/30/60/100)` | 连续天数**恰好达到**该里程碑的次数 |
| `active_day_count` | 活跃天数（当日有登录或产生内容） |
| `report_upheld_count` | 已受理成立的被举报数 |

### 1.2 表

| 表 | 关键列 | 说明 |
|---|---|---|
| `xp_rules` | `metric`(枚举), `delta`, `daily_cap`, `enabled`, `updated_at` | 指标 → XP |
| `xp_streak_bonuses` | `days`, `bonus`, `enabled` | 连续签到阶梯（列表，可增删） |
| `trust_levels` | `level`, `key`, `name`(可改), `min_xp`, `window_days`, `min_active_days`, `min_posts`, `min_likes`, `max_reports`, `manual_only`, `enabled` | 等级定义 |
| `gate_rules` | `id`, `priority`, `enabled`, `domain`, `conditions`(JSON，受限谓词集), `action`, `action_params`(JSON), `note`, `updated_by`, `updated_at` | 门控规则 |
| `gate_domain_defaults` | `domain`, `default_action` | 每个能力域的默认动作 |
| `xp_adjustments` | `id`, `user_id`, `delta`, `reason`, `actor_id`, `created_at` | 人工/活动加减分 |
| `users`（改） | `total_xp`(冗余), `trust_level`(数值), `level_evaluated_at`, `level_metrics`(JSON 快照)；**删除 `reputation`** | 派生缓存 |
| `spaces`（改） | `visibility`(`public`/`login`/`level`), `min_level` | 取代 `auth_required` bool |
| `posts`（改） | `min_level`（0=继承版面）、`featured_at`（精华） | 单篇覆盖 + 精华标记 |

### 1.3 关键性质

1. **派生 vs 权威**：`users.total_xp` / `trust_level` / `level_metrics` 是缓存；权威 = 行为表 + `xp_rules` + `trust_levels` + `xp_adjustments`。写入必须经 `TrustService.Recompute(userID)` 单点，其它 service 禁止直写。
2. **TL0 = 公共游客 + 刚注册用户**：未登录访问者在等级比较中**一律视为 TL0**，因此"等级 ≥1"这类规则天然排除游客；`is_anonymous` 谓词仍保留（用于 `login` 可见性等场景）。
3. **等级名称可改**：`trust_levels.name` 后台可编辑，前端不硬编码中文标签（现在 `profile-header.tsx` 的 `TRUST_LABELS` 要改为读配置）。

---

## 2. 求值链路与规则引擎语义

### 2.1 XP 求值

```
total_xp(u) = Σ_{自然日 d} Σ_{规则 r} min(count_r(u, d), daily_cap_r) × delta_r
            + Σ_{阶梯 s} checkin_streak_hit_s(u) × bonus_s
            + Σ xp_adjustments(u)
```

- **日上限按「自然日 × 指标」分别截断**：每条规则的 `count_r(u, d)` 是该指标在自然日 `d` 的计数（被赞按点赞行的创建日归集）；无上限的规则（精华、里程碑）不参与截断。
- **只计有效内容**：帖子/评论必须 `approved` 且未删除；取消赞、取消收藏、取消精华、删帖都**自动回收**，不需要"扣分"逻辑。
- **里程碑是"恰好达到"计数**：`streak == 3/7/15/30/60/100` 各计一次；断签重新累计可再次获得（符合"里程碑一次奖励"语义）。
- **刷新时机**：行为发生时（发帖/评论/被赞/被收藏/加精/签到/上述撤销）→ 重算 `total_xp` 并触发等级评估；另有每日对账任务兜底。
- 不记事件 → 重复计分与计数漂移从根上不存在。

### 2.2 等级求值（累计 XP 定级 + 100 天窗口保级）

```
target = 满足 min_xp ≤ total_xp 的最高等级
若 target 级配有保级条件（window_days = 100）：
    校验 活跃天数 / 发帖 / 获赞 / 成立举报上限
    不满足 → 逐级下探，取第一个「min_xp 与保级条件同时满足」的等级
manual_only 的等级（TL4）不参与自动定级
```

- **触发**：① 事件驱动（升级）；② 每日 ticker（处理不活跃衰减降级）；③ 读时惰性（快照过期先重算）。
- 降级是"下探到仍然合格的最高级"，不会一次掉到底。
- 等级门控对**所有角色**生效，无"等级>0 直接放行"短路；需要豁免就写显式 `allow` 规则。

### 2.3 门控规则引擎

**谓词集**（固定枚举，后台表单化）：`level`（比较）、`space` / `tag`（范围）、`author_level`、`is_author`、`window_metric`、`xp`、状态类（`is_anonymous` / `email_verified` / `banned` / `exiled`）、`contains_external_link`。

**动作集**（固定枚举，四个，互不重叠）：`allow` / `deny` / `require_review`（转待审）/ `rate_limit(n, window)`

> **可见性不进动作集**：内容可见性由数据属性决定（`spaces.visibility` + `min_level`、`posts.min_level`），见 §3.1。规则引擎只回答"能不能做某动作"。原稿把 `min_level` 同时放在两处，是自检抓出的重叠，已移除。

**能力域与默认动作**（每域一个显式默认，无规则命中时生效）：

| 能力域 | 默认动作 |
|---|---|
| `content_view`（读内容） | `allow`（站点默认公开，与线上 `post_auth_required=false` 一致） |
| `post_create` | `allow` |
| `comment_create` | `allow` |
| `external_link` | `deny`（沿用现在"TL0 禁外链"的保守默认） |
| `rate_limit` | 无额外限制（14 处端点预算继续承担） |

**求值语义**：按 `priority` 降序取**首个命中**；无命中走该能力域的默认动作；命中结果 MUST 记入 trace（命中哪条规则 / 默认动作）。

**执行位置**：

| 执行点 | 现状 | 新设计 |
|---|---|---|
| 等级门 | 只覆盖 TL0（`service/trust.go:169,179` 短路） | `GateService.Evaluate` 统一求值，去掉短路 |
| 发帖审核 / 外链 | 两个 TL0 专用开关（`trust_tl0_block_links` 连 DefaultSettings 都不在） | 两条可配规则；seed 只建外链那条 |
| 限流 | 14 处预算全硬编码、一律按 IP | `rate_limit` 动作引入按用户/等级维度（键形状新增，注意清零在计数的窗口） |
| 内容可见性 | `AuthGate` 两态 | 见 §3.1，下沉到 service/repository SQL 条件 |

**缓存**：规则集内存编译快照 + KV Pub/Sub 失效（沿用 `perm:sync` 模式）。

**已有先例可抄**：内容审核已是"规则表 + 优先级 + 动作"（`model/moderation.go:61-76`、`:22-27`）。

---

## 3. 可见性策略与后台配置面

### 3.1 可见性策略

| `spaces.visibility` | 语义 | 迁移自 |
|---|---|---|
| `public` | 匿名可读（免登录） | `auth_required=false` |
| `login` | 需登录 | `auth_required=true` |
| `level` | 需 `level ≥ min_level` | 新增 |

- `posts.min_level` 覆盖版面（0=继承）；版面可见性保留祖先继承。
- **执行位置**：可见性依赖数据 → 必须在 service/repository 层做 SQL 过滤；路由级 `AuthGate` 退化为站点级登录墙。
- **出口一致性**：列表 / 详情 / 搜索 / sitemap / RSS / `/posts/batch` MUST 共用同一可见性谓词。
- **对外表现**：受限内容在聚合出口不出现；直连详情 **403 + 所需等级/当前等级**（区别于封禁的 404）。
- **游客即 TL0**：匿名访客对 `level` 策略版面一律视为等级 0。

### 3.2 后台配置面

| 页面 | 管什么 |
|---|---|
| `/admin/levels` | 等级定义（名称可改 / XP 门槛 / 保级条件 / 是否仅人工） |
| `/admin/xp-rules` | 指标 → XP、日上限、连续签到阶梯 |
| `/admin/gates` | 规则 CRUD（优先级、条件表单、动作下拉）+ 能力域默认动作 + 冲突/死规则校验 + "以用户试算" |
| `/admin/spaces`（改） | 版面可见性策略（public / login / level + min_level） |
| `/admin/posts`（改） | "设为精华"操作（写 `posts.featured_at`） |
| `/admin/users/detail`（改） | XP / 等级 / 保级指标 + **各指标分解**（"帖子 12 篇 → 120 XP、签到 30 天 → 60 XP…"）+ 人工调整 XP |

新增权限码 `level:manage` / `gate:manage`，菜单登记 `admin-shell.tsx`、路由门控登记 `route-access.ts`。

---

## 4. 迁移 · 分期 · 测试

### 4.1 存量迁移

| 现状 | 动作 |
|---|---|
| `users.trust_level` 字符串 | 等级改数值；XP 走聚合 → 迁移 = 跑一次全量重算 |
| `users.total_points` | 改名 `total_xp`；API 保留 `total_points` 兼容别名，前端随后迁移 |
| `trust_tl0_require_approval` / `trust_tl0_block_links` | 转成 seed 门控规则（只建外链 deny），旧 key 标废弃 |
| `space_view_auth_required` + `Space.AuthRequired` | 迁移成 `visibility`（`true` → `login`），保留祖先继承 |
| `users.reputation` | **删除列**；动议票权改为按等级（community-motion delta） |
| 死配置 11 个 | 清理（`upload_auth_required`、`user_profile_auth_required`、`security_ip_blacklist`、`security_brute_force_enabled`、`passkey_*`、`mfa_sms/email/backup_codes_enabled`） |

### 4.2 分期

- **P1 基座**：三表 + `xp_streak_bonuses` + `posts.featured_at` + XP 聚合求值 + 等级求值（100 天保级）+ 每日重算 ticker + 单点写入 + `/admin/levels`、`/admin/xp-rules`、`/admin/posts` 加精 + 用户详情指标分解。
- **P2 可见性**：`visibility` 枚举 + `posts.min_level` + 统一可见性谓词（六出口）+ 详情 403 提示 + 版面配置。
- **P3 门控引擎**：`gate_rules` + `gate_domain_defaults` + 求值器（首匹配 + 域默认）+ `/admin/gates` + 接管 TL0 外链门 + 限流按用户/等级维度。
- **P4 收尾**：删 `reputation` 并改票权、前端等级文案改读配置、榜单"发帖榜显示积分"错位、死配置清理、前端迁移到 `total_xp`。

### 4.3 测试策略

- **XP 聚合**：纯函数表驱动（日上限、删帖/取消赞/取消精华回收、改分值后重算、连签里程碑"恰好达到"计数）。
- **等级求值**：升级 / 保级失败降级 / 边界 XP / 窗口过期 / `manual_only` 不自动升降。
- **可见性出口一致性（最关键）**：同一篇受限帖在 **list / detail / search / sitemap / RSS / batch** 六个出口的可见性必须一致；含"游客 = TL0"用例。
- **门控引擎**：首匹配、域默认动作、优先级、非法/矛盾规则校验、试算与线上结果一致。
- **迁移**：旧 `auth_required` / `trust_level` / `points` → 新模型的迁移脚本测试。

### 4.4 已定 seed 默认值

| 项 | 值 |
|---|---|
| 等级 | TL0 `new` 新手上路（游客+新注册）/ TL1 `basic` 基础 / TL2 `member` 成员 / TL3 `regular` 常客 / TL4 `leader` 意见领袖（仅人工） |
| XP 门槛 | 0 / 50 / 200 / 500 / 1000 |
| 保级窗口 | 100 天 |
| 保级条件 | TL1 无；TL2 活跃 ≥10 天、成立举报 ≤5；TL3 活跃 ≥50 天、发帖 ≥10、获赞 ≥20、成立举报 ≤3；TL4 仅人工 |
| XP 分值（日上限） | 发帖 10（25）/ 评论 3（20）/ 被赞 1（30）/ 被收藏 2 / 精华 10 / 签到 2 |
| 连续签到阶梯 | 3/7/15/30/60/100 天 → +3/+10/+20/+40/+80/+150（恰好达到计一次） |
| 能力域默认 | `content_view` allow / `post_create` allow / `comment_create` allow / `external_link` deny / `rate_limit` 无 |
| seed 规则 | ① `external_link` + `level = 0` + 含外链 → `deny`（优先级 100）；② 不 seed 发帖强制待审 |
| 动议票权 | TL0–TL1 → 1 票 / TL2–TL3 → 2 票 / TL4 → 3 票（可配） |
