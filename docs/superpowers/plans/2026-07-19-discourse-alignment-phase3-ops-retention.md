# Discourse 对齐阶段 3：运营与留存 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Index:** `docs/superpowers/plans/2026-07-19-discourse-alignment-index.md`  
> **Depends on:** 阶段 1–2 已合并（审核闭环、通知、私信、阅读状态、注册模式）

**Goal:** 对齐 Discourse 运营与留存主能力：@提及、Watch、屏蔽、2FA、敏感词反垃圾、SEO URL、邮件 Digest、积分榜、徽章 MVP、出站 Webhook。

**Architecture:** 内容管道增加 mention 解析与 EventBus 事件；Watch/Block 为关系表；2FA 绑定用户凭证表 + 登录二次挑战；敏感词真正执行 `ModerationRule`；SEO 用 slug 字段兼容旧 `?id=`；Digest 定时任务扫 watch/热帖；徽章规则引擎扫事件；Webhook 持久投递表 + HMAC。

**Tech Stack:** 同前；可选 `github.com/pquerna/otp` 做 TOTP；cron/goroutine ticker 做 digest/webhook retry。

**DoD：**
1. 正文 `@username` 产生通知
2. 可 watch 帖/版面/标签；被 watch 内容更新推送通知
3. 用户可屏蔽他人（列表/通知过滤）
4. TOTP 2FA 可 enroll/登录校验/备份码
5. 关键词规则能 block/flag 真实生效
6. 帖子 URL 支持 `/t/{slug}/{id}` + 旧链兼容
7. 周 Digest 邮件可开关发送
8. 公开积分榜 + 签到/发帖产分
9. 徽章自动授予 + 用户卡片展示
10. Webhook CRUD + 投递重试

---

## File Map

| 能力 | 路径 |
|------|------|
| 提及 | `service/mention.go`、content/post create 钩子、notification type=`mention` |
| Watch | `model/watch.go`、repo/svc/handler、Web 按钮 |
| Block | `model/user_block.go`、timeline/notification 过滤 |
| 2FA | `model/user_mfa.go`、`service/mfa.go`、auth login 流程 |
| 敏感词 | `service/moderation.go` CheckContent 实现 keyword/regex |
| SEO | `model/post.go` Slug；web routes；sitemap handler |
| Digest | `service/digest.go` + mailer 模板 |
| 积分榜 | post/content/like 产分；`handler/leaderboard.go` |
| 徽章 | `model/badge.go`、规则求值 |
| Webhook | `model/webhook.go`、`service/webhook.go`、EventBus 订阅 |

---

### Task 1: @提及解析与通知

**Files:**
- Create: `app/server/internal/service/mention.go`
- Modify: `service/content.go`、`service/post.go`
- Modify: `pkg/eventbus` 事件常量
- Modify: `service/notification.go` 订阅
- Modify: Web `MarkdownRenderer` 链到用户页（可选）

- [x] **Step 1: 解析器**

