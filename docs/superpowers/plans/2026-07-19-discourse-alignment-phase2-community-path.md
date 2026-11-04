# Discourse 对齐阶段 2：社区主路径 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.
>
> **Index:** `docs/superpowers/plans/2026-07-19-discourse-alignment-index.md`  
> **Depends on:** 阶段 1 已合并（审核状态、通知基础、Post.Status 常量、邮箱验证字段可用）

**Goal:** 补齐 Discourse 社区主路径：私信、通知质量、话题未读与活跃排序、关闭态、邀请/审批注册、发帖限流、运营设置/审计/用户批量。

**Architecture:** 新增 `Conversation`/`Message` 模块按现有 checklist 全层落地；阅读状态用独立 `TopicReadState` 表；关闭态复用 `Post.Status=closed` 或 `Closed bool`（本计划采用 `Status=closed` + 保留 Locked 语义分离）；邀请用 `Invite` 表 + settings `register_mode`；审计补 List API；限流挂 router。

**Tech Stack:** 同阶段 1；EventBus 扩展事件类型；KV 未读计数模式复用 notification。

**DoD：**
1. 用户可发起 1:1 私信、列表、发送、已读、通知 type=`pm`
2. 单条通知 MarkRead 回写 KV 未读；点击可跳帖
3. 登录用户列表可见未读点；进入详情更新 last_read
4. 列表支持 `sort_by=active`（`last_active_at` bump）
5. 可关闭话题（禁回帖、可浏览）
6. `register_mode=invite|approve` 可用
7. 发帖/评论限流生效
8. Dashboard：设置项补齐、审计日志页、用户批量操作

---

## File Map

| 模块 | 路径 |
|------|------|
| 私信 model/repo/svc/handler | `app/server/internal/model/conversation.go`、`message.go`；`repository/conversation.go`；`service/pm*.go`；`handler/pm.go`；`router` |
| 阅读状态 | `model/topic_read.go`；`repository/topic_read.go`；`service` 挂 post list/detail |
| Post 活跃 | `model/post.go` 加 `LastActiveAt`；content create bump |
| 邀请 | `model/invite.go`；`service/invite.go`；auth Register 校验 |
| 通知修复 | `service/notification.go` MarkRead |
| 限流 | `router/router.go` |
| 审计 | `repository/audit_log.go` List；`handler/admin.go`；Dashboard page |
| 设置 UI | `settingsConfig.ts` |
| 用户批量 | `app/dashboard/src/pages/users` |
| api-client | `services/pm.ts`、`invite.ts`、types |
| Web | `pages/(app)/messages/**`、PostCard 未读、通知跳转 |

---

### Task 1: 通知 MarkRead 回写未读缓存 + 深链字段

**Files:**
- Modify: `app/server/internal/service/notification.go`
- Modify: `app/web/src/components/notification/NotificationBell.tsx`
- Modify: `app/web/src/pages/(app)/user/notifications.tsx`（若存在）

- [x] **Step 1: 修复 MarkRead**

当前 `MarkRead` 只改 DB。改为与 `Delete` 对称：

```go
func (s *notificationService) MarkRead(ctx context.Context, id, userID string) error {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "通知不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if n.UserID != userID {
		return pkg.NewAppError(pkg.NOT_FOUND, "通知不存在")
	}
	if n.IsRead {
		return nil
	}
	if err := s.repo.MarkRead(ctx, id); err != nil {
		return err
	}
	if _, err := s.kv.IncrBy(ctx, s.unreadKey(userID), -1); err != nil {
		logrus.Warnf("notification: kv decr on mark-read failed: %v", err)
	}
	return nil
}
```

- [x] **Step 2: 前端点击跳转**

通知项根据 `type` + `reference_id`：

```ts
// replied / liked → /timeline/post?id=${reference_id}
// followed → /user/${reference_id}
// pm → /messages/${reference_id}  // 会话 id，Task 2 后生效
```

先实现帖子类跳转；pm 占位路由可后补。

- [x] **Step 3: 测试**

```bash
cd app/server && go test ./internal/service/ -run Notification -count=1
```

- [x] **Step 4: Commit**

```bash
git commit -m "fix(notification): decrement unread cache on mark read and deep-link"
```

---

### Task 2: 私信 MVP 数据模型与 Repository

**Files:**
- Create: `app/server/internal/model/conversation.go`
- Create: `app/server/internal/repository/conversation.go`（接口+SQL+Mongo）
- Modify: `app/server/server/iris.go` AutoMigrate
- Modify: `app/server/internal/container/container.go`

- [x] **Step 1: 模型**

