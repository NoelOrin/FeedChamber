## Purpose

让社区就规范灰色地带进行集体裁判（自治动议），与既有的「用户→管理员」举报体系并存；结果柔性化（折叠/观察/移交/暗区软隔离），硬封禁与封号保留为版主/管理员的人工裁量升级通道。

## ADDED Requirements

### Requirement: 用户可发起社区自治动议
系统 SHALL 允许已登录用户对 `post` / `comment` / `user` 发起自治动议，指定类型 `fold`（折叠）、`watch`（观察区）、`escalate`（移交版主）、`exile`（放逐暗区）并附理由。

#### Scenario: 发起一个帖子折叠动议
- **WHEN** 已登录用户对任意帖子提交 `type=fold` 的动议
- **THEN** 系统创建一条 `status=pending` 的动议记录，进入公示-表决期，并向发起者返回该记录

#### Scenario: 未登录不可发起
- **WHEN** 匿名请求 `POST /motions/create`
- **THEN** 系统拒绝并返回未认证错误

#### Scenario: 被限权用户不可发起
- **WHEN** `users.motion_banned_until` 在有效期内（滥诉达阈值）的用户发起动议
- **THEN** 系统拒绝并返回「发起权限已暂停」错误

### Requirement: 动议采用信誉加权表决
系统 SHALL 对动议进行公示-表决：每张票按投票人 `reputation` 折算票权（参考 `1 + min(reputation/1000, 1)`），同一用户对同一动议仅能投一票；动议通过需满足「加权赞成 > 加权反对」且「加权总参与 ≥ 版面法定人数」。

#### Scenario: 达法定人数且加权赞成占优
- **WHEN** 某动议加权总参与达到版面法定人数，且加权赞成票数大于加权反对
- **THEN** 动议结算为 `passed`，触发对应结果执行

#### Scenario: 未达法定人数
- **WHEN** 表决期满加权总参与仍低于版面法定人数
- **THEN** 动议结算为 `rejected`，不执行任何结果，不记滥诉

#### Scenario: 单一用户重复投票
- **WHEN** 同一用户对同一动议发起第二次投票
- **THEN** 系统拒绝第二次投票（唯一约束 `motion_votes(motion_id, voter_id)`）

### Requirement: 动议结果执行柔性化
系统 SHALL 在动议 `passed` 后按类型执行：折叠（内容降权+UI 标记）、观察区（限时标记）、移交版主（生成 `report:manage` 队列项）、放逐暗区（内容路由至暗区软隔离）。

#### Scenario: exile 通过→帖子进入暗区
- **WHEN** 针对某帖子的 `exile` 动议 `passed`
- **THEN** 该帖子 `space_id` 置为暗区系统版面且 `exiled=true`，主推荐流不再收录

#### Scenario: exile 用户→软隔离而非硬封禁
- **WHEN** 针对某用户的 `exile` 动议 `passed`
- **THEN** 系统置该用户 `exiled_until`（可空=永久/限时），其后续新发内容路由至暗区；账号仍可登录与浏览（除非另有硬封禁）

#### Scenario: 移交版主落举报队列
- **WHEN** 针对某目标的 `escalate` 动议 `passed`
- **THEN** 系统创建一条 `flag_type=community_escalation`、`status=pending` 的 `Report`，进入 `report:manage` 队列

### Requirement: 暗区为可访问的软隔离版面
系统 SHALL 提供暗区版面（系统 Space，`recommendable=false`），聚合被放逐内容，主站 feed 不推荐但仍可访问与搜索（可配置）。

#### Scenario: 访问暗区
- **WHEN** 任意用户访问暗区版面列表
- **THEN** 系统返回被放逐内容，且该版面不出现在主推荐版面大全

### Requirement: 硬封禁/封号保留为管理裁量升级
系统 SHALL 不自动执行封禁或封号；但当动议结果严重（如 `exile` 伴随骚扰/违规证据，或移交版主后核实触发红线）时，版主/管理员可凭 `motion:manage` 或既有 `Banned`/`role=ban` 权限将软隔离升级为硬封禁或封号。

#### Scenario: 版主将软隔离升级为硬封禁
- **WHEN** 版主对某 `exile` 结果判定需硬处罚，操作升级
- **THEN** 系统在保留软隔离标记的同时，对该用户执行既有硬封禁流程（`Banned` 表或 `role=ban`），登录/写操作受既有网关拦截

#### Scenario: 动议机器不自动删号
- **WHEN** 任意动议（含 `exile`）结算完成
- **THEN** 系统不自动删除账号、不自动写入硬封禁记录；硬处罚仅由 `motion:manage`/管理员人工触发

### Requirement: 反猎巫与翻案
系统 SHALL 提供反猎巫护栏：`rejected` 动议可由 `motion:manage` 标记为滥诉，发起者 `abuse_count` 递增，达阈值后限时禁止发起新动议。系统 SHALL 允许对 `exile`/折叠结果发起反向动议（reverse）或经 `motion:manage` 直接解除。

#### Scenario: 滥诉达阈值自动限权
- **WHEN** 某用户被标记的滥诉次数达到阈值（默认 3）
- **THEN** 系统置 `motion_banned_until` 为限时未来时间，期间其发起动议被拒

#### Scenario: 反向动议解除软隔离
- **WHEN** 针对某 `exile` 结果的反向动议达同等门槛通过
- **THEN** 系统解除该目标的软隔离标记（帖子还原原版面 / 清除 `exiled_until`）

### Requirement: 动议事件通知
系统 SHALL 在动议创建与结算时发布 `motion.created` / `motion.resolved` 事件；通知服务订阅后向发起者、目标、订阅者推送，复用既有 SSE 通道。

#### Scenario: 动议结算通知
- **WHEN** 某动议结算为 `passed` 或 `rejected`
- **THEN** 系统发布 `motion.resolved` 事件，相关用户收到通知（不新建 SSE 端点）

### Requirement: 自治动议与举报并存且职责分离
系统 SHALL 维持 `Report`（违规→管理员私裁，可硬处罚）与 `Motion`（灰区→社区加权表决，柔性结果）两套独立机制；二者仅在 `escalate` 类型落举报队列处交汇，不共享状态机、不互相改写对方状态。

#### Scenario: 举报与动议互不影响
- **WHEN** 对同一目标同时存在 `Report` 与 `Motion`
- **THEN** 两者的状态流转相互独立，任一方结算不改变另一方的状态字段