```go
package service

import (
	"regexp"
	"strings"
)

var mentionRe = regexp.MustCompile(`(?:^|[\s(])@([A-Za-z0-9_]{2,32})`)

func ExtractMentions(body string) []string {
	matches := mentionRe.FindAllStringSubmatch(body, -1)
	seen := map[string]struct{}{}
	var out []string
	for _, m := range matches {
		name := strings.ToLower(m[1])
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
```

- [x] **Step 2: 创建内容后**

对每个 username `FindByUsername` → 非作者 → EventBus `EventUserMentioned` → 通知  
`reference_id` = postID，content 含截断摘要。

- [x] **Step 3: 测试**

```go
func TestExtractMentions(t *testing.T) {
	got := ExtractMentions("hi @Alice and @bob and @Alice")
	// expect alice, bob unique case-normalized per implementation
}
```

- [x] **Step 4: Commit**

```bash
git commit -m "feat(social): parse @mentions and send notifications"
```

---

### Task 2: Watch（帖 / 版面 / 标签）

**Files:**
- Create: `model/watch.go`、`repository/watch.go`、`service/watch.go`、`handler/watch.go`
- Modify: content/post 创建时通知 watchers
- Modify: router + api-client + Web 按钮

- [x] **Step 1: 模型**

```go
type Watch struct {
	ID         string `gorm:"primaryKey" json:"id" bson:"_id"`
	UserID     string `gorm:"uniqueIndex:idx_watch;not null" json:"user_id"`
	TargetType string `gorm:"uniqueIndex:idx_watch;size:16;not null" json:"target_type"` // post|space|tag
	TargetID   string `gorm:"uniqueIndex:idx_watch;not null" json:"target_id"`
	Level      string `gorm:"size:16;default:watching" json:"level"` // watching|tracking|muted
	CreatedAt  time.Time
}
```

- [x] **Step 2: RPC**

```
POST /watches/set     { target_type, target_id, level }
POST /watches/list
POST /watches/delete  { target_type, target_id }
```

- [x] **Step 3: 扇出通知**

新回复：查 `target_type=post` watchers + space/tag watchers（去重、排除作者、排除 muted）。

- [x] **Step 4: UI**

帖详情「关注话题」；版面页「关注版面」。

- [x] **Step 5: Commit**

```bash
git commit -m "feat(social): watch posts, spaces, and tags"
```

---

### Task 3: 用户屏蔽

**Files:**
- Create: `model/user_block.go`、repo/svc/handler
- Modify: post list、notification list、pm start 校验
- Web 用户页「屏蔽」

- [x] **Step 1: 模型**

```go
type UserBlock struct {
	ID        string `gorm:"primaryKey"`
	BlockerID string `gorm:"uniqueIndex:idx_block;not null"`
	BlockedID string `gorm:"uniqueIndex:idx_block;not null"`
	CreatedAt time.Time
}
```

- [x] **Step 2: 过滤**

- 时间线：排除 blocked 作者（可选 setting）
- 通知：不创建给 blocker←blocked 或创建后 list 过滤
- 私信：`StartDirect` 若任一方拉黑 → 403

- [x] **Step 3: Commit**

```bash
git commit -m "feat(social): user block with feed and pm enforcement"
```

---

### Task 4: TOTP 2FA + 备份码

**Files:**
- Create: `model/user_mfa.go`、`service/mfa.go`、`handler/mfa.go`
- Modify: `service/auth.go` Login
- Modify: settings `mfa_totp_enabled`
- Web/Dashboard 安全设置页

- [x] **Step 1: 模型**

```go
type UserMFA struct {
	UserID      string `gorm:"primaryKey"`
	TOTPSecret  string `gorm:"size:64"` // encrypt at rest if密钥可用，阶段 3 可先存服务端加密
	Enabled     bool
	BackupCodes string `gorm:"type:text"` // bcrypt hashes JSON array
	UpdatedAt   time.Time
}
```

- [x] **Step 2: API**

```
POST /auth/mfa/totp/setup     → { secret, otpauth_url }
POST /auth/mfa/totp/enable    { code }
POST /auth/mfa/totp/disable   { code }
POST /auth/mfa/backup/regenerate
```

Login：若用户 MFA enabled → 返回 `{ mfa_required: true, mfa_token: "..." }`（短 TTL JWT/KV）  
`POST /auth/mfa/verify` `{ mfa_token, code }` → 正式 access/refresh。

- [x] **Step 3: 仅当 setting mfa_totp_enabled=true 时允许 enable**

- [x] **Step 4: 测试 otp 验证逻辑（固定 secret）**

- [x] **Step 5: Commit**

```bash
git commit -m "feat(auth): TOTP MFA enroll, login challenge, backup codes"
```

---

### Task 5: 敏感词 / 正则规则真正执行

**Files:**
- Modify: `app/server/internal/service/moderation.go` `CheckContent`
- Optional: Dashboard 规则管理页（若阶段 1 未做完整）

- [x] **Step 1: 加载 enabled rules by scene**

```go
rules, err := s.repo.ListEnabledRules(ctx, scene)
for _, rule := range rules {
  switch rule.Type {
  case "keyword":
    if strings.Contains(strings.ToLower(body), strings.ToLower(rule.Pattern)) {
      return s.flagOrAction(ctx, rule, ...)
    }
  case "regex":
    re, err := regexp.Compile(rule.Pattern)
    if err == nil && re.MatchString(body) {
      return s.flagOrAction(...)
    }
  }
}
```

- [x] **Step 2: action 映射**

- `block` → pending/rejected + 对用户返回错误「含违规内容」
- `flag` → pending 人工
- `replace` → 阶段 3 可做简单 `*` 替换后放行并记录
- `allow` → 跳过

- [x] **Step 3: 单测**

关键字命中 block；白名单角色放行。

- [x] **Step 4: Commit**

```bash
git commit -m "feat(moderation): execute keyword and regex rules in CheckContent"
```

---

### Task 6: SEO 友好 URL + sitemap

**Files:**
- Modify: `model/post.go` 加 `Slug string`
- Modify: post create 生成 slug（title 拼音/url-safe + 截断）
- Modify: `app/web` 路由增加 `/t/$slug/$id` 与兼容 `?id=`
- Create: `handler/seo.go` `POST` 或系统 `GET /sitemap.xml`（sitemap 可用 GET 例外）
- meta canonical 修复

- [x] **Step 1: slug 生成**

```go
func SlugifyTitle(title string) string {
	// lowercase, replace spaces with -, strip unsafe, max 80
}
```

同 slug 冲突时后缀 `-` + id 后 6 位。

- [x] **Step 2: Web 路由**

TanStack file route：`app/web/src/pages/(app)/t/$slug.$id.tsx` 或 `t/$slug/$id.tsx`  
loader 用 id 拉详情；旧 `timeline/post?id=` redirect 到新 URL。

- [x] **Step 3: sitemap**

`GET /sitemap.xml` 输出最近 N 帖绝对 URL（读 `site_url` setting）。

- [x] **Step 4: Commit**

```bash
git commit -m "feat(seo): post slug URLs, redirects, and sitemap"
```

---

### Task 7: 邮件 Digest

**Files:**
- Create: `service/digest.go`
- Modify: `mailer` 或 `email.go` 模板发送
- Modify: user preferences 表（最小：`user_settings` key `email_digest=weekly|off`）
- Modify: `server/iris.go` 启动 ticker（如每周一 09:00 本地）

- [x] **Step 1: 用户偏好最小表**

```go
type UserPreference struct {
	UserID string `gorm:"primaryKey"`
	Key    string `gorm:"primaryKey;size:64"`
	Value  string `gorm:"type:text"`
}
```

默认 `email_digest=weekly` 当全局 `email_notify_enabled` 与 `email_digest_enabled` true。

- [x] **Step 2: Digest 内容**

过去 7 天：用户 watch 的帖新回复摘要（最多 10）+ 全站 hot top 5。

- [x] **Step 3: 发送**

复用 `EmailService.SendNotification`；失败记日志不阻断。

- [x] **Step 4: Commit**

```bash
git commit -m "feat(email): weekly digest job and user preference"
```

---

### Task 8: 积分闭环 + 排行榜 + 签到 UI 接通

**Files:**
- Modify: `service/post.go`/`content.go`/`like.go`/`checkin.go` 产分
- Create: `handler/leaderboard.go` + router
- Modify: Web 用户页取消注释 `CheckInPanel`/`ActivityHeatmap`
- Create: Web `/leaderboard` 页

- [x] **Step 1: 产分规则（settings 可配，先常量）**

| 事件 | 分 |
|------|----|
| 发帖 | +5 |
| 评论 | +2 |
| 被赞 | +1 |
| 签到 | 已有 |

写 `user.TotalPoints` 原子增量（repo `IncrPoints`）。

- [x] **Step 2: API**

```
POST /leaderboard/list  { type: points|checkin|posts, page, pageSize }
```

- [x] **Step 3: UI 露出签到**

`app/web/src/pages/(app)/user/` 恢复组件引用。

- [x] **Step 4: Commit**

```bash
git commit -m "feat(growth): points economy, leaderboard, enable checkin UI"
```

---

### Task 9: 徽章 MVP

**Files:**
- Create: `model/badge.go`、`user_badge.go`、repo/svc
- Hook: 发帖/签到/点赞事件后 `BadgeService.Evaluate(userID, trigger)`
- Web 用户卡片展示

- [x] **Step 1: 种子徽章**

| code | 条件 |
|------|------|
| first_post | 发帖数 ≥1 |
| streak_7 | current_streak ≥7 |
| liked_10 | 累计获赞 ≥10 |

- [x] **Step 2: API**

```
POST /badges/list
POST /user/{id}/badges/list
```

- [x] **Step 3: Commit**

```bash
git commit -m "feat(growth): badge rules engine and profile display"
```

---

### Task 10: 出站 Webhook

**Files:**
- Create: `model/webhook.go`、`webhook_delivery.go`
- Create: `service/webhook.go`、`handler/webhook.go`
- Modify: EventBus 增加 post.created/user.created/report.created 等并订阅
- Dashboard Webhooks 页
- api-client

- [x] **Step 1: 模型**

```go
type Webhook struct {
	ID        string `gorm:"primaryKey"`
	URL       string `gorm:"size:512;not null"`
	Secret    string `gorm:"size:128"`
	Events    string `gorm:"type:text"` // JSON string array
	Enabled   bool
	CreatedAt time.Time
}

type WebhookDelivery struct {
	ID        string
	WebhookID string
	Event     string
	Payload   string
	Status    string // pending|success|failed
	Attempts  int
	LastError string
	CreatedAt time.Time
}
```

- [x] **Step 2: 投递**

HTTP POST JSON；header `X-Orin-Signature: sha256=HMAC`  
失败指数退避最多 5 次；后台 worker goroutine。

- [x] **Step 3: 管理 API**

```
POST /admin/webhooks/create|list|{id}/update|{id}/delete|{id}/deliveries/list
```

- [x] **Step 4: Commit**

```bash
git commit -m "feat(integration): outbound webhooks with signed delivery retries"
```

---

### Task 11: 阶段 3 回归

- [x] **Step 1: 自动化**

```bash
cd app/server && go test ./internal/service/... -count=1
pnpm --filter @orinforum/api-client build
```

- [x] **Step 2: 手工 DoD 全表勾选**

- [x] **Step 3: 文档**

更新 index 阶段 3 完成；必要时补 `docs/superpowers/specs` 简述。

```bash
git commit -m "docs: mark discourse alignment phase3 complete"
```

---

## Phase 3 Self-Review

| 需求 | Task |
|------|------|
| @提及 | 1 |
| Watch | 2 |
| 屏蔽 | 3 |
| 2FA | 4 |
| 敏感词 | 5 |
| SEO URL | 6 |
| Digest | 7 |
| 积分榜 | 8 |
| 徽章 | 9 |
| Webhook | 10 |

**Out of scope:** Groups、完整 TL、插件运行时、Chat、Discourse 导入器（阶段 4）。
