# Discourse 对齐阶段 4：平台化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Index:** `docs/superpowers/plans/2026-07-19-discourse-alignment-index.md`  
> **Depends on:** 阶段 1–3 已合并；本阶段可按业务优先级拆 PR，不必一次做完。

**Goal:** 平台化能力：Groups/多身份、信任等级简化或完整 TL、备份导入、内部扩展点（非动态插件运行时优先）、可选 Chat/高级治理。

**Architecture:** Groups 作为权限主体逐步替代单一 `User.Role`（兼容期双读：role + group membership）；TL 用统计表 + 定时晋升；导入用 CLI `go run` 子命令；扩展点先标准化 EventBus 事件目录与 handler hook 接口，**不做**任意代码热加载插件。Chat 为独立模块可选。

**Tech Stack:** 同前；CLI 挂 `app/server/cmd`；备份用 SQL dump + 附件清单。

**DoD（分项可独立交付）：**
1. Groups CRUD + 成员 + 可绑定版面权限
2. 信任等级字段 + 定时晋升任务（至少 TL0–TL2 规则）
3. 备份导出 + Discourse 基础导入（用户/分类→版面/话题/帖）
4. 文档化扩展事件目录 + 官方模块注册表
5. （可选）Chat 频道 MVP 或明确不做决议

---

## File Map

| 能力 | 路径 |
|------|------|
| Groups | `model/group.go`、`group_member.go`、repo/svc/handler、Dashboard |
| 权限演进 | `service/permission.go` 读 group perms |
| TL | `model/user.go` TrustLevel；`service/trust.go`；cron |
| 备份 | `cmd/backup.go` 或 `scripts/backup.sh` + admin API |
| 导入 | `cmd/import_discourse.go` |
| 扩展点 | `pkg/eventbus/events.go` 常量全集；`docs/extension-points.md` |
| Chat 可选 | `model/chat_*.go` … |

---

### Task 1: Groups 基础

**Files:**
- Create: `model/group.go`、`group_member.go`
- Create: repo/svc/handler
- Modify: permission 解析：`HasPermission` 聚合 user.role + groups
- Dashboard：组管理
- 兼容：现有 admin/moderator/user/ban **自动组** seed

- [ ] **Step 1: 模型**

```go
type Group struct {
	ID          string `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex;size:64"`
	DisplayName string
	Visibility  string `gorm:"size:16;default:public"` // public|closed|secret
	IsSystem    bool
	CreatedAt   time.Time
}

type GroupMember struct {
	GroupID  string `gorm:"primaryKey"`
	UserID   string `gorm:"primaryKey"`
	Role     string `gorm:"size:16;default:member"` // owner|moderator|member
	JoinedAt time.Time
}
```

- [ ] **Step 2: RPC**

```
POST /groups/list|create|{id}/update|{id}/delete
POST /groups/{id}/members/list|add|remove
```

- [ ] **Step 3: 权限**

`PermissionService.HasPermission(userID/role, code)` 增加 group→permission 映射表 `group_permissions`（可复用 role_permissions 模式）。

过渡策略：`User.Role` 仍保留；登录时把 system group 与 role 同步。

- [ ] **Step 4: 测试** 组成员获得版面权限覆盖。

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(authz): groups membership and permission aggregation"
```

---

### Task 2: 信任等级（TL）

**Files:**
- Modify: `model/user.go` 加 `TrustLevel int`
- Create: `service/trust.go`
- Modify: seed 默认限制（TL0 外链、TL0 强制审核）
- Cron：每日晋升

- [ ] **Step 1: 规则（可配置）**

| 等级 | 条件（示例） |
|------|----------------|
| TL0 | 新注册 |
| TL1 | 阅读 10 帖 + 在线 10min + 耗时 1 天（阶段 4 可用发帖/登录天数近似） |
| TL2 | 获赞 10 + 回复 10 + 拒绝率低 |

最小实现：按 `posts_created`、`likes_received`、`days_since_join` 晋升 TL1/TL2。

- [ ] **Step 2: 门禁**

- TL0 + setting → 发帖进审（与 moderation 叠加）
- TL0 禁止帖内外链（Markdown 渲染 strip 或创建拒绝）

