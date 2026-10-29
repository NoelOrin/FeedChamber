## Purpose

定义用户成长体系的契约：经验值（XP）如何产生、等级如何由 XP 与行为窗口派生、谁有权写入等级字段、游客与新手处在什么位置、以及这些规则如何被后台配置。目的是把当前"三方抢写 `users.trust_level`、规则全硬编码、积分只加不减且不可解释"的状态，收敛为**单一权威来源 + 单点写入 + 全部可配**的模型。

## ADDED Requirements

### Requirement: XP 由行为聚合派生

用户的经验值 MUST 由行为聚合派生，MUST NOT 依赖逐次累加的计数器列：

```
total_xp(u) = Σ_{自然日 d} Σ_{规则 r} min(count_r(u, d), daily_cap_r) × delta_r
            + Σ_{阶梯 s} checkin_streak_hit_s(u) × bonus_s
            + Σ xp_adjustments(u)
```

`daily_cap_r` MUST 按「自然日 × 指标」分别截断（`count_r(u, d)` 为该指标在自然日 `d` 的计数；被赞按点赞行的创建日归集）；无上限的规则（精华、连续签到里程碑）MUST NOT 参与截断。

行为计数 MUST 从既有业务表聚合（帖子、评论、点赞、收藏、精华标记、签到、登录），MUST NOT 为此新增埋点表或事件账本。因此：**撤销行为 MUST 自动回收对应 XP**（删除内容、取消点赞、取消收藏、取消精华），重复提交 MUST NOT 造成重复计分。

只有通过审核且未删除的内容 MUST 计入 XP；被审核拒绝或进入回收站的内容 MUST NOT 计分。

#### Scenario: 删除内容后 XP 自动回收

- **WHEN** 用户有一篇计 XP 的帖子，随后该帖被删除
- **THEN** 该用户的 `total_xp` MUST 相应减少
- **AND** 该用户 MUST 被重新评估等级

#### Scenario: 取消点赞与取消收藏自动回收

- **WHEN** 他人取消了对某用户内容的点赞（或收藏）
- **THEN** 该用户的 `total_xp` MUST 相应减少

#### Scenario: 被审核拒绝的内容不计分

- **WHEN** 某帖被审核置为 `rejected`
- **THEN** 其作者 MUST NOT 因该帖获得 XP

#### Scenario: 修改 XP 规则立即对全员生效

- **WHEN** 运营把「发帖」的 XP 分值从 10 改为 20
- **THEN** 所有用户下一次求值得到的 `total_xp` MUST 按新分值重算（MUST NOT 只影响此后的新帖）

#### Scenario: 每日上限截断

- **WHEN** 用户当日发帖数超过该规则的 `daily_cap` 折算上限
- **THEN** 超出部分 MUST NOT 计入当日 XP

### Requirement: 连续签到按里程碑一次性奖励

连续签到奖励 MUST 按「连续天数**恰好达到**某个里程碑」计一次（达到 3 / 7 / 15 / 30 / 60 / 100 天各一次），MUST NOT 按「每满 N 天重复计」。断签后重新累计并再次达到里程碑时 MUST 可再次获得。里程碑天数与奖励值 MUST 可配置。

#### Scenario: 达到里程碑给一次奖励

- **WHEN** 用户连续签到天数达到 30 天
- **THEN** 该用户 MUST 获得 30 天档的奖励且仅一次

#### Scenario: 同一轮连续期不重复领取

- **WHEN** 用户连续签到达到 60 天
- **THEN** 系统 MUST NOT 因为"每满 30 天"再补发一次 30 天档奖励

#### Scenario: 断签后重新累计可再次获得

- **WHEN** 用户连续签到 30 天后断签，之后又重新连续签到到 30 天
- **THEN** 该用户 MUST 再次获得 30 天档奖励

### Requirement: 等级由累计 XP 定级并由滚动窗口保级

等级 MUST 由「累计 XP 门槛」定级，并由「滚动窗口保级条件」约束其保留，因此等级 MUST 可升可降。

- 定级：取满足 `min_xp ≤ total_xp` 的最高等级。
- 保级：该等级若配置了保级条件（近 `window_days` 天内的活跃天数、发帖、获赞、被举报上限等），MUST 校验；不满足时 MUST 逐级下探，取第一个「`min_xp` 与保级条件同时满足」的等级，MUST NOT 一次降至最低级。
- 标记为**仅人工授予**的等级 MUST NOT 参与自动升降级。
- **「活跃天数」的定义是唯一的**：该用户在某个自然日内发生过登录，或产生过内容（发帖 / 评论 / 签到），该日即计为一个活跃日。MUST NOT 由各调用方各自解释。
- **「被举报」口径**：计入保级条件的被举报数 MUST 只统计**已受理成立**的举报，MUST NOT 把任意举报都计入，否则可被恶意举报压制他人等级。
- 评估时机 MUST 覆盖三类：行为引发的 XP 变化（事件驱动）、定时重算（处理不活跃导致的衰减降级）、读时惰性重算（快照过期时）。

#### Scenario: 达到 XP 门槛且满足保级条件则升级

