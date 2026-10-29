## Why

Feed Chamber 现有「举报」是**单向**机制：用户 → 管理员，由版主/管理员私裁处理。它解决「违规」，但覆盖不了**规范灰色地带**——内容不违规、却引发争议，社区想集体表态却无处可去，只能私下抱怨或升级成对抗。

引入**社区自治动议（Community Motion）**：把「社区就规范灰区的集体裁判」做成一等公民。它与「举报」**并存两套**——举报交给管理员，动议交给社区集体表决。动议是治理工具，不是娱乐、不是猎巫、不取代版主处罚权。

## What Changes

- **新增「自治动议」子系统**，独立于现有 `Report`：
  - 任何登录用户可对 `post` / `comment` / `user` 发起动议，类型：`fold`（折叠/降权）、`watch`（观察区，限时标记）、`escalate`（移交版主）、`exile`（放逐至暗区）。
  - 动议进入**公示-表决**两阶段。表决采用**法定人数 + 信誉加权多数**：参与人数需达版面动态法定人数；每张票按发起人/投票人信誉折算权重（高信誉用户票权更高，稀释水军与小号）。
  - 通过后的结果：
    - 折叠（帖子降权、UI 标记）、观察区（限时标记）、移交版主（生成一条 `report:manage` 队列项，由版主最终裁定）。
    - **放逐至暗区（exile）**：目标进入软隔离区。帖子被聚合进 `/版/暗区` 且不进主推荐流；用户被放逐后其新内容自动路由到暗区，但账号默认不封、不删号、不删内容。`exile` 以软隔离为基准。
  - **硬处罚的边界（视情况而定）**：常规自治结算**不自动**执行封禁或封号——删除与封禁始终只走版主/管理员的既有权限（`Banned` 表 / `role=ban` / `report:manage`）。但当动议结果严重（如 `exile` 且伴随骚扰、违规证据，或移交版主后核实触犯红线）时，**可经版主/管理员裁量升级为硬封禁或封号**，而非由动议机器自动执行。自治与硬处罚的边界是「社区表决 → 管理裁量」，不是彻底剥离。
  - **暗区（/版/暗区）语义**：一个普通版面，聚合被放逐内容；主站 feed 永不推荐，仍可访问、可被搜索（可配置）。用户可在暗区内正常互动（除非已被硬封禁）。
  - **翻案机制**：对 `exile`/折叠结果可发起反向动议（reverse），达同等门槛即解除软隔离；版主凭 `motion:manage` 也可直接解除，或将软隔离升级为硬封禁/封号。
  - **反猎巫护栏**：动议被表决驳回且判定为恶意（由版主或自动规则标记）→ 发起者记「滥诉」记录，累计达阈值后限权（禁止发起新动议一段时间）。恶意判定不自动发生，需经 `report:manage` 角色确认。
- **信誉（Reputation）双轴化**：在现有 `points`/积分体系上新增**信誉分（reputation）**，与内容质量/互动挂钩，作为票权与发起资格的依据。本期仅新增 `reputation` 字段与基础累加点，不做复杂衰减。
- **复用现有基础设施**：EventBus 发 `motion.created` / `motion.resolved`；SSE 通知订阅者；gateway 权限映射新增 `motion:create` / `motion:manage`；沿用 `pkg.Success` / `AppError` / RPC 风格。
- **与举报的边界（明确并存）**：
  - `Report`：疑似违规 → 管理员私裁 → 可封禁/删内容。
  - `Motion`：灰区争议 → 社区加权表决 → 折叠/观察/移交/放逐，其中放逐=软隔离到暗区（无硬处罚）。
  - 移交类动议的结果会落进举报队列，二者在「移交版主」处交汇，但不共享状态机。
  - 暗区是放逐结果的收容区，由自治体系产生、版主可干预解除，与硬封禁（Banned 表/role=ban）完全独立。

## Capabilities

### New Capabilities

- `community-motion`: 社区自治动议子系统——动议的发起、公示、信誉加权表决、结果执行（折叠/观察/移交）、反猎巫护栏，以及与现有举报体系的并存边界。

### Modified Capabilities

- `module-conventions`: 新增模块需登记权限码（`motion:create` / `motion:manage`）、gateway 路径映射、api-client 类型与 service，与既有 `report` 模块保持同构。新增的系统 Space `dark-zone` 遵循既有 `Space` 种子模式（幂等 seed、不在主推荐）。

## Impact

- **后端（新增）**：`internal/model/motion.go`、`internal/repository/motion.go`、`internal/service/motion.go`(+`motion_iface.go`)、`internal/handler/motion.go`、`internal/pkg/eventbus/events.go`(新增 2 事件)。
- **后端（修改）**：
  - `internal/model/permission.go`：新增 `PermMotionCreate` / `PermMotionManage` 及权限名；`DefaultRolePermissions` 中 `admin`/`moderator` 加 `motion:manage`，`user` 加 `motion:create`。
  - `internal/model/user.go`（或信誉表）：新增 `reputation` 字段/累加点。
  - `internal/gateway/permission.go`：新增 `/api/v1/motions/...` 路径→权限映射。
  - `internal/handler/handler.go`：聚合 `MotionHandler` + `NewMotionHandler` 接线。
  - `internal/container/container.go`：构造 `MotionRepo`/`MotionSvc` 并接入 EventBus。
  - `server/iris.go`：AutoMigrate 加 `Motion`；`server/seed.go` 已幂等 seed 权限（无需改）。
  - `internal/router/`：新增 `motions` 路由组（遵循 RPC 风格：`/create`、`/list`、`/{id}/vote`、`/{id}/status`、`/{id}/delete`）。
- **前端（新增）**：
  - `packages/api-client/src/types.ts`：新增 `Motion`/`MotionType`/`MotionStatus` 类型。
  - `packages/api-client/src/constants.ts`：新增 `MOTION_CREATE` / `MOTION_MANAGE` 权限码。
  - `packages/api-client/src/services/motion.ts`：动议 service 模块。
  - `app/web`：`/版/自治大厅` 页面 + 帖子卡片「发起自治动议」按钮 + 「进行中动议」侧栏（参考 demo 交互）。
- **破坏性变更**：无。新增独立模块，不改动 `Report` 任何既有行为；自治动议常规结算不执行硬处罚，但保留「经管理裁量升级硬封禁/封号」的通道，移交版主仍走既有 `report:manage` 流程；暗区为新增软隔离版面，不影响既有 `Space` 体系。
- **不在本轮范围**：盐经济、赌局、毒性打分、自动 AI 判定滥诉。这些在 demo 中已弱化剔除，本轮不实现；暗区/放逐已纳入本轮。
