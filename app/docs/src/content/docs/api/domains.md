---
title: 按域查端点
description: 按业务域索引常用端点，附真实请求示例
sidebar:
  order: 3
---

后端共 **313 个 path / 314 个操作**，按业务域分组。下面每域给「这个域能做什么 + 最常用的几个端点 + 一个可粘贴的示例」。

完整清单以 [openapi.json](/api/download/) 与运行时 `/swagger/` 为准。

## 认证 `认证`

注册、登录、会话管理、密码变更、邮箱验证码流程、MFA。

| 端点 | 说明 |
|------|------|
| `POST /api/v1/auth/register` | 注册（限流） |
| `POST /api/v1/auth/login` | 登录，返回 `sid` |
| `POST /api/v1/auth/refresh` | 单用轮换：签发新 sid 并作废旧 sid |
| `POST /api/v1/auth/logout` | 登出（删会话行） |
| `POST /api/v1/auth/change_password` | 改密 |
| `POST /api/v1/auth/mfa/verify` | TOTP 第二步校验 |

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<密码>"}'
```

## 帖子 `帖子`

| 端点 | 说明 |
|------|------|
| `POST /api/v1/posts/list` | 列表（支持版面 / 标签 / 排序 / 递归子版） |
| `POST /api/v1/posts/create` | 发帖，初始状态由 `moderation_mode` 决定 |
| `GET /api/v1/posts/{id}` | 详情（受限内容返回 403 并提示所需等级） |
| `POST /api/v1/posts/{id}/update` | 更新（作者 OR 有权限） |
| `POST /api/v1/posts/{id}/delete` | 删除（软删除，进回收站） |
| `POST /api/v1/posts/{id}/pin` | 置顶 |
| `POST /api/v1/posts/{id}/status` | 直改状态（审核通过 / 拒绝） |

```bash
curl -s -X POST http://localhost:8080/api/v1/posts/list \
  -H 'Content-Type: application/json' \
  -d '{"page":1,"pageSize":20,"sort_by":"latest"}'
```

:::tip[递归子版]
列表带 `space_id` 且 `include_sub: true` 时，后端把过滤展开成「本版 + 全部后代版」。
:::

## 内容（评论）`内容`

| 端点 | 说明 |
|------|------|
| `POST /api/v1/posts/{postId}/contents/create` | 发表评论（支持 `parentId` 嵌套） |
| `POST /api/v1/posts/{postId}/contents/list` | 评论列表 |
| `POST /api/v1/posts/{postId}/contents/{id}/delete` | 删除 |

一条评论只产生**一条**通知：命中「帖子作者 / @提及 / 关注话题」多条路径时按优先级去重。

## 版面 `版面`

树形版面体系，带图标、slug、排序、启禁状态与可见性（`public` / `login` / `level`）。

| 端点 | 说明 |
|------|------|
| `POST /api/v1/spaces/tree` | 树形列表 |
| `POST /api/v1/spaces/create` | 创建 |
| `GET /api/v1/spaces/{space_id}` | 详情 |
| `POST /api/v1/spaces/{space_id}/update` | 更新 |

## 用户与关系 `用户` `关注` `禁言`

资料编辑、角色变更、禁言/解禁、关注、拉黑。

| 端点 | 说明 |
|------|------|
| `GET /api/v1/user/profile` | 当前用户资料 |
| `POST /api/v1/user/{id}/update` | 更新资料 |
| `POST /api/v1/users/{userId}/follow` | 关注（禁止自关注） |
| `POST /api/v1/admin/banned/create` | 禁言 |

## 成长体系 `信任等级` `签到` `排行榜` `徽章`

XP 是**行为聚合**而不是累加计数器：`total_xp = Σ(指标计数 × 当前规则) + Σ(连续签到里程碑) + Σ(人工调整)`。删帖 / 取消点赞 / 审核拒绝会自动回收，改分值立即对全员生效。

| 端点 | 说明 |
|------|------|
| `GET /api/v1/levels` | 公开等级表 |
| `POST /api/v1/checkin` | 签到 |
| `GET /api/v1/checkin/status` | 今日状态与连续天数 |
| `POST /api/v1/leaderboard/list` | 排行榜（积分 / 签到 / 发帖） |

## 审核与治理 `举报` `自治动议` `管理后台`

四种审核模式：`off` / `async`（默认，先公开后复查）/ `external`（发布前调外部服务）/ `manual`。

| 端点 | 说明 |
|------|------|
| `POST /api/v1/reports/create` | 举报 |
| `POST /api/v1/admin/moderation/pending/list` | 待审队列 |
| `POST /api/v1/admin/moderation/{id}/approve` | 通过 |
| `POST /api/v1/motions/create` | 发起动议 |

## 通知与消息 `通知` `私信`

| 端点 | 说明 |
|------|------|
| `POST /api/v1/notifications/list` | 通知列表 |
| `GET /api/v1/notifications/stream` | SSE 未读数推送（15s 心跳） |
| `POST /api/v1/pm/conversations/create` | 创建会话 |

## 站点自定义 `站点自定义`

不改代码就能改外观与结构：主题 token、CSS/JS/HTML 片段、导航、首页模块、自定义页。

| 端点 | 说明 |
|------|------|
| `POST /api/v1/customization/public` | 前台聚合（导航 / 模块 / 生效片段 / token） |
| `POST /api/v1/admin/customization/snippets/create` | 新建片段 |
| `POST /api/v1/admin/customization/tokens/set` | 批量设置主题 token |
| `POST /api/v1/admin/customization/pages/{id}/publish` | 发布自定义页 |

**分级生效**：机器凭证（API Key / MCP）写 JS / HTML / 导航 / 模块 / 自定义页会被强制存草稿，需人工启用；只有凭证自身 scope 含 `customize:publish` 才能直接生效。

## 平台能力 `系统设置` `权限` `角色` `用户组` `邀请码` `API Key` `Webhook` `插件` `主题` `备份` `会话` `安全` `MFA` `SSO`

| 端点 | 说明 |
|------|------|
| `POST /api/v1/settings/public` | 公开设置（站点名、Logo 等） |
| `POST /api/v1/admin/settings/list` | 全部设置 |
| `POST /api/v1/permission/list` | 权限码清单 |
| `POST /api/v1/admin/api-keys/create` | 创建 API Key |

## MCP `MCP` `外部 MCP`

见 [MCP 概览](/mcp/overview/)。
