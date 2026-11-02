---
title: 后台总览
description: 管理后台的入口、准入判定、模块级门控与完整路由表
---

管理后台是主站里的一段路由（`app/web/app/admin/**`），不是单独的部署：同一个构建产物、同一个 origin、同一套部署，
组件与前台共用 `components/ui/` 下的同一批原语。

它也不走服务端渲染。`app/admin/layout.tsx` 用客户端挂载门把整段路由挡在 SSR 之外，
服务端只吐主站骨架，页面组件全部 `'use client'`。所以后台首屏永远是「正在加载后台…」的骨架，
数据来自浏览器发起的 API 请求。

一句话记法：**后台是前台的一个分区，不是第二个应用。**

## 准入：三种结局

准入判定集中在 `lib/admin/auth/access.ts`，用会话 cookie 拉 `GET /user/profile`——
以服务端返回的角色为准，不信任前端缓存的角色投影。判定顺序：

1. 角色是 `admin` 或 `moderator` → 放行。
2. 否则拉该角色的权限码，命中后台权限码清单任一 → 放行（给自定义角色留的口子）。
3. 都不满足 → 拒绝。

拒绝分两种，取决于会话状态：

| 情形 | 表现 |
|------|------|
| 未登录 | 跳主站 `/login?redirect=/admin/...`，登录后回到后台 |
| 已登录但无后台权限 | 直接送回进入后台之前那一页 |

普通用户被弹回前台时，回跳目标按「`sessionStorage` 记录的上次来路 → `document.referrer` → 站点地址」依次取，
拿不到任何可用目标才显示整页 403。也就是说，点错链接不会把人按在错误页上。

后台权限码清单刻意排除了 `user:read`、`role:read`、`stats:read` 这类普通用户也有的读权限——
否则任何注册用户都能进后台。完整清单在 `DASHBOARD_PERMISSION_CODES`，包含 `setting:manage`、`role:manage`、
`user:update`、`user:delete`、`banned:manage`、`batch:edit`、`announcement:manage`、`report:manage`、
`backup:manage`、`plugin:manage`、`level:manage`、`gate:manage` 等 21 个码。

:::note
后台没有独立登录页，登录统一走主站 `/login`；`?redirect=` 只接受站内相对路径。
:::

## 模块级门控

进了后台不等于每个模块都能看。`lib/admin/auth/route-access.ts` 维护「路径前缀 → 权限码数组」，
数组语义是 anyOf（命中任一即可）。同一份表干两件事：过滤侧栏菜单，以及把无权限的模块渲染成壳层内的「无权限」页。

| 路径 | 需要的权限码（任一） |
|------|--------------------|
| `/admin` | `stats:read` |
| `/admin/posts`、`/admin/posts/detail` | `post:read` |
| `/admin/comments`、`/admin/comments/detail` | `comment:delete` |
| `/admin/reports`、`/admin/moderation` | `report:manage` |
| `/admin/users`、`/admin/users/detail` | `user:read` |
| `/admin/roles` / `/admin/roles/detail` | `role:read` / `role:manage` |
| `/admin/settings`、`/admin/mcp`、`/admin/oauth-clients` | `setting:manage` |
| `/admin/trash` | `post:delete` 或 `comment:delete` |

其余模块的对应关系可在同一文件里逐条读到；未登记的路径不设门控，只要求已进后台。

:::caution
前端门控只负责体验：隐藏入口、避免点进去必然报错。真正的安全边界在后端——网关按「路径 → 权限码」
拦截每个请求，权限码再经统一判定：会话看角色权限，API Key 取 scope 与角色权限的交集。
改前端判定不会放宽任何接口，改后端判定也不会自动更新菜单。
:::

## 路由表

