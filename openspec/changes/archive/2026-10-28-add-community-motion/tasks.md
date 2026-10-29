## 1. 数据模型与权限

- [x] 1.1 新增 `internal/model/motion.go`：`Motion`（`target_type`/`target_id`/`type`/`reason`/`status`/`initiator_id`/`abuse_flag`/`exiled_until` 等）、`MotionVote`（`motion_id`/`voter_id` UNIQUE、`weight`、`side`）、`MotionReverse` 关联；`Motion`/`MotionVote` `TableName()`。
- [x] 1.2 在 `internal/model/user.go` 为 `User` 增加字段：`reputation`（默认 0）、`abuse_count`（默认 0）、`exiled_until *time.Time`、`motion_banned_until *time.Time`；保证 AutoMigrate 加列（sqlite/mysql/postgres 兼容）。
- [x] 1.3 在 `internal/model/permission.go` 新增 `PermMotionCreate = "motion:create"`、`PermMotionManage = "motion:manage"` 常量，加入权限定义列表（`{Code,Name,Description}`）；在 `DefaultRolePermissions` 中 `admin`/`moderator` 追加 `motion:manage`，`user` 追加 `motion:create`。
- [x] 1.4 在 `server/seed.go` 幂等 seed 系统 Space `dark-zone`（`slug=dark-zone`、`system=true`、`recommendable=false`、`name=/版/暗区`），沿用 `alreadySeeded` 门控。

## 2. Repository 层

- [x] 2.1 新增 `internal/repository/motion.go`：`MotionRepository` 接口 + `sqlMotionRepo` 实现（`WrapDBErr` 统一错误）：Create、FindByID、FindPendingByTarget、List（按 status/space 过滤+分页）、Count、AddVote（唯一冲突处理）、FindVote、IncrementWeightedTally、UpdateStatus、MarkAbuse、Delete。
- [x] 2.2 在 repository 构造处（`container.go` 或 `repository` 工厂）登记 `NewMotionRepository(store)`。

## 3. Service 层

- [x] 3.1 新增 `internal/service/motion_iface.go`：`MotionService` 接口（Create/Vote/List/UpdateStatus/Delete/Resolve 等）。
- [x] 3.2 新增 `internal/service/motion.go`：`motionService` 实现，构造依赖 `MotionRepository` + `PostRepository` + `ContentRepository` + `UserRepository` + `SpaceRepository` + `ReportService` + `PermissionService` + `EventBus` + `BannedService`（仅用于升级通道的既有封禁调用，不自动触发）。
- [x] 3.3 实现 Create：校验 targetType∈{post,comment,user}、检查发起者 `motion_banned_until`、写 `Motion`（status=pending）、发布 `motion.created`。
- [x] 3.4 实现 Vote：按投票人 reputation 算 weight（`1+min(rep/1000,1)`），唯一约束防止重复投票；更新加权 tally。
- [x] 3.5 实现结算（Resolve）：加权总参与≥版面法定人数且赞成>反对→`passed` 执行结果；否则 `rejected`。结果执行：fold→内容降权标记；watch→限时标记；escalate→生成 `Report(flag_type=community_escalation)`；exile→帖子路由 dark-zone（`exiled=true`）/ 用户置 `exiled_until`；发布 `motion.resolved`。
- [x] 3.6 实现 UpdateStatus（`motion:manage`）：翻案 `overturned` 解除软隔离、标记 `abuse=true`（触发发起者 `abuse_count++`，达阈值置 `motion_banned_until`）、将软隔离**升级**为硬封禁/封号（调用既有 `BannedService` / 角色变更，需显式传参，不自动）。
- [x] 3.7 实现 Delete：仅 `pending` 且发起者可撤回。

## 4. Handler 与网关

- [x] 4.1 新增 `internal/handler/motion.go`：`MotionHandler` 接口 + `motionHandler` 实现 + swag 注解（RPC 风格：/motions/create、/motions/list、/motions/{id}/vote、/motions/{id}/status、/motions/{id}/delete）；统一 `pkg.Success`/`HandleError`。
- [x] 4.2 在 `internal/handler/handler.go` 聚合 `Motion MotionHandler` 字段 + `NewMotionHandler` 接线（依赖 `MotionService` 等）。
- [x] 4.3 在 `internal/container/container.go` 构造 `MotionRepo`/`MotionSvc` 并接入 `EventBus`，遵循现有两步装配（New + Start）。
- [x] 4.4 在 `internal/gateway/permission.go` 新增 path→permission 映射：`/api/v1/motions/create`→`motion:create`；`/api/v1/motions/list`→`motion:create`（读取自己参与/可见）；`/api/v1/motions/{id}`、`/motions/{id}/vote`→`motion:create`；`/motions/{id}/status`、`/motions/{id}/delete`→`motion:manage`（撤回由 service 按发起者校验）。
- [x] 4.5 在 `internal/router/` 新增 `motionRoutes()`：`protected.PartyFunc("/motions", ...)` 注册上述 RPC 路由（遵循 POST 写、POST /list 读约定）。
- [x] 4.6 在 `internal/pkg/eventbus/events.go` 新增 `EventMotionCreated` / `EventMotionResolved` 常量并加入事件集合；NotificationService 订阅并复用现有 SSE 推送（不新建端点）。

## 5. 前端（api-client）

- [x] 5.1 `packages/api-client/src/types.ts`：新增 `Motion`、`MotionType`（`fold|watch|escalate|exile`）、`MotionStatus`（`pending|passed|rejected|overturned`）、`MotionVote` 类型。
- [x] 5.2 `packages/api-client/src/constants.ts`：新增 `MOTION_CREATE: 'motion:create'`、`MOTION_MANAGE: 'motion:manage'` 权限码常量。
- [x] 5.3 `packages/api-client/src/services/motion.ts`：新增 `motionService`（create/list/vote/status/delete），遵循现有 report service 同构。

## 6. 前端（Web 页面与交互，参考 demo）

- [x] 6.1 新增「自治大厅」页面与暗区版面视图（复用 Space/Post 组件，暗区标记 `recommendable=false` 不进主推荐）。
- [x] 6.2 帖子/评论卡片增加「发起自治动议」按钮，弹窗选类型+填理由（参考 `demo/troll-bbs.html` 的交互）。
- [x] 6.3 侧栏「进行中动议」面板：列示 active 动议、法定人数进度、赞成/反对按钮、已结案切换。

## 7. 验证

- [x] 7.1 后端：`go build ./...` 通过；新增 `motion` 相关单测（service 结算、票权加权、反猎巫阈值、暗区路由）。
- [x] 7.2 用 `orinforum_test.sh` 扩展或手工 curl 走通：发起→投票达法定人数→passed→执行结果；rejected→标记滥诉→限权；exile→暗区路由。
- [x] 7.3 确认 `Report` 既有行为未被改动（回归不破坏现有举报测试）。
- [x] 7.4 `openspec validate add-community-motion` 通过；`make swag` 更新 openapi.json。