```go
// model/conversation.go
package model

import (
	"time"

	"feedchamber/internal/pkg"

	"gorm.io/gorm"
)

type Conversation struct {
	ID            string         `gorm:"primaryKey" json:"id" bson:"_id"`
	CreatedAt     time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at" bson:"updated_at"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty" bson:"last_message_at,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-" bson:"deleted_at,omitempty"`
}

func (Conversation) TableName() string { return "conversations" }

func NewConversation() *Conversation {
	return &Conversation{ID: pkg.NewID(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

type ConversationMember struct {
	ID             string     `gorm:"primaryKey" json:"id" bson:"_id"`
	ConversationID string     `gorm:"index;not null" json:"conversation_id" bson:"conversation_id"`
	UserID         string     `gorm:"index;not null" json:"user_id" bson:"user_id"`
	LastReadAt     *time.Time `json:"last_read_at,omitempty" bson:"last_read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at" bson:"created_at"`
}

func (ConversationMember) TableName() string { return "conversation_members" }

type Message struct {
	ID             string         `gorm:"primaryKey" json:"id" bson:"_id"`
	ConversationID string         `gorm:"index;not null" json:"conversation_id" bson:"conversation_id"`
	SenderID       string         `gorm:"index;not null" json:"sender_id" bson:"sender_id"`
	Body           string         `gorm:"type:text;not null" json:"body" bson:"body"`
	CreatedAt      time.Time      `json:"created_at" bson:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-" bson:"deleted_at,omitempty"`
}

func (Message) TableName() string { return "messages" }

func NewMessage(conversationID, senderID, body string) *Message {
	return &Message{
		ID:             pkg.NewID(),
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           body,
		CreatedAt:      time.Now(),
	}
}
```

- [x] **Step 2: Repository 接口**

```go
type ConversationRepository interface {
	CreateConversation(ctx context.Context, c *model.Conversation, memberUserIDs []string) error
	FindByID(ctx context.Context, id string) (*model.Conversation, error)
	FindDirect(ctx context.Context, userA, userB string) (*model.Conversation, error)
	ListForUser(ctx context.Context, userID string, offset, limit int) ([]model.ConversationListItem, int, error)
	IsMember(ctx context.Context, conversationID, userID string) (bool, error)
	CreateMessage(ctx context.Context, m *model.Message) error
	ListMessages(ctx context.Context, conversationID string, offset, limit int) ([]model.Message, int, error)
	MarkRead(ctx context.Context, conversationID, userID string, at time.Time) error
}
```

`ConversationListItem` 含 peer 用户摘要、last_message 预览、unread_count。

SQL 实现优先；Mongo 同步接口，可先返回错误但工厂完整。

- [x] **Step 3: AutoMigrate**

`server/iris.go` 加入 `Conversation{}`、`ConversationMember{}`、`Message{}`。

- [x] **Step 4: Commit**

```bash
git commit -m "feat(pm): add conversation/message models and repository"
```

---

### Task 3: 私信 Service / Handler / Router / 通知

**Files:**
- Create: `app/server/internal/service/pm_iface.go`、`pm.go`
- Create: `app/server/internal/handler/pm.go`
- Modify: `handler/handler.go`、`router/router.go`、`container.go`
- Modify: `notification` 支持 type `pm`
- Create: `packages/api-client/src/services/pm.ts`
- Create: `app/web/src/pages/(app)/messages/` 列表+会话页

- [x] **Step 1: Service API**

```go
type PMService interface {
	StartDirect(ctx context.Context, actorID, peerUserID, firstBody string) (*model.Conversation, *model.Message, error)
	ListConversations(ctx context.Context, userID string, page, pageSize int) ([]model.ConversationListItem, int, error)
	ListMessages(ctx context.Context, userID, conversationID string, page, pageSize int) ([]model.Message, int, error)
	Send(ctx context.Context, userID, conversationID, body string) (*model.Message, error)
	MarkRead(ctx context.Context, userID, conversationID string) error
}
```

规则：
- 禁止自己给自己发
- 非成员不可读发
- `StartDirect`：已有会话则复用
- `Send` 后给对方 `notification type=pm`，`reference_id=conversationID`

- [x] **Step 2: RPC 路由（protected）**

```
POST /api/v1/messages/conversations/list
POST /api/v1/messages/conversations/create   // body: { peer_user_id, body }
POST /api/v1/messages/conversations/{id}/messages/list
POST /api/v1/messages/conversations/{id}/messages/create
POST /api/v1/messages/conversations/{id}/read
```

- [x] **Step 3: api-client**

```ts
export const pmService = {
  listConversations(params: ListParams) {
    return postPaginated<ConversationListItem[]>('/messages/conversations/list', params)
  },
  create(body: { peer_user_id: string; body: string }) {
    return post<{ conversation: Conversation; message: Message }>('/messages/conversations/create', body)
  },
  listMessages(id: string, params: ListParams) {
    return postPaginated<Message[]>(`/messages/conversations/${id}/messages/list`, params)
  },
  send(id: string, body: { body: string }) {
    return post<Message>(`/messages/conversations/${id}/messages/create`, body)
  },
  markRead(id: string) {
    return post<null>(`/messages/conversations/${id}/read`, {})
  },
}
```

- [x] **Step 4: Web 最小页**

- `/messages` 会话列表
- `/messages/$id` 消息列表 + 输入框
- 用户主页增加「私信」按钮 → create

- [x] **Step 5: 测试**

Service 单测：禁止自聊、非成员 403、StartDirect 幂等。

```bash
cd app/server && go test ./internal/service/ -run PM -count=1
```

- [x] **Step 6: Commit**

```bash
git commit -m "feat(pm): direct messages MVP with notifications and web UI"
```

---

### Task 4: last_active_at + active 排序 + 关闭话题

**Files:**
- Modify: `app/server/internal/model/post.go`
- Modify: `app/server/internal/repository/post_sql.go`、`post_mongo.go`、`post_filter.go`
- Modify: `app/server/internal/service/post.go`、`content.go`
- Modify: `handler/post.go`、`router` pin/lock 旁加 close
- Modify: Web `PostListPage` 排序选项；详情关闭态 UI

- [x] **Step 1: 字段**

```go
LastActiveAt time.Time `gorm:"index" json:"last_active_at" bson:"last_active_at"`
```

`NewPost`：`LastActiveAt = now`  
评论创建成功：`postRepo.TouchActive(ctx, postID, time.Now())`  
列表 `sort_by=active` → `ORDER BY last_active_at DESC`

- [x] **Step 2: 关闭态**

```go
// service
func (s *postService) SetClosed(ctx context.Context, id string, closed bool, actorID, actorRole string) (*model.Post, error)
```

- closed=true → `Status=closed`（若原 pending 不允许随意改，仅 approved↔closed）
- `ContentService.Create`：`post.Status==closed` 与 `Locked` 一样禁评
- 路由：`POST /posts/{id}/close` body `{ "closed": true }` 权限 `post:pin` 或作者+mod

- [x] **Step 3: 前端排序**

`PostListPage` 增加「活跃」：`sort_by=active`

- [x] **Step 4: Commit**

```bash
git commit -m "feat(post): last_active_at sorting and topic close state"
```

---

### Task 5: 话题阅读状态 / 未读标记

**Files:**
- Create: `app/server/internal/model/topic_read.go`
- Create: `repository/topic_read.go`
- Modify: post list DTO 增加 `has_new` / `last_read_at`
- Modify: post detail handler 进入时 upsert read state
- Modify: Web `PostCard.tsx`

- [x] **Step 1: 模型**

```go
type TopicReadState struct {
	ID           string    `gorm:"primaryKey" json:"id" bson:"_id"`
	UserID       string    `gorm:"uniqueIndex:idx_user_post;not null" json:"user_id" bson:"user_id"`
	PostID       string    `gorm:"uniqueIndex:idx_user_post;not null" json:"post_id" bson:"post_id"`
	LastReadAt   time.Time `json:"last_read_at" bson:"last_read_at"`
	LastContentID string   `json:"last_content_id" bson:"last_content_id"`
}
```

- [x] **Step 2: API**

隐式：`GET/POST` 详情时（已登录）`UpsertRead(user, post, now, latestContentID)`  
列表：left join 计算 `has_new = post.last_active_at > read.last_read_at`（无记录且非作者可视为 new 或 false；阶段 2 采用：无记录 → has_new=false，避免全站红点爆炸；**仅跟踪过的帖**显示新回复。  
可选显式：`POST /posts/{id}/read` body `{}`。

- [x] **Step 3: PostCard UI**

```tsx
<Show when={props.post.has_new}>
  <span class="w-2 h-2 rounded-full bg-blue-500" title="有新回复" />
</Show>
```

- [x] **Step 4: Commit**

```bash
git commit -m "feat(post): topic read state and unread indicators"
```

---

### Task 6: 邀请制 / 审批注册

**Files:**
- Create: `model/invite.go`、`repository/invite.go`、`service/invite.go`、`handler/invite.go`
- Modify: `service/auth.go` Register
- Modify: `model/setting.go` 文档化 `register_mode`
- Modify: Dashboard 邀请管理页（最小 list/create）
- Modify: Web 注册表单 invite_code 字段

- [x] **Step 1: register_mode 语义**

| 值 | 行为 |
|----|------|
| `all` | 开放注册（默认） |
| `email` | 仅邮箱格式（现有） |
| `invite` | 必须有效 invite_code |
| `approve` | 注册成功但 `Role=pending` 或 `EmailVerified=false` 且 `Status=pending`；**本阶段用 meta/role：`role=pending_user` 不可发帖，admin 审批改 role=user** |

- [x] **Step 2: Invite 模型**

```go
type Invite struct {
	ID        string     `gorm:"primaryKey" json:"id"`
	Code      string     `gorm:"uniqueIndex;size:32" json:"code"`
	CreatorID string     `json:"creator_id"`
	MaxUses   int        `json:"max_uses"` // 0=无限
	UsedCount int        `json:"used_count"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
```

Register body 增加 `invite_code`；成功 `UsedCount++`。

- [x] **Step 3: 审批**

`POST /admin/users/{id}/approve_registration` → role user  
列表可筛 pending_user。

- [x] **Step 4: Commit**

```bash
git commit -m "feat(auth): invite and approve registration modes"
```

---

### Task 7: 发帖/评论限流

**Files:**
- Modify: `app/server/internal/router/router.go`

- [x] **Step 1: 挂载**

在 protected posts 路由：

```go
p.Post("/create", middleware.RateLimit(deps.Limiter, "post_create", 10, time.Minute), h.Post.Create)
// contents create
p.Post("/{postId}/contents/create", middleware.RateLimit(deps.Limiter, "comment_create", 30, time.Minute), h.Content.Create)
```

举报：

```go
r.Post("/create", middleware.RateLimit(deps.Limiter, "report_create", 10, time.Minute), ...)
```

数值可用 settings 后续化；阶段 2 硬编码合理默认。

- [x] **Step 2: 验证**

短时间超限返回业务限流错误码（与 login 一致）。

- [x] **Step 3: Commit**

```bash
git commit -m "feat(security): rate limit post, comment, and report creates"
```

---

### Task 8: 设置 UI 补齐 + 审计可查询 + 用户批量

**Files:**
- Modify: `app/dashboard/src/pages/settings/settingsConfig.ts`
- Modify: `repository/audit_log.go`、`service/audit_log.go`、`handler/admin.go`、`router`
- Create: `app/dashboard/src/pages/audit-logs/index.tsx`
- Modify: `app/dashboard/src/pages/users/*`
- Modify: 关键写操作调用 `AuditLogSvc.Record`

- [x] **Step 1: settingsConfig 增加分组**

至少暴露：
- `email_enabled`、SMTP/SendGrid/Resend 关键 key
- `mfa_totp_enabled`（只读开关，阶段 3 才实现逻辑）
- `api_key_enabled`
- `site_maintenance`（若阶段 1 已加则校验）
- `email_verification_required`
- `register_mode` select: all/email/invite/approve

- [x] **Step 2: Audit List**

```go
// repository
List(ctx context.Context, offset, limit int, actorID, action string) ([]model.AuditLog, int64, error)

// route
POST /api/v1/admin/audit_logs/list  body { page, pageSize, actor_id, action }
```

Record 点：settings update、batch users/posts、role sync、ban、moderation approve/reject。

- [x] **Step 3: 用户批量 UI**

对接已有 `adminService.batchUsers`：表格 `rowSelection` + 批量删除/改角色。

- [x] **Step 4: Commit**

```bash
git commit -m "feat(admin): settings coverage, audit log list, user batch UI"
```

---

### Task 9: 阶段 2 回归

- [x] **Step 1: 测试**

```bash
cd app/server && go test ./internal/service/... -count=1
pnpm --filter @feedchamber/api-client build
```

- [x] **Step 2: 手工 DoD 清单**

| 项 | OK |
|----|----|
| 私信收发 | |
| 通知已读数字正确 | |
| active 排序 | |
| 关闭话题禁评 | |
| 未读点 | |
| invite 注册 | |
| 限流 | |
| 审计页 | |
| 用户批量 | |

- [x] **Step 3: 更新 index 勾选阶段 2**

```bash
git commit -m "docs: mark discourse alignment phase2 complete"
```

---

## Phase 2 Self-Review

| 需求 | Task |
|------|------|
| 通知质量 | Task 1 |
| 私信 MVP | Task 2–3 |
| active + close | Task 4 |
| 阅读未读 | Task 5 |
| 邀请/审批 | Task 6 |
| 限流 | Task 7 |
| 设置/审计/批量 | Task 8 |
| 回归 | Task 9 |

**Out of scope:** @提及、watch、2FA 实现、Webhook、SEO slug、徽章（阶段 3）。