| 分组 | 路由 | 用途 |
|------|------|------|
| 首页 | `/admin` | 运营概览：统计卡、快捷入口、全局搜索、近期内容 |
| 论坛管理 | `/admin/posts`、`/admin/posts/detail?id=` | 帖子列表与详情 |
| 论坛管理 | `/admin/comments`、`/admin/comments/detail?id=` | 评论列表与详情 |
| 论坛管理 | `/admin/banned` | 禁言记录、手动禁言与解禁 |
| 论坛管理 | `/admin/tags` | 标签管理 |
| 论坛管理 | `/admin/trash` | 回收站（帖子 / 评论） |
| 运营管理 | `/admin/announcements` | 公告 |
| 运营管理 | `/admin/reports`、`/admin/reports/detail?id=` | 举报处理 |
| 运营管理 | `/admin/moderation` | 审核队列与审核模式 |
| 运营管理 | `/admin/spaces` | 版面树、可见性、版面级权限 |
| 站点自定义 | `/admin/customize/snippets` 等 | 自定义代码 / 主题 / 导航 / 首页模块 / 页面 |
| 用户与权限 | `/admin/users`、`/admin/users/detail?id=` | 用户列表与详情 |
| 用户与权限 | `/admin/invites` | 邀请码 |
| 用户与权限 | `/admin/roles`、`/admin/roles/detail?id=` | 角色列表与权限矩阵 |
| 用户与权限 | `/admin/groups` | 用户组 |
| 用户与权限 | `/admin/levels` | 等级定义 |
| 用户与权限 | `/admin/xp-rules` | XP 规则与连续签到阶梯 |
| 用户与权限 | `/admin/gates` | 门控规则 |
| 系统设置 | `/admin/api-keys` | API Key |
| 系统设置 | `/admin/webhooks` | Webhook |
| 系统设置 | `/admin/backups` | 备份 |
| 系统设置 | `/admin/oauth-clients` | OAuth 客户端 |
| 系统设置 | `/admin/plugins` | 插件 |
| 系统设置 | `/admin/mcp`、`/admin/mcp-servers` | 本站 MCP 工具与外部 MCP 服务 |
| 系统设置 | `/admin/security` | 安全事件 |
| 系统设置 | `/admin/sessions` | 会话管理 |
| 系统设置 | `/admin/settings?tab=` | 服务设置 |
| 系统设置 | `/admin/audit-logs` | 审计日志 |
| 不进菜单 | `/admin/search` | 全局搜索，仅 `admin:search` 持有者可达 |
| 认证 | `/admin/first-change-password` | 首次登录改密 |

「站点自定义」这组的用法见 [站点自定义](/configuration/customization/)；本组四个页面不重复它的编辑细节。

## 菜单在哪登记

菜单不是自动生成的。新增一个后台页面要同时改两处，否则路由存在但没人找得到：

1. `components/admin/layout/admin-shell.tsx` 的 `menuItems` —— 分组、图标、顺序；不想出现在菜单里的
   （如详情页、`/admin/search`）加 `hideInMenu`。
2. `lib/admin/auth/route-access.ts` 的 `ROUTE_PERMISSIONS` —— 该路径需要的权限码；漏登记等于对所有
   已进后台的账号放行。

## 首次登录改密

由种子数据创建的 `admin` 账号带「必须改密」标记，登录响应里的 `must_change_password` 为 `true`。

1. 用初始账号登录，打开 `/admin/first-change-password`。
2. 提交新密码（可同时改用户名）→ `POST /auth/first_change_password`。
3. 成功后标记被删除，页面会登出，再用新密码登录即可。

该接口只对 `admin` 角色开放；密码长度下限取设置 `password_min_length`，默认 8。

## 验证

1. 用普通账号打开 `/admin`：应被送回进入前的页面（或整页 403），URL 不会停在 `/admin`。
2. 用 admin 打开 `/admin`：侧栏出现全部有权限的分组；把某角色的 `report:manage` 去掉后重新进入，
   「举报管理 / 内容审核」消失，直接输 URL 则在壳层内看到「无权限」。
3. 用普通用户的会话调 `POST /api/v1/admin/stats`，应返回 403——前端门控不是边界。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 一直停在「正在加载后台…」 | 浏览器拿不到 `/user/profile` | 检查后端可达性、会话 cookie、`/api/v1` 反代 |
| 菜单只剩一两个分组 | 该角色的权限码不足 | 在「角色权限」里补码，重新登录刷新缓存 |
| 直接输 URL 看到壳层内「无权限」 | `route-access.ts` 的 anyOf 一条都没命中 | 注意 `/admin/posts` 要 `post:read`，不是 `post:create` |
| 新页面访问 404 | 目录不在 `app/web/app/admin/` 下，或页面不是客户端组件 | 后台页面一律 `'use client'` |
