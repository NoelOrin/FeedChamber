# Tasks

提案：[proposal.md](./proposal.md) · 设计：[2026-11-09-backend-domain-consolidation-design.md](../../../docs/superpowers/specs/2026-11-09-backend-domain-consolidation-design.md)

> 执行前置：`develop` 当前有 32 个未提交改动（email/settings 相关）。MUST 从干净的 `HEAD` 开独立 worktree，MUST NOT 在脏工作区做搬迁。

## 1. P0 工位与护栏

- [ ] 1.1 从 `develop` 的 `HEAD` 开 worktree，确认工作区干净
- [ ] 1.2 记录基线：`go build ./... && go vet ./... && go test ./...` 的包数与通过状态（实测 26 包 / 18 含测试）
- [ ] 1.3 记录基线数字：路由 `315 POST + 36 GET`；`AutoMigrate` `73` 个 `model.*`；`service` `91 文件 / 16094 行`；`handler` `48 / 10684`；`repository` `106 / 6830`；`model` `60 / 3578`
- [ ] 1.4 记录基线 `openapi.json` 的 sha256（`make swag` 后取），作为搬迁第一验收项
- [ ] 1.5 新库跑一次 `feedchamber_test.sh -all`，把通过/失败清单存档为行为基线
- [ ] 1.6 写域依赖快照脚本（解析 service 包接口字段依赖，输出 T1..T6 分层与全部跨域边），产物入库以便 P4 对比
- [ ] 1.7 新增 `internal/app/importboundary_test.go`：域顺序单向、域内 `service` 不 import `handler`、任何 `handler` 不 import `repository`/`db`，失败输出「文件 → 被禁包 → 规则名」
- [ ] 1.8 确认 10 个共享 helper 清单与处置（见设计 §4.4），逐条落到具体目标文件

## 2. P1 core：抽出 T1 共享服务

- [ ] 2.1 新建 `internal/core/setting/`，迁 `service/setting.go` + `setting_iface.go`（含 handler）
- [ ] 2.2 新建 `internal/core/permission/`，迁 `service/permission.go` + `permission_iface.go` + `perm_seed.go`
- [ ] 2.3 新建 `internal/core/email/`，迁 `service/email.go` + `email_iface.go` + `digest.go`
- [ ] 2.4 新建 `internal/core/audit/`，迁 `service/audit_log.go` + `audit_log_iface.go`
- [ ] 2.5 新建 `internal/core/session/`，只放被服务层消费的接口形状（实现仍留 `internal/gateway`）
- [ ] 2.6 新建 `internal/core/mcpclient/`，迁 `service/mcp_server.go` + `mcp_server_iface.go`，解决 `moderation → mcp` 的逆边
- [ ] 2.7 新建 `internal/pkg/httpx/`，迁 `handler/ctx.go` 与 `handler/params.go` 的共享部分
- [ ] 2.8 提交时验证：`go build/vet/test` 绿、`openapi.json` 哈希不变、`feedchamber_test.sh -all` 与基线一致

## 3. P1 growth 试点

- [ ] 3.1 新建 `internal/growth/service/`，迁 `trust_level.go` `trust_iface.go` `gate.go` `gate_iface.go` `visibility.go` `checkin.go` `checkin_iface.go` `badge.go` `leaderboard.go`
- [ ] 3.2 新建 `internal/growth/handler/`，迁 `trust.go` `gate.go` `checkin.go` `badge.go` `leaderboard.go`，并建立该域 handler 聚合体
- [ ] 3.3 处置 helper：`gateInputForUser` / `gateDenyMessage` 导出为域内公开函数
- [ ] 3.4 逐域验证同上（build/vet/test、openapi 哈希、API 脚本）
- [ ] 3.5 试点复盘：记录实际耗时、边数与设计预测的偏差；偏差影响后续域顺序时 MUST 在 proposal 中更新

## 4. P2 identity

- [ ] 4.1 迁 auth / mfa / totp / user / role / group / invite / banned / oauth 至 `internal/identity/service/`
- [ ] 4.2 迁对应 handler 至 `internal/identity/handler/` 并建域聚合体
- [ ] 4.3 处置 helper：`displayNameOf` 导出为 `identity` 的公开函数；`validateDisplayName` 留域内
- [ ] 4.4 验证同上

## 5. P2 engagement

- [ ] 5.1 迁 like / favorite / follow / watch / user_block / notification / pm / report 至 `internal/engagement/service/`
- [ ] 5.2 迁对应 handler 至 `internal/engagement/handler/` 并建域聚合体
- [ ] 5.3 保持通知扇出去重（`notified` 集合按「作者 > 提及 > 关注」优先级）语义不变，搬迁 MUST NOT 触碰
- [ ] 5.4 验证同上

## 6. P2 content

- [ ] 6.1 迁 post / content / tag / space / draft / revision / moderation / mention / slug 至 `internal/content/service/`
- [ ] 6.2 迁对应 handler 至 `internal/content/handler/` 并建域聚合体
- [ ] 6.3 处置 helper：`publishUserMentions` `isValidPostStatus` 留域内
- [ ] 6.4 `moderation` 对 `core/mcpclient` 的依赖确认走 T5→T1
- [ ] 6.5 验证同上；`postService` 18 个依赖在搬迁后仍应可在域内一次读完

## 7. P2 ops

- [ ] 7.1 迁 admin / announcement / backup(+snapshot) / webhook / plugin / motion / customize / theme / upload / slot 至 `internal/ops/service/`
- [ ] 7.2 迁对应 handler（含 dashboard / seo / realtime / trash）至 `internal/ops/handler/` 并建域聚合体
- [ ] 7.3 处置 helper：`truncate` / `mapRepoErr` 按拍板结果移入 `internal/pkg` 或域内去重
- [ ] 7.4 验证同上

## 8. P2 收尾

- [ ] 8.1 删除已空的 `internal/service/` 与 `internal/handler/` 目录
- [ ] 8.2 全仓确认无残留 import 指向旧路径
- [ ] 8.3 跑一次完整基线比对：路由数、AutoMigrate 数、生产 LOC 净变化为 0、openapi 哈希相等

## 9. P4 文档与规范收口

- [ ] 9.1 按新目录重写/合并受影响的模块 `AGENTS.md`（45 份中的后端部分）
- [ ] 9.2 更新 `docs/ARCHITECTURE.md` 层表：层 → 域，补各域文档链接
- [ ] 9.3 更新 `docs/system-map.md` 的目录与职责表
- [ ] 9.4 用实测数字重写 `docs/backend-overview.md`（当前记载 253 路由/55 模型/9876 行均已失真，实测 351/73/16094）
- [ ] 9.5 `node scripts/check-agents-docs.mjs --write-index` 重建根 `AGENTS.md` 索引
- [ ] 9.6 更新根 `AGENTS.md` 的「项目结构」与「新增模块 checklist」
- [ ] 9.7 `make swag-check`、文档门禁、`openspec validate --all --strict` 全绿
- [ ] 9.8 `openspec archive 2026-11-09-backend-domain-consolidation -y`，归档后再 validate 一次

## 10. 未纳入本次（另开 change）

- [ ] 10.1 `internal/app/` 装配收拢 + `iris.go` 754 行 `Start()` 拆成具名 bootstrap 步骤（见 proposal「待拍板」1）
- [ ] 10.2 前端后台 CRUD 脚手架（`app/web/app/admin`，7 个 470–841 行同构页面），建议 change 名 `admin-console-scaffold`
