# OrinForum 开发计划

## 项目概述

OrinForum 是一个社群/论坛项目，基于以下技术栈:

- **后端:** Go (Iris v12) + GORM，支持 SQLite / MySQL / PostgreSQL / MongoDB
- **前端:** SolidJS + TanStack Router + Tailwind CSS
- **架构:** Monorepo (pnpm workspace)

---

## 当前状态

### 已完成

- 认证系统 (注册/登录/JWT/密码管理)
- 用户 CRUD + 角色权限系统 (RBAC + Permission)
- 帖子/评论/分类 CRUD
- 中间件 (JWT/权限校验/封禁检查/CORS)

---

## Phase 1: 基础完善 (核心可用) ✅

**目标:** 论坛能跑起来，用户能注册、发帖、浏览

### 后端 ✅

| 任务 | 说明 | 状态 |
|------|------|------|
| 用户资料编辑 | User 增加 Bio 字段，添加 `PUT /api/v1/user/profile` 接口 | ✅ |
| 帖子浏览计数 | `GET /posts/{id}` 时自动 +1 ViewCount | ✅ |
| 帖子列表增强 | 列表返回作者信息、评论数、分类名 (关联查询) | ✅ |
| 标签系统 | 新增 Tag + PostTag 模型，帖子支持多标签 CRUD | ✅ |
| 搜索功能 | 帖子标题/内容模糊搜索 `GET /api/v1/posts/search?q=xxx` | ✅ |

### 前端 (暂不开发)

| 任务 | 说明 | 状态 |
|------|------|------|
| 登录/注册页 | 对接 auth API，JWT 存 localStorage | 待开发 |
| 首页改造 | 对接帖子列表 API，替换硬编码数据 | 待开发 |
| 帖子详情页 | 展示帖子内容、评论列表、发表评论 | 待开发 |
| 发帖页 | 标题 + 内容 + 分类选择 + 标签 | 待开发 |

---

## Phase 2: 社交互动 ✅

**目标:** 用户之间有互动，内容有反馈

| 任务 | 说明 | 状态 |
|------|------|------|
| 点赞系统 | PostLike / CommentLike 模型 + toggle API | ✅ |
| 收藏功能 | PostFavorite 模型，用户可收藏帖子 | ✅ |
| 用户关注 | UserFollow 模型，关注/粉丝列表和计数 | ✅ |
| 个人主页 | 前端: 发帖列表、收藏、关注/粉丝 | 待开发 |
| 帖子排序 | 按热度(点赞+浏览)、最新、最热排序 | 待开发 |

---

## Phase 3: 通知与消息 ✅

**目标:** 用户感知社区动态

| 任务 | 说明 | 状态 |
|------|------|------|
| 通知系统 | Notification 模型 (评论/点赞/关注通知) | ✅ |
| 通知 API | GET /notifications, PUT read, PUT read_all | ✅ |
| 未读计数 | GET /notifications/unread_count | ✅ |
| 前端通知铃铛 | Header 显示未读数，下拉展示通知列表 | 待开发 |
| @提及 | 评论中 @username 触发通知 | 待开发 |

---

## Phase 4: 内容管理增强 ✅

**目标:** 内容质量管控

| 任务 | 说明 | 状态 |
|------|------|------|
| 文件上传 | 图片上传接口 POST /upload，存储到本地 | ✅ |
| Markdown 支持 | 帖子内容支持 Markdown 渲染 | 待开发 |
| 帖子置顶/锁定 | 权限控制 (post:pin, post:lock) | ✅ |
| 内容举报 | Report 模型，用户可举报帖子/评论 | ✅ |
| 举报管理 | Admin 端审核举报、处理违规内容 | ✅ |
| 评论编辑 | PUT /posts/{id}/comments/{id} | ✅ |

---

## Phase 5: 管理后台 ✅

**目标:** 站长可管理整个站点

| 任务 | 说明 | 状态 |
|------|------|------|
| Admin 仪表盘 | GET /admin/stats (用户/帖子/评论统计) | ✅ |
| 用户管理 | 用户搜索 (GET /user/search?q=&role=) | ✅ |
| 内容管理 | 通过帖子/评论 CRUD + 置顶/锁定/删除实现 | ✅ |
| 分类管理 | 分类 CRUD 已有 | ✅ |
| 角色权限管理 | 角色/权限 CRUD + sync 已有 | ✅ |
| 系统设置 | Setting 模型 + GET/PUT/DELETE /admin/settings | ✅ |

---

## Phase 6: 高级功能 (可选)

| 任务 | 说明 | 状态 |
|------|------|------|
| Redis 集成 | Token 黑名单、缓存、排行榜 | - [ ] |
| WebSocket | 实时通知推送 | - [ ] |
| 私信系统 | 用户间一对一消息 | - [ ] |
| 邮件通知 | 重要事件发送邮件 | - [ ] |
| OAuth 社交登录 | GitHub / Google 登录 | - [ ] |
| Rate Limiting | 防刷中间件 | - [ ] |
| 国际化 i18n | 中英文切换 | - [ ] |

---

## 基础设施 ✅

| 任务 | 说明 | 状态 |
|------|------|------|
| Swagger 文档 | 47 个 API 端点全部添加 swaggo 注解，/swagger 访问 | ✅ |