- **WHEN** 用户 `total_xp` 达到某等级的 `min_xp`，且近 `window_days` 天的窗口指标满足该等级的保级条件
- **THEN** 该用户等级 MUST 变为该等级

#### Scenario: 不活跃导致降级

- **WHEN** 用户等级为 3，但其近 `window_days` 天窗口指标不再满足等级 3 的保级条件，且没有产生任何 XP 事件
- **THEN** 定时重算 MUST 将其等级降到仍然合格的最高等级
- **AND** MUST NOT 直接降到等级 0

#### Scenario: 保级指标不合格时不越级上升

- **WHEN** 用户 `total_xp` 达到等级 3 的 `min_xp`，但近 `window_days` 天窗口指标只满足等级 1 的保级条件
- **THEN** 该用户等级 MUST 为 1

#### Scenario: 仅人工授予的等级不自动升降

- **WHEN** 某等级被标记为仅人工授予，而某用户 `total_xp` 已达到其 `min_xp`
- **THEN** 系统 MUST NOT 自动把该用户升到该等级

### Requirement: 等级与 XP 是派生缓存且写入单点

`users.trust_level`、`users.total_xp`、等级指标快照 MUST 被视为派生缓存，MUST NOT 作为权威数据源。除等级评估服务外，任何 service MUST NOT 直接写这些字段；等级评估服务 MUST 是唯一写入方。

等级门控 MUST 对所有角色生效，MUST NOT 存在「等级 > 0 直接放行」之类的硬编码短路；需要豁免 MUST 通过显式规则表达。

#### Scenario: 其它服务不得直写等级

- **WHEN** 任意业务服务（签到、发帖、点赞等）需要更新用户等级
- **THEN** 它 MUST 调用等级评估服务，MUST NOT 自行写 `trust_level` / `total_xp` 字段

#### Scenario: 管理员手工设级不被打回

- **WHEN** 管理员把某用户等级手工设为高等级
- **THEN** 该用户此后因发帖/签到触发的重算 MUST NOT 仅凭 XP 把它静默改回低等级；若与 XP/保级条件冲突，系统 MUST 以管理员意图为准并留下记录

### Requirement: 游客与刚注册用户同处 TL0

未登录的公共游客 MUST 在等级比较中被视为等级 0；刚注册的用户 MUST 从等级 0 起步。等级 0 MUST 是等级体系中的一个**正式等级**（可配置名称），MUST NOT 表示为"没有等级"。

#### Scenario: 游客按 TL0 参与判定

- **WHEN** 未登录访客访问一个要求「等级 ≥ 1」才能查看的版面
- **THEN** 系统 MUST 按等级 0 判定，MUST 拒绝访问

#### Scenario: 新注册用户从 TL0 起步

- **WHEN** 用户完成注册
- **THEN** 其等级 MUST 为 0

### Requirement: 等级定义必须后台可配

等级数量、等级名称、XP 门槛、保级条件与窗口天数、连续签到阶梯、各指标的 XP 分值、日上限 MUST 全部可通过后台配置读写，MUST NOT 硬编码在代码常量中。等级数量 MUST 可增删，MUST NOT 固定为编译期常量。等级名称 MUST 可改，前端 MUST 从配置读取名称，MUST NOT 硬编码中文标签。

配置变更 MUST 在无需重启进程的前提下生效。

#### Scenario: 运营新增一个等级

- **WHEN** 运营在后台新增一个等级（名称 / XP 门槛 / 保级条件）
- **THEN** 该等级 MUST 参与定级与保级求值
- **AND** 无需重启后端进程

#### Scenario: 改等级名称前端即刻反映

- **WHEN** 运营把某等级名称从「常客」改为「资深」
- **THEN** 用户主页与他主页展示的等级名称 MUST 都变为「资深」（MUST NOT 出现一处显示原文 key、另一处显示旧中文）

#### Scenario: 调整保级窗口

- **WHEN** 运营把保级窗口从 100 天改为 60 天
- **THEN** 下一次等级评估 MUST 使用 60 天窗口

### Requirement: 人工调整 XP 必须可追溯

后台对用户 XP 的人工/活动加减分 MUST 写入独立记录（用户、增量、原因、操作者、时间），且 MUST 计入 `total_xp` 求值。MUST NOT 直接改写 `total_xp` 冗余列而不留记录。

后台用户详情 MUST 展示该用户 XP 的**指标分解**（每个指标贡献多少 XP），使用户可自查等级来源。

#### Scenario: 后台给用户加分

- **WHEN** 运营在用户详情页给某用户加 100 XP 并填写原因
- **THEN** 系统 MUST 写入一条可查询的调整记录（含操作者与原因）
- **AND** 该用户的 `total_xp` MUST 增加 100

#### Scenario: 撤销人工加分

- **WHEN** 运营删除或反向该调整记录
- **THEN** 该用户 `total_xp` MUST 回到调整前的水位

#### Scenario: 展示 XP 指标分解

- **WHEN** 运营打开某用户详情
- **THEN** 页面 MUST 按指标列出 XP 贡献（如「帖子 12 篇 → 120 XP」「签到 30 天 → 60 XP」）
- **AND** 各项之和（含人工调整）MUST 等于该用户的 `total_xp`
