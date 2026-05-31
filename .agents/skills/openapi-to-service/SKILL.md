---
name: openapi-to-service
description: Use when generating TypeScript API service files for packages/web from the OrinForum OpenAPI spec (openapi.json at project root, or fetched from http://localhost:8080/swagger/doc.json). Outputs per-tag service files to packages/web/src/api/service/.
---

# OpenAPI to Service — OrinForum

## Overview

将 OrinForum 的 Swagger 2.0 规范（项目根目录 `openapi.json`，basePath `/api/v1`）转换为按 tag 分组的 TypeScript service 文件，输出到 `packages/web/src/api/service/`。

分割脚本 `index.js`（与本 skill 同目录）读取 `doc/orinforum/api.v1.json`，按 tag 拆分为独立 JSON，再逐个生成 service 文件。

## 项目约定

### client 层（`packages/web/src/api/client.ts`）

```ts
import { get, post, postForm } from '../client'
```

- `get<T>(path)` — GET 请求
- `post<T>(path, body?)` — POST 请求（body 可选）
- `postForm<T>(path, form)` — multipart 文件上传

**关键规则**：后端路由全部为 RPC 风格，列表/操作接口均用 `POST` + body 传参，不用 GET query string。只有少数真正的 GET 接口（如 `getById`、`getProfile`）才用 `get()`。

### 公共类型（`packages/web/src/api/types.ts`）

以下类型已在 `types.ts` 中定义，**不要在 service 文件中重复定义**，直接 import：

```ts
import type { ListParams, User, Post, PostListItem, Content, ContentListItem,
  Category, Tag, Notification, Upload, Report, Role, AdminStats, AuthResponse } from '../types'
```

`ListParams` 定义：
```ts
interface ListParams {
  page?: number
  pageSize?: number
  startTime?: string
  endTime?: string
}
```

### service 文件结构

每个 service 文件的固定结构：

```ts
import { get, post } from '../client'           // 只导入实际用到的方法
import type { Xxx, ListParams } from '../types'  // 只导入实际用到的类型

export const xxxService = {
  methodName(params) {
    return post<ReturnType>('/path/to/endpoint', params)
  },
  // ...
}
```

**命名规则**：
- 导出名：`{tag}Service`（camelCase），如 `authService`、`postService`、`followService`
- 方法名：简短动词，如 `list`、`create`、`update`、`delete`、`getById`、`toggle`、`status`、`counts`
- 不加冗余前缀：`postService.list()` 而非 `postService.listPosts()`

### 实际路由约定（以现有 service 为准）

后端实际路由与 openapi.json 有差异，以 `packages/server/internal/router/router.go` 为准：

| 操作 | 实际路径 | 方法 |
|------|---------|------|
| 列表 | `/posts/list` | POST + body |
| 搜索 | `/posts/search` | POST + body |
| 获取单个 | `/posts/{id}` | GET |
| 创建 | `/posts/create` | POST + body |
| 更新 | `/posts/{id}/update` | POST + body |
| 删除 | `/posts/{id}/delete` | POST（无 body） |
| 操作（pin/lock等） | `/posts/{id}/pin` | POST（无 body） |
| 评论列表 | `/posts/{postId}/contents/list` | POST + body |
| 评论创建 | `/posts/{postId}/contents/create` | POST + body |
| 评论更新 | `/posts/{postId}/contents/{id}/update` | POST + body |
| 评论删除 | `/posts/{postId}/contents/{id}/delete` | POST（无 body） |

## 现有 service 示例

### auth.ts
```ts
import { post } from '../client'
import type { AuthResponse } from '../types'

export const authService = {
  register(body: { username: string; email: string; password: string }) {
    return post<AuthResponse>('/auth/register', body)
  },
  login(body: { username?: string; email?: string; password: string }) {
    return post<AuthResponse>('/auth/login', body)
  },
  logout() { return post<null>('/auth/logout') },
  refresh() { return post<{ access_token: string }>('/auth/refresh') },
  refreshToken(refresh_token: string) {
    return post<{ access_token: string }>('/auth/refresh_token', { refresh_token })
  },
  changePassword(body: { old_password: string; new_password: string }) {
    return post<null>('/auth/change_password', body)
  },
  firstChangePassword(body: { new_password: string; name?: string }) {
    return post<null>('/auth/first_change_password', body)
  },
  resetPassword(body: { username: string; new_password: string }) {
    return post<null>('/auth/reset_password', body)
  },
}
```

### post.ts
```ts
import { get, post } from '../client'
import type { Post, PostListItem, Tag, ListParams } from '../types'

export interface PostListParams extends ListParams {
  categoryId?: string
  authorId?: string
  status?: string
}

export const postService = {
  list(params: PostListParams) { return post<PostListItem[]>('/posts/list', params) },
  search(params: ListParams & { q: string }) { return post<PostListItem[]>('/posts/search', params) },
  getById(id: string) { return get<PostListItem>(`/posts/${id}`) },
  create(body: { title: string; content: string; categoryId: string }) { return post<Post>('/posts/create', body) },
  update(id: string, body: { title: string; content: string }) { return post<Post>(`/posts/${id}/update`, body) },
  delete(id: string) { return post<null>(`/posts/${id}/delete`) },
  togglePin(id: string) { return post<Post>(`/posts/${id}/pin`) },
  toggleLock(id: string) { return post<Post>(`/posts/${id}/lock`) },
  getTags(postId: string) { return post<Tag[]>(`/posts/${postId}/tags`) },
  setTags(postId: string, tags: string[]) { return post<Tag[]>(`/posts/${postId}/tags/set`, { tags }) },
}
```

### follow.ts
```ts
import { post } from '../client'
import type { User, ListParams } from '../types'

export const followService = {
  follow(userId: string) { return post<null>(`/users/${userId}/follow`) },
  unfollow(userId: string) { return post<null>(`/users/${userId}/unfollow`) },
  status(userId: string) { return post<{ following: boolean }>(`/users/${userId}/follow/status`) },
  followers(userId: string, params: ListParams) { return post<{ list: User[] }>(`/users/${userId}/followers`, params) },
  following(userId: string, params: ListParams) { return post<{ list: User[] }>(`/users/${userId}/following`, params) },
  counts(userId: string) { return post<{ followers: number; following: number }>(`/users/${userId}/counts`) },
}
```

## 可用模块（tags）

| Tag | 接口数 | 输出文件 | 导出名 |
|-----|--------|---------|--------|
| `认证` | 8 | `service/auth.ts` | `authService` |
| `用户` | 7 | `service/user.ts` | `userService` |
| `角色` | 3 | `service/role.ts` | `roleService` |
| `权限` | 3 | `service/permission.ts` | `permissionService` |
| `帖子` | 8 | `service/post.ts` | `postService` |
| `评论` | 4 | `service/content.ts` | `contentService` |
| `分类` | 3 | `service/category.ts` | `categoryService` |
| `标签` | 5 | `service/tag.ts` | `tagService` |
| `收藏` | 3 | `service/favorite.ts` | `favoriteService` |
| `点赞` | 4 | `service/like.ts` | `likeService` |
| `关注` | 6 | `service/follow.ts` | `followService` |
| `通知` | 4 | `service/notification.ts` | `notificationService` |
| `举报` | 4 | `service/report.ts` | `reportService` |
| `文件上传` | 4 | `service/upload.ts` | `uploadService` |
| `管理后台` | 1 | `service/admin.ts` | `adminService` |
| `系统设置` | 4 | `service/admin.ts` | （合并到 adminService） |
| `系统` | 1 | 跳过（health check 无需 service） | — |

## 操作流程

### ⚠️ GATE：选择目标模块（必须）

用户必须指定：**单个**、**多个**（逗号分隔）或 **全部**。

询问示例：
```
需要生成哪些模块？
- 单个：认证
- 多个：认证, 用户, 帖子
- 全部：all / 全部

可用模块：认证 | 用户 | 角色 | 权限 | 帖子 | 评论 | 分类 | 标签 | 收藏 | 点赞 | 关注 | 通知 | 举报 | 文件上传 | 管理后台
```

### Step 0：获取并分割规范（可选）

如果 `doc/orinforum/` 下已有分割好的 JSON，跳过此步。

```bash
# 直接使用项目根目录的 openapi.json
mkdir -p .agents/skills/openapi-to-service/doc/orinforum
cp openapi.json .agents/skills/openapi-to-service/doc/orinforum/api.v1.json

# 或从运行中的服务器获取最新版本
curl -o .agents/skills/openapi-to-service/doc/orinforum/api.v1.json \
  http://localhost:8080/swagger/doc.json

# 按 tag 分割
node .agents/skills/openapi-to-service/index.js
```

### Step 1：读取规范

优先从 `doc/orinforum/{tag}.json` 读取，或直接从项目根目录 `openapi.json` 中提取该 tag 的 paths 和 definitions。

**重要**：openapi.json 中的路径可能与实际路由不一致（如 openapi 用 GET，实际用 POST）。以 `packages/server/internal/router/router.go` 中的实际路由为准，同时参考 `packages/web/src/api/service/` 下已有的 service 文件。

### Step 2：确定路由和方法

对照 `router.go` 确认每个接口的实际 HTTP 方法和路径，不要盲目照搬 openapi.json。

### Step 3：生成 service 文件

按照项目约定生成，注意：
1. 只 import 实际用到的 client 方法（`get`/`post`/`postForm`）
2. 只 import 实际用到的类型，优先使用 `types.ts` 中已有的类型
3. 如需额外的 params 类型（如 `PostListParams`），在 service 文件中定义并 export
4. 导出名为 `{tag}Service` 对象（不是 class，不是 default export）
5. 方法名简短，不加冗余前缀

### Step 4：写入文件

输出到 `packages/web/src/api/service/{name}.ts`。

如果文件已存在，对比差异后更新，不要覆盖已有的自定义逻辑。

### Step 5：更新 index.ts（如需要）

如果生成了新的 service 文件，确认 `packages/web/src/api/index.ts` 中已有对应的 export：

```ts
export { xxxService } from './service/xxx'
```

## 常见错误

- **用 GET 代替 POST**：后端列表/操作接口几乎全是 POST + body，不是 GET + query
- **路径照搬 openapi.json**：openapi.json 路径可能过时，以 router.go 为准
- **重复定义 types.ts 中的类型**：`User`、`Post`、`ListParams` 等直接 import，不要重定义
- **方法名冗余**：`postService.listPosts()` 应为 `postService.list()`
- **忘记 export**：service 对象必须 export，方法内联在对象中
- **用 default export**：项目约定是具名 export（`export const xxxService = {}`）
