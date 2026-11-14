# Discourse 对齐阶段 1：主路径闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Index:** `docs/superpowers/plans/2026-07-19-discourse-alignment-index.md`

**Goal:** 打通 OrinForum 已有半成品，使论坛主路径“正确可用”：详情渲染、互动按钮、审核闭环、可用搜索、本地草稿、邮箱验证、维护模式。

**Architecture:** 不引入新架构。后端继续 Handler→Service→Repository；审核通过 `ModerationService.CheckContent` 注入 `PostService`/`ContentService`；内容状态用 `Post.Status`（`approved|pending|rejected|closed` 中本阶段只用到 pending/approved/rejected）；前端 Web 复用 `MarkdownRenderer`、`likeService`/`favoriteService`、`postService.search`。

**Tech Stack:** Go/Iris、SolidJS、React Dashboard、api-client、KV/settings、现有 moderation/report/email 模块。

**DoD（阶段出口）：**
1. 帖子详情正文与评论渲染 Markdown（非原文直出）
2. 帖子可收藏、内容可点赞，状态可回显
3. 开启审核后发帖/评论进入 pending，Approve 可见、Reject 不可见
4. Dashboard 有审核队列页
5. 举报处理可联动删内容/锁帖/禁言（至少一种动作 API）
6. Nav 搜索调用 `POST /posts/search` 并展示结果
7. 发帖/回复本地草稿自动恢复
8. 注册后可发邮箱验证码并完成验证；未验证可限制发帖（setting 控制）
9. `site_maintenance=true` 时写操作 503/只读

---

## File Map

| 区域 | 创建/修改 |
|------|----------|
| Web 渲染 | `app/web/src/pages/(app)/timeline/-modules/ContentItem.tsx`、`PostDetailView.tsx` |
| Web 互动 | 同上 + `app/web/src/hooks/like.ts`、`favorite.ts` |
| Web 搜索 | `app/web/src/components/lc/Search.tsx`、`app/web/src/api/services/index.ts` |
| Web 草稿 | `app/web/src/pages/(app)/timeline/create.tsx`、`app/web/src/components/content/Reply.tsx`、新建 `app/web/src/utils/draft.ts` |
| 审核后端 | `service/post.go`、`content.go`、`moderation.go`、`container.go`、`model/post.go`（状态常量） |
| 审核前端 | `packages/api-client/src/services/moderation.ts`、`types`；`app/dashboard/src/pages/moderation/` |
| 举报 | `service/report.go`、`handler/report.go`、Dashboard reports |
| 邮箱验证 | `model/user.go`、`service/auth.go`、`handler/auth.go`、`router/router.go`、api-client auth |
| 维护模式 | `middleware/middleware.go` 或新建 `middleware/maintenance.go`、`router` 挂载、settings seed + Dashboard settingsConfig |
| 测试 | `service/moderation` 相关 test、`post` create approval test |

---

### Task 1: 详情/评论 Markdown 渲染

**Files:**
- Modify: `app/web/src/pages/(app)/timeline/-modules/ContentItem.tsx`
- Modify: `app/web/src/components/post/PostDetailView.tsx`（若首帖不走 ContentItem 再补）
- Reuse: `app/web/src/components/content/MarkdownRenderer.tsx`

- [ ] **Step 1: 确认 ContentItem 当前直出 body**

打开 `ContentItem.tsx`，确认存在：

```tsx
<div class="leading-relaxed">{props.content.body}</div>
```

- [ ] **Step 2: 改为 Markdown 渲染**

```tsx
import MarkdownRenderer from "~/components/content/MarkdownRenderer";

// 在 body 位置替换为：
<div class="leading-relaxed prose prose-sm dark:prose-invert max-w-none">
  <MarkdownRenderer content={props.content.body} />
</div>
```

若 `MarkdownRenderer` prop 名不是 `content`，以组件定义为准（当前文件为 `MarkdownRendererProps`，读取其字段名后对齐，常见为 `markdown` 或 `content`）。

- [ ] **Step 3: 手工验证**

Run:

```bash
pnpm --filter web dev
```

打开含 Markdown 的帖子（`**粗体**`、代码块、图片），确认渲染而非原文。

- [ ] **Step 4: Commit**

```bash
git add "app/web/src/pages/(app)/timeline/-modules/ContentItem.tsx"
git commit -m "fix(web): render post/comment body as markdown"
```

