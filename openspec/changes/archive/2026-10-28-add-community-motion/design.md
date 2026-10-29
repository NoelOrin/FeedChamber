## Context

OrinForum 后端已有完整分层（`handler → service → repository → db`），统一响应 `pkg.Success/HandleError`、RPC 风格 URL、gateway 两段式鉴权（Preflight/Enforce）与 path→permission 映射、`eventbus` 事件总线、`kvstore` 缓存。现有 `Report` 模块是「用户单向上报 + 管理员私裁」的范本，本变更要做的 `Motion` 子系统与之**同构并存**，不改动 `Report` 任何行为。暗区以「新增一个软隔离 Space」的形式落地，复用现有 `Space`/`Post` 模型，避免引入平行的内容隔离体系。

## Goals / Non-Goals

**Goals:**
- 提供独立于 `Report` 的社区集体裁判通道（动议 + 信誉加权表决）。
- 结果柔性化：折叠 / 观察 / 移交版主 / 放逐（暗区软隔离），不执行硬封禁、不删号、不删内容。
- 暗区作为一个普通 Space 存在，主推荐流不收录，可访问、可翻案。
- 反猎巫：恶意动议记滥诉、限权，但恶意判定需 `motion:manage` 确认，不自动发生。

**Non-Goals:**
- 不做盐经济、赌局、毒性打分、AI 自动判滥诉。
- 不改动 `Report` 状态机、不共享 `Motion` 与 `Report` 的状态。
- 不改变版主/管理员既有的封禁、删除权限（那些走既有 `report:manage` / `Banned` / `role=ban`）。

## Decisions

### D1. 动议状态机
`pending`（公示中，仅收集投票）→ `active`（表决期，达法定人数前）→ `resolved`（`passed` / `rejected`）。
- 公示与表决合并为单一开放期（demo 的 24h），开放期内持续计信誉加权票；开放期满或达法定人数即结算。
- 反对与赞成同权（均计入法定人数分母与加权分子），通过条件：加权赞成 > 加权反对 **且** 加权总参与 ≥ 版面法定人数。
- 选择「单一开放期」而非 demo 的两段式，是因为实现更简单、用户认知成本更低；法定人数用版面动态值（如版面活跃用户数的百分比，配置默认 20）。

### D2. 信誉加权票权
- 新增 `users.reputation`（int，默认 0）；票权 = `1 + min(reputation/1000, 1)`（上限 ×2，稀释极端值）。发起动议资格要求 `reputation >= 0`（所有人可发起，靠反猎巫护栏兜底）。
- 替代方案（按毒性的双轴声誉、按历史投票准确率加权）成本过高，本期不做；信誉仅做正向累积（发帖被赞、互动），不做衰减。

### D3. 暗区 = 软隔离 Space
- 新增一个系统 Space（slug=`dark-zone`，`system=true`，`recommendable=false`），不出现在版面大全主推位置。
- 被 `exile` 通过的帖子：标记 `posts.space_id = dark-zone`，并置 `posts.exiled=true`（UI 折叠标记 + 主推荐过滤）。
- 被 `exile` 通过的用户：置 `users.exiled_until`（可空=永久，否则限时）；其**新发帖**在 service 层路由到 dark-zone。`exiled` 不等于 `role=ban`，登录与浏览默认不受影响。
- **硬处罚升级通道（视情况而定）**：常规 `exile` 仅软隔离。但当情节涉及骚扰/违规红线，或移交版主后核实触发硬规则时，版主/管理员可凭 `motion:manage` 或既有 `Banned`/`role=ban` 权限，将软隔离**升级为硬封禁或封号**。这一步是人工裁量，不由动议状态机自动执行——避免社区机器直接删号。
- 翻案：对 `exile`/fold 结果发起 `reverse` 类型动议，达同等门槛解除软隔离；版主也可直接解除，或将软隔离升级为硬封禁/封号。

### D4. 与 Report 的交汇点
仅 `escalate` 类型：动议 passed 后，service 创建一条 `Report`（`flag_type="community_escalation"`，`status=pending`），进入既有 `report:manage` 队列。二者不共享状态机；动议自身已 resolved。

### D5. 反猎巫护栏
- `motions` 表记录 `abuse_flag` 与 `initiator_id`。
- 动议 rejected 后，可由 `motion:manage` 角色在 `/motions/{id}/status` 标记 `abuse=true`；发起者 `users.abuse_count++`，达阈值（默认 3）自动置 `users.motion_banned_until`（限时禁止发起，默认 7 天）。
- 不自动判恶意：避免误伤正常争议讨论。

### D6. 事件与通知
- `eventbus` 新增 `EventMotionCreated` / `EventMotionResolved`；`NotificationService` 订阅，向动议发起者、目标、订阅者推送（复用 SSE `/stream`）。
- 不新建 SSE 端点，复用现有通知通道。

### D7. API 形态（RPC 风格，遵循 api-style）
- `POST /motions/create`（body: target_type,target_id,type,reason）
- `POST /motions/list`（body: status,space_id,page,pageSize）—— 读，但遵循「列表 POST + /list」
- `POST /motions/{id}/vote`（body: side=approve|reject）
- `POST /motions/{id}/status`（body: status,abuse?）—— `motion:manage`，用于翻案/标记滥诉
- `POST /motions/{id}/delete` —— 发起者撤回（仅 pending）
- `GET /spaces/dark-zone`（暗区内容列表，复用 space 读取）

## Risks / Trade-offs

- [风控不足导致小号刷票] → 信誉加权 + 法定人数 + 单一账号每动议仅一票（唯一约束：`motion_votes(motion_id, voter_id) UNIQUE`）。
- [暗区变成猎巫乐园] → exile 是软隔离非荣耀；翻案与版主解除通道常驻；主推荐永不收录，降低炫耀动机。
- [动议被滥用骚扰个人] → user 类动议默认更高法定人数；exile 用户走软隔离不影响登录；恶意判定需 `motion:manage`。
- [与 Report 职责混淆] → 文档与 UI 明确分流：举报=违规→管理员；动议=灰区→社区；仅在 escalate 交汇。

## Migration Plan

1. 新表 `motions` / `motion_votes` 由 AutoMigrate 创建；新增 `users.reputation` / `users.abuse_count` / `users.exiled_until` / `users.motion_banned_until` 列随 `User` AutoMigrate 加列（SQLite/MySQL/PostgreSQL 均支持）。
2. 系统 Space `dark-zone` 由 seed 幂等创建（仿照现有种子模式，`alreadySeeded` 门控）。
3. 权限码 `motion:create` / `motion:manage` 走 `PermissionService` 幂等 seed，重启不覆盖自定义。
4. 回滚：本变更为纯增量（新表/新列/新权限/新 Space），不改动既有 schema 与 `Report`；回滚只需删除新增模块代码与权限码，遗留空表无副作用。

## Open Questions

- 版面级动态法定人数是否需后台可调？本期用全局默认 + 大版面系数，后续可做 Space 设置项。
- 信誉初始值如何分发（新用户 0，是否给老用户批量初始化）？本期默认 0，后续可脚本补。