- [ ] **Step 3: API**

admin 可手动 `POST /user/{id}/trust_level/update` `{ level }`

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(trust): trust levels with scheduled promotion and gates"
```

---

### Task 3: 备份导出

**Files:**
- Create: `app/server/cmd/backup.go` 或 root cobra 子命令
- Optional: `POST /admin/backup/create` 仅本地路径

- [ ] **Step 1: CLI**

```bash
cd app/server && go run . backup --out /tmp/orinforum-backup
```

产出：
- `db.sql` 或 sqlite 文件拷贝
- `meta.json`（版本、时间、DB_TYPE）
- `uploads/` 清单或打包

- [ ] **Step 2: 权限** 仅运维 CLI；若 API 则 `role:manage` + 维护模式建议。

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(ops): site backup export command"
```

---

### Task 4: Discourse 导入器（MVP）

**Files:**
- Create: `app/server/cmd/import_discourse.go`
- Docs: `docs/import-discourse.md`

- [ ] **Step 1: 输入**

支持 Discourse 导出 JSON/或 PG 只读连接（二选一，**本任务选 JSON 导出目录**）：
- users.json
- categories.json → spaces
- topics.json + posts.json → posts/contents

- [ ] **Step 2: 映射**

| Discourse | OrinForum |
|-----------|-----------|
| user.id | 新 ULID，保留 external_id 映射表 |
| category | space |
| topic | post |
| post #1 | content root |
| replies | contents parent/root |

密码：随机 + 强制 first_change_password 或禁用密码仅 OAuth。

- [ ] **Step 3: 幂等**

`import_map` 表 `(source, external_id, local_id)` 避免重复导入。

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(import): discourse JSON importer MVP"
```

---

### Task 5: 扩展点文档与事件目录固化

**Files:**
- Create: `app/server/internal/pkg/eventbus/events.go`（若分散则集中）
- Create: `docs/extension-points.md`
- Modify: Webhook/Notification/Badge 均只订阅常量事件

- [ ] **Step 1: 事件清单**

```go
const (
	EventPostCreated     = "post.created"
	EventPostClosed      = "post.closed"
	EventContentCreated  = "content.created"
	EventContentLiked    = "content.liked"
	EventContentReplied  = "content.replied"
	EventUserRegistered  = "user.registered"
	EventUserFollowed    = "user.followed"
	EventUserMentioned   = "user.mentioned"
	EventReportCreated   = "report.created"
	EventModerationDone  = "moderation.decided"
)
```

- [ ] **Step 2: 文档说明如何新增官方模块**（实现 interface + container 注册），明确不支持第三方 .so 热加载。

- [ ] **Step 3: Commit**

```bash
git commit -m "docs(ext): stabilize event catalog and extension guide"
```

---

### Task 6（可选）: Chat MVP 或正式裁剪

**决策门：** 若产品需要站内实时群聊再做；否则在 index 标注 `won't do`。

若做：
- `ChatChannel` / `ChatMessage`
- SSE 或复用 websocket（新依赖需论证）
- 与 PM 分离（PM=私密，Chat=频道）

- [ ] **Step 1: 写 ADR 短文 `docs/adr/chat-mvp.md` 批准后再编码**
- [ ] **Step 2: 实现或关闭**

---

### Task 7: 阶段 4 收尾

- [ ] **Step 1: 回归关键路径** 注册→发帖→审核→通知→私信→watch→webhook
- [ ] **Step 2: 更新 index 全部阶段状态**
- [ ] **Step 3: 输出「与 Discourse 对齐度」复盘文档一页**

```bash
git commit -m "docs: complete discourse alignment phase4 platformization notes"
```

---

## Phase 4 Self-Review

| 需求 | Task |
|------|------|
| Groups | 1 |
| TL | 2 |
| 备份 | 3 |
| 导入 | 4 |
| 扩展点 | 5 |
| Chat 可选 | 6 |

**YAGNI：** 不做任意 Ruby 式插件沙箱；不做完整 SAML/LDAP 除非明确客户需求；不做主题市场。

**风险：** Groups 与单一 Role 双轨复杂度高——必须保留兼容层与迁移脚本，禁止一次性删除 `User.Role`。