---

### Task 2: 点赞 / 收藏 UI 闭环

**Files:**
- Modify: `app/web/src/pages/(app)/timeline/-modules/ContentItem.tsx`
- Modify: `app/web/src/components/post/PostDetailView.tsx` 或 `PostHeader`（收藏挂在帖级）
- Reuse: `app/web/src/hooks/like.ts`、`app/web/src/hooks/favorite.ts`
- Reuse API: `packages/api-client` like/favorite 或 web `~/api/services`

- [ ] **Step 1: 扩展 ContentItem 支持点赞**

在 `ContentItem.tsx` 增加本地状态与按钮（SolidJS）：

```tsx
import { createSignal, onMount, Show } from "solid-js";
import { toggleLike, getLikeStatus } from "~/hooks/like";

// props 内已有 content.like_count
const [liked, setLiked] = createSignal(false);
const [likeCount, setLikeCount] = createSignal(props.content.like_count ?? 0);
const [likeLoading, setLikeLoading] = createSignal(false);

onMount(async () => {
  try {
    const res = await getLikeStatus(props.content.id);
    // 兼容 { data: { liked: boolean } } 或直接 boolean
    const data = (res as any)?.data ?? res;
    setLiked(Boolean(data?.liked ?? data));
  } catch {
    // 未登录忽略
  }
});

async function onToggleLike() {
  if (likeLoading()) return;
  setLikeLoading(true);
  try {
    const res = await toggleLike(props.content.id);
    const data = (res as any)?.data ?? res;
    const next = Boolean(data?.liked ?? !liked());
    setLiked(next);
    setLikeCount((c) => c + (next ? 1 : -1));
  } finally {
    setLikeLoading(false);
  }
}
```

UI 放在作者行下方或 body 下方：

```tsx
<button
  type="button"
  class="text-xs opacity-70 hover:opacity-100"
  disabled={likeLoading()}
  onClick={onToggleLike}
>
  {liked() ? "已赞" : "点赞"} · {likeCount()}
</button>
```

- [ ] **Step 2: 帖级收藏按钮**

在 `PostDetailView.tsx`（或 `PostHeader`）增加：

```tsx
import { createSignal, onMount } from "solid-js";
import { toggleFavorite, getFavoriteStatus } from "~/hooks/favorite";

const [favorited, setFavorited] = createSignal(false);
const [favLoading, setFavLoading] = createSignal(false);

onMount(async () => {
  try {
    const res = await getFavoriteStatus(props.post.id);
    const data = (res as any)?.data ?? res;
    setFavorited(Boolean(data?.favorited ?? data));
  } catch {}
});

async function onToggleFavorite() {
  if (favLoading()) return;
  setFavLoading(true);
  try {
    const res = await toggleFavorite(props.post.id);
    const data = (res as any)?.data ?? res;
    setFavorited(Boolean(data?.favorited ?? !favorited()));
  } finally {
    setFavLoading(false);
  }
}
```

- [ ] **Step 3: 手工验证**

登录后：点赞数字变化、刷新保持；收藏后 `/user/favorites` 可见。

- [ ] **Step 4: Commit**

```bash
git add "app/web/src/pages/(app)/timeline/-modules/ContentItem.tsx" \
  app/web/src/components/post/PostDetailView.tsx \
  app/web/src/pages/\(app\)/timeline/-modules/PostHeader.tsx 2>/dev/null || true
git commit -m "feat(web): wire like and favorite actions on post detail"
```

---

### Task 3: 审核主链路 — Service 注入与状态回写

**Files:**
- Modify: `app/server/internal/service/post.go`
- Modify: `app/server/internal/service/content.go`
- Modify: `app/server/internal/service/moderation.go`
- Modify: `app/server/internal/service/post_iface.go`（若构造签名变）
- Modify: `app/server/internal/service/content_iface.go` / 构造函数
- Modify: `app/server/internal/container/container.go`
- Modify: `app/server/internal/model/post.go`（状态常量）
- Test: `app/server/internal/service/post_moderation_test.go`（新建）

- [ ] **Step 1: 状态常量**

在 `model/post.go` 增加：

```go
const (
	PostStatusApproved = "approved"
	PostStatusPending  = "pending"
	PostStatusRejected = "rejected"
	PostStatusClosed   = "closed" // 阶段 2 使用，阶段 1 先定义
)
```

- [ ] **Step 2: PostService 注入 ModerationService + 作者角色查询**

`NewPostService` 增加参数 `moderationSvc ModerationService` 与现有 `userRepo`（若无则加 `UserRepository` 只为取 role）。

`Create` 在 `CreateWithContent` **之后**调用审核（需要 targetID）：

```go
// after CreateWithContent success
if s.moderationSvc != nil {
	role := "user"
	if s.users != nil {
		if u, err := s.users.FindByID(ctx, authorID); err == nil && u != nil {
			role = u.Role
		}
	}
	action, err := s.moderationSvc.CheckContent(ctx, "post", post.ID, body, model.ModerationScenePost, role)
	if err == nil {
		switch action {
		case model.ModerationActionBlock, model.ModerationActionFlag:
			post.Status = model.PostStatusPending
			_ = s.posts.Update(ctx, post)
		case model.ModerationActionAllow:
			// keep approved unless post_approval already set pending
		}
	}
}
```

保留现有 `post_approval==true` → pending 逻辑；审核 flag/block 也应 pending。

列表默认只返回 `approved`（若 `status` 参数为空时强制 `approved`）：

```go
if status == "" {
	status = model.PostStatusApproved
}
```

- [ ] **Step 3: ContentService 同样接入**

`Create` 成功后：

```go
if s.moderationSvc != nil {
	role := "user"
	// resolve author role
	action, _ := s.moderationSvc.CheckContent(ctx, "content", c.ID, body, model.ModerationSceneComment, role)
	if action == model.ModerationActionBlock || action == model.ModerationActionFlag {
		// 评论无独立 status 字段时：阶段 1 方案 A — 软删或增加 Status 字段
		// 推荐：给 Content 增加 Status string `default:approved`
	}
}
```

**阶段 1 明确采用：给 `Content` 加 `Status` 字段**（与 Post 一致），默认 `approved`；列表查询过滤非 approved（作者本人与 admin 可见 pending）。

`model/content.go`：

```go
Status string `gorm:"size:16;default:approved;index" json:"status" bson:"status"`
```

`NewContent` 设 `Status: PostStatusApproved` 或 content 常量。

- [ ] **Step 4: Approve/Reject 回写目标状态**

`moderationService` 需要 `postRepo`/`contentRepo`（或专用接口）：

```go
func (s *moderationService) Approve(ctx context.Context, id, reviewerID string) error {
	m, err := s.repo.GetModeration(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "moderation record not found")
	}
	if err := s.repo.UpdateModerationStatus(ctx, id, model.ModerationApproved, reviewerID); err != nil {
		return err
	}
	return s.applyTargetStatus(ctx, m.TargetType, m.TargetID, model.PostStatusApproved)
}

func (s *moderationService) Reject(ctx context.Context, id, reviewerID, reason string) error {
	m, err := s.repo.GetModeration(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "moderation record not found")
	}
	if err := s.repo.UpdateModerationStatus(ctx, id, model.ModerationRejected, reviewerID); err != nil {
		return err
	}
	return s.applyTargetStatus(ctx, m.TargetType, m.TargetID, model.PostStatusRejected)
}
```

- [ ] **Step 5: container 装配**

`container.go` 中 `ModerationSvc` 构造后注入 `PostSvc`/`ContentSvc`（注意依赖顺序：ModerationRepo → ModerationSvc → Post/Content）。

若循环依赖，采用：

```go
c.ModerationSvc = service.NewModerationService(c.ModerationRepo, c.PostRepo, c.ContentRepo)
c.PostSvc = service.NewPostService(..., c.ModerationSvc, c.UserRepo)
c.ContentSvc = service.NewContentService(..., c.ModerationSvc)
```

- [ ] **Step 6: 写失败测试再实现（TDD）**

`post_moderation_test.go` 用 mock：当 `CheckContent` 返回 `flag`，Create 后 status 为 pending。

Run:

```bash
cd app/server && go test ./internal/service/ -run Moderation -count=1
```

Expected: 先 FAIL 后 PASS。

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/model/post.go app/server/internal/model/content.go \
  app/server/internal/service/post.go app/server/internal/service/content.go \
  app/server/internal/service/moderation.go app/server/internal/container/container.go \
  app/server/internal/service/*_test.go
git commit -m "feat(moderation): wire CheckContent into post/content and sync status"
```

---

### Task 4: Dashboard 审核队列 + api-client 对齐

**Files:**
- Modify: `packages/api-client/src/services/moderation.ts`
- Modify: `packages/api-client/src/types.ts`（如需）
- Create: `app/dashboard/src/pages/moderation/index.tsx`
- Modify: Dashboard 路由/菜单（通常 `app/dashboard/src/app.tsx` 或 `config/routes.ts`，以仓库实际路由文件为准）

- [ ] **Step 1: 修正 ModerationItem 类型**

对齐后端 `model.ContentModeration`：

```ts
export interface ModerationItem {
  id: string
  target_type: string
  target_id: string
  scene?: string
  status: string
  action?: string
  strategy?: string
  original_body?: string
  reviewer_id?: string
  reason?: string
  created_at: string
  updated_at: string
}
```

`listPending` 响应使用项目统一分页：`{ code, data, total }` 或 `postPaginated`。

```ts
import { postPaginated, post } from '../client'

export const moderationService = {
  listPending(params?: ListParams) {
    return postPaginated<ModerationItem[]>('/admin/moderation/pending', params ?? {})
  },
  approve(id: string) {
    return post<null>(`/admin/moderation/${id}/approve`, {})
  },
  reject(id: string, reason: string) {
    return post<null>(`/admin/moderation/${id}/reject`, { reason })
  },
  listRules(params?: ListParams) {
    return postPaginated<ModerationRule[]>('/admin/moderation/rules/list', params ?? {})
  },
  getConfigs() {
    return post<Record<string, string>>('/admin/moderation/configs', {})
  },
}
```

路径以 `router.go` 中 `/admin/moderation` 实际注册为准，**先读 router 再定 URL**。

- [ ] **Step 2: Dashboard 页面**

`app/dashboard/src/pages/moderation/index.tsx`：ProTable 列：id、target_type、target_id、status、original_body 摘要、created_at；操作：通过 / 拒绝。

拒绝弹窗输入 reason，调 `reject`。

- [ ] **Step 3: 菜单**

在运营菜单加入「内容审核」→ `/operations/moderation`（路径风格对齐现有 `reports`）。

- [ ] **Step 4: 验证**

1. settings/`post_approval` 或 moderation enabled=true
2. 发帖 → 列表不可见（非作者）
3. Dashboard 审核通过 → 列表可见

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/services/moderation.ts \
  app/dashboard/src/pages/moderation \
  app/dashboard/src/**/routes* app/dashboard/src/**/menu* 2>/dev/null || true
git commit -m "feat(dashboard): add moderation queue page and fix api types"
```

---

### Task 5: 举报处理闭环（动作）

**Files:**
- Modify: `app/server/internal/service/report.go`
- Modify: `app/server/internal/service/report_iface.go`
- Modify: `app/server/internal/handler/report.go`
- Modify: `app/dashboard/src/pages/reports/`（处理弹窗）
- Optional: inject PostSvc/BannedSvc into ReportService

- [ ] **Step 1: 扩展 UpdateStatus 请求**

```go
type UpdateReportRequest struct {
	Status string `json:"status"` // resolved | dismissed
	Action string `json:"action"` // none | delete_target | lock_post | ban_user
	Note   string `json:"note"`
}
```

- [ ] **Step 2: Service 实现**

```go
func (s *reportService) UpdateStatus(ctx context.Context, id, status, action, actorID, note string) error {
	if status != "resolved" && status != "dismissed" && status != "pending" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "status必须为pending、resolved或dismissed")
	}
	r, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "举报不存在")
	}
	if status == "resolved" {
		switch action {
		case "", "none":
			// no-op
		case "delete_target":
			// call post/content delete by r.TargetType/TargetID
		case "lock_post":
			// only if target is post
		case "ban_user":
			// ban target author if resolvable
		default:
			return pkg.NewAppError(pkg.INVALID_PARAMS, "unknown action")
		}
	}
	return s.repo.UpdateStatus(ctx, id, status)
}
```

阶段 1 至少实现 `none` + `delete_target`；lock/ban 可在同 PR 完成若依赖简单。

- [ ] **Step 3: Dashboard 处理表单增加 action 下拉**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(report): support resolve actions delete/lock/ban"
```

---

### Task 6: Web 搜索接真实 API

**Files:**
- Modify: `app/web/src/components/lc/Search.tsx`
- Modify: `app/web/src/api/services/index.ts`（确保 `postService.search` 或等价）
- Optional backend later FTS；阶段 1 先用现有 LIKE 搜索

- [ ] **Step 1: 确认客户端有 search**

`packages/api-client/src/services/post.ts` 已有：

```ts
search(params: ListParams & { q: string }) {
  return postPaginated<PostListItem[]>('/posts/search', params)
}
```

Web 侧若未 re-export，在 `app/web/src/api/services/index.ts` 增加：

```ts
search: (params: { q: string; page?: number; page_size?: number }) =>
  postPaginated<PostListItem[]>('/posts/search', params),
```

- [ ] **Step 2: 替换 Search.tsx TODO**

```tsx
import { postService } from '~/api/services'; // 以实际导出为准

const performSearch = async (q: string) => {
  if (!q.trim()) {
    setResults([]);
    return;
  }
  setLoading(true);
  try {
    const res = await postService.search({ q: q.trim(), page: 1, page_size: 10 });
    const items = (res as any)?.data ?? [];
    setResults(
      (items as Array<{ id: string; title: string; content_digest?: string }>).map((p) => ({
        title: p.title,
        url: `/timeline/post?id=${p.id}`,
        content: p.content_digest ?? '',
      })),
    );
  } catch {
    setResults([]);
  } finally {
    setLoading(false);
  }
};
```

点击结果 `navigate({ to: item.url })` 或 `window.location`。

- [ ] **Step 3: 验证**

创建帖含独特关键词 → 打开 Nav 搜索 → 出现结果并可跳转。

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(web): connect nav search to posts/search API"
```

---

### Task 7: 发帖/回复本地草稿

**Files:**
- Create: `app/web/src/utils/draft.ts`
- Modify: `app/web/src/pages/(app)/timeline/create.tsx`
- Modify: `app/web/src/components/content/Reply.tsx`

- [ ] **Step 1: draft 工具**

```ts
const PREFIX = 'orinforum:draft:';

export function saveDraft(key: string, value: unknown) {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(PREFIX + key, JSON.stringify({ v: value, t: Date.now() }));
}

export function loadDraft<T>(key: string): T | null {
  if (typeof localStorage === 'undefined') return null;
  const raw = localStorage.getItem(PREFIX + key);
  if (!raw) return null;
  try {
    return JSON.parse(raw).v as T;
  } catch {
    return null;
  }
}

export function clearDraft(key: string) {
  if (typeof localStorage === 'undefined') return;
  localStorage.removeItem(PREFIX + key);
}
```

- [ ] **Step 2: 发帖页**

key = `post:create:${spaceId || 'global'}`  
title/content 变更 debounce 800ms 调用 `saveDraft`  
onMount `loadDraft` 恢复  
提交成功 `clearDraft`

- [ ] **Step 3: 回复框**

key = `reply:${postId}:${parentId || 'root'}`  
同样 debounce 保存；提交成功清除。

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(web): auto-save local drafts for create and reply"
```

---

### Task 8: 邮箱验证闭环

**Files:**
- Modify: `app/server/internal/model/user.go`
- Modify: `app/server/internal/service/auth.go`、`auth_iface.go`
- Modify: `app/server/internal/handler/auth.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/internal/service/post.go`（发帖门禁）
- Modify: `packages/api-client/src/services/auth.ts`
- Modify: Web 用户设置/提示页（最小：登录后 banner 或 profile）
- Seed: `model/setting.go` 增加 `email_verification_required`

- [ ] **Step 1: User 字段**

```go
EmailVerified bool `gorm:"default:false" json:"email_verified" bson:"email_verified"`
```

`UserResponse` 同步暴露 `email_verified`。

- [ ] **Step 2: API**

```
POST /api/v1/auth/send_verify_email   (JWT) body {}
POST /api/v1/auth/verify_email        (JWT) body { "code": "123456" }
```

实现：

```go
func (s *authService) SendVerifyEmail(ctx context.Context, userID string) error {
	u, err := s.users.FindByID(ctx, userID)
	// ...
	if u.EmailVerified {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "邮箱已验证")
	}
	return s.emailSvc.SendCode(ctx, PurposeVerifyEmail, u.Email)
}

func (s *authService) VerifyEmail(ctx context.Context, userID, code string) error {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil { return err }
	if err := s.emailSvc.VerifyCode(ctx, PurposeVerifyEmail, u.Email, code); err != nil {
		return err
	}
	u.EmailVerified = true
	return s.users.Update(ctx, u)
}
```

- [ ] **Step 3: 发帖限制**

setting `email_verification_required` 默认 `false`（兼容现网）；为 `true` 时 `PostService.Create` / `ContentService.Create` 检查 `EmailVerified`。

- [ ] **Step 4: 前端最小 UI**

Profile 显示未验证 + 发送验证码 + 输入验证。

- [ ] **Step 5: 测试**

```bash
cd app/server && go test ./internal/service/ -run EmailVerify -count=1
```

- [ ] **Step 6: Commit**

```bash
git commit -m "feat(auth): email verification flow and optional post gate"
```

---

### Task 9: 维护模式

**Files:**
- Create: `app/server/internal/middleware/maintenance.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/internal/model/setting.go` DefaultSettings
- Modify: `app/dashboard/src/pages/settings/settingsConfig.ts`

- [ ] **Step 1: setting**

```go
"site_maintenance": "false",
"site_maintenance_message": "站点维护中，请稍后再试",
```

- [ ] **Step 2: 中间件**

```go
func MaintenanceGate(settingSvc service.SettingService) iris.Handler {
	return func(ctx iris.Context) {
		on := pkg.GetSettingOrDefault(ctx.Request().Context(), settingSvc, "site_maintenance", "SITE_MAINTENANCE", "false")
		if on != "true" {
			ctx.Next()
			return
		}
		// 允许：health、login、admin 读、settings/public、已登录 admin 写
		path := ctx.Path()
		if strings.HasSuffix(path, "/health") || strings.Contains(path, "/auth/login") {
			ctx.Next()
			return
		}
		role, _ := ctx.Values().GetString("user_role")
		if role == "admin" {
			ctx.Next()
			return
		}
		// 写方法或业务 POST：统一拦截非 admin
		msg := pkg.GetSettingOrDefault(ctx.Request().Context(), settingSvc, "site_maintenance_message", "", "站点维护中")
		ctx.StatusCode(503)
		_ = ctx.JSON(iris.Map{"code": 5030, "msg": msg, "data": nil})
	}
}
```

挂载位置：全局中间件链，JWT 之后以便识别 admin（或 OptionalJWT + 再判断）。

- [ ] **Step 3: Dashboard 开关**

`settingsConfig.ts` 站点组增加 boolean `site_maintenance` 与 message 文本。

- [ ] **Step 4: 验证**

开维护 → 普通用户发帖 503；admin 可操作；关维护恢复。

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(ops): site maintenance mode gate and dashboard toggle"
```

---

### Task 10: 阶段 1 回归与收尾

- [ ] **Step 1: 后端测试**

```bash
cd app/server && go test ./internal/service/... -count=1
```

Expected: PASS（允许跳过与本阶段无关的既有失败，但本阶段新增测试必须过）。

- [ ] **Step 2: 手工清单**

| 检查项 | 结果 |
|--------|------|
| Markdown 渲染 | |
| 点赞/收藏 | |
| 审核 pending→approve | |
| 搜索跳转 | |
| 草稿恢复 | |
| 邮箱验证 | |
| 维护模式 | |

- [ ] **Step 3: 更新索引勾选**

将 `2026-07-19-discourse-alignment-index.md` 阶段 1 勾为完成。

- [ ] **Step 4: 合并前 commit**

```bash
git commit -m "docs: mark discourse alignment phase1 complete"
```

---

## Phase 1 Self-Review

| 需求 | Task |
|------|------|
| Markdown 渲染 | Task 1 |
| 点赞收藏 UI | Task 2 |
| 审核主链路 | Task 3 |
| 审核 Dashboard | Task 4 |
| 举报动作 | Task 5 |
| 搜索 | Task 6 |
| 草稿 | Task 7 |
| 邮箱验证 | Task 8 |
| 维护模式 | Task 9 |
| 回归 | Task 10 |

**Placeholder scan:** 无 TBD；router 真实 path 要求实现时先读 `router.go`。  
**Out of scope:** 私信、2FA、Webhook、FTS 引擎、SEO slug（阶段 2/3）。
