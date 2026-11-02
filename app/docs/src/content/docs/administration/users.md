---
title: 用户与权限
description: 用户管理、角色与权限码、用户组、禁言与封号、邀请注册、等级与 XP 调整
---

后台的「用户与权限」回答三件事：这个人现在是什么角色、他能做什么、他做错了什么该怎么处置。前两件由角色与权限码决定，第三件由禁言（禁止发言）与封号（禁止登录）两套独立机制承担。

## 前置条件

- 已用 `admin` 或 `moderator` 账号进入后台（见 [后台总览](/administration/overview/)）。
- 改角色需要 `user:update`；改角色权限需要 `role:manage`；禁言记录列表要 `banned:manage`，执行禁言要 `user:update`；
  调 XP 需要 `level:manage`。菜单会按你的权限码自动收缩。

## 用户列表与详情

`/admin/users` 是分页列表，每行可直接改角色、禁言、查看详情；审批模式下的待审账号多一个「审批」按钮。
勾选多行可批量删除或批量改角色，**单次最多 100 个**。

`/admin/users/detail?id=<用户ID>` 分五个分区：

| 分区 | 内容 | 数据来源 |
|------|------|----------|
| 基本信息 | 用户名、邮箱、角色、注册时间 | `GET /user/{id}` |
| 成长 | XP 分解（各指标的次数 × 单次 XP）、等级、人工调整记录 | `GET /admin/users/{id}/xp`（需 `level:manage`） |
| 帖子 | 该用户发过的帖子 | `POST /admin/user/{id}/activities` |
| 评论 | 该用户发过的评论 | 同上 |
| 被举报 | 该用户内容被举报的记录 | 同上 |

## 角色变更

在用户详情或列表行内打开「修改用户角色」，选目标角色后确认 → `POST /user/{id}/role/update`。

- **角色变更会吊销该用户的全部会话**，包括封号（把角色设为 `ban`）。对方下一次请求就失效，不需要等会话过期。
- 系统会拦住「把最后一个持有 `role:manage` 的用户降级」的操作，返回 403——否则没人能再管理权限。
- 用户角色变化时会同步系统组成员身份（见下方「用户组」）。

角色是新旧两套机制的汇合点：

| 角色 | 含义 |
|------|------|
| `admin` | 管理员，默认拥有全部权限码 |
| `moderator` | 版主，内容与举报相关的运营权限 |
| `user` | 普通用户，可发帖、评论、举报 |
| `pending_user` | 审批注册的待审账号，可登录但不可发帖评论 |
| `ban` | 封号：登录直接被拒，会话同时吊销 |

## 权限码与角色矩阵

平台级权限码共 **61 个**，按前缀分组：`post` / `comment` / `tag` / `space` / `user` / `role` /
`report` / `announcement` / `setting` / `stats` / `banned` / `batch` / `admin` / `reconcile`，
另有 `motion` / `api-key` / `session` / `security` / `backup` / `plugin` / `level` / `gate` / `customize` 等域。

`/admin/roles` 列出全部角色，`/admin/roles/detail?id=<角色>` 是权限矩阵：

- 上半部分是全局权限勾选（对所有版面生效）。
- 下半部分是版面级覆盖，只对 **8 个按版面校验的权限码**提供：`post:update`、`post:delete`、
  `post:pin`、`post:lock`、`comment:update`、`comment:delete`、`user:update`、`space:update`。
  其余权限码是全局判定，给它设版面覆盖不会生效。
- 版面级覆盖的语义是**完全替换**：某角色在某版面设了覆盖，就按覆盖算，不再并上全局；
  子版面沿祖先链找最近的覆盖，整条链都没有才回退全局。这带来一个容易踩的坑——
  给子版面加一条覆盖，父版面的设置对它不再生效。

默认矩阵的边界值得记住：

- `admin`：全部权限码，含 `setting:manage`、`role:manage`、`backup:manage`、`plugin:manage`、
  `level:manage`、`gate:manage` 与 `customize:*`（站点自定义能注入脚本，只给 admin）。
- `moderator`：帖子 / 评论 / 标签 / 版面 / 举报 / 禁言 / 批量 / 统计，加上 `role:read`；
  没有 `role:manage`、`setting:manage`、`user:update`、`backup:manage` 等。
- `user`：发帖、评论、举报加全部自服务权限。
- `pending_user`：只能读与使用自服务功能，发帖评论权限缺失。
- `ban`：没有业务权限。

`/permission/...` 下的接口（`role` / `space` / `space/sync` / `sync`）就是这张矩阵的读写通道，
`space/sync` 与 `sync` 需要 `role:manage`。

## 用户组

`/admin/groups` 管理两类组：

- **系统组**：与角色同名（`admin` / `moderator` / `user` / `pending_user` / `ban`），
  成员身份随用户角色自动同步，不要手工维护。
- **自定义组**：自建、自管成员。可见性 `public` / `closed` / `secret` 只影响前台展示语义。

成员角色有 `owner` / `moderator` / `member` 三档；「权限」按钮为组挂权限码（`POST /groups/{id}/permissions/sync`）。
注意：后台准入、菜单与网关门控只读**角色权限**，组权限不会替角色开门——给组加码不等于给成员后台访问权。

## 禁言与封号

这是两套独立机制，别混用：

| 机制 | 效果 | 用户能否登录 |
|------|------|-------------|
| 禁言（`banned` 表） | 拦截写操作（发帖、评论等） | 能 |
| 封号（角色设为 `ban`） | 登录直接拒绝，会话被吊销 | 不能 |

禁言分两级：

- **全局禁言**：`POST /user/{id}/banned`，body 传 `reason`、`is_permanent`，临时禁言再给
  `duration_minutes`（或 `expires_at`）。
- **版面禁言**：`POST /user/{id}/space/{space_id}/banned`，只在指定版面拦截写操作。

不传时长且不是永久 → 后端按永久禁言处理（避免「禁了但永不过期」和「禁了但立刻过期」两种歧义）。
`/admin/banned` 列出全部禁言记录（可按 `scope`、`space_id` 过滤），行内「解禁」调
`POST /user/{id}/unbanned` 或对应的版面解禁接口。每次禁言 / 解禁都写审计日志。

:::caution
禁言不吊销会话——被禁言的用户可以继续浏览、登录；要立刻让他下线，必须改角色（封号）或直接撤销他的会话。
:::

## 邀请码与注册模式

注册行为由设置 `register_mode` 决定：`all`（开放）、`email` / `username`（限定标识类型）、
`invite`（邀请码）、`approve`（审批，注册后是 `pending_user`）。

`/admin/invites` 用于邀请码模式：

1. 「新建邀请码」→ `POST /admin/invites/create`，可设最大使用次数（0 表示无限）与过期时间。
2. 列表展示使用情况与过期时间；删除即作废 → `POST /admin/invites/{id}/delete`。
3. 邀请模式下注册必须携带有效 `invite_code`，无效码直接拒登。

审批模式下，新账号在用户列表里显示「审批」按钮 → `POST /admin/users/{id}/approve_registration`，
通过后角色从 `pending_user` 变为普通用户。

## 等级与 XP 的人工调整

- `/admin/levels`：等级定义（等级值、名称、Key、XP 门槛、保级窗口天数、是否仅人工授予）。
  保级窗口为 0 表示该等级没有保级条件；「仅人工」的等级不参与自动升降。
- `/admin/xp-rules`：指标 → 单次 XP、每指标每日上限（0 不限）、启用开关；下面是连续签到阶梯
  （连续天数 → 奖励 XP）。
- 用户详情「成长」页：看 XP 分解后可直接调整，填写 `delta`（可负）与**必填的原因**
  → `POST /admin/users/{id}/xp/adjust`；历史调整可查、可删（`/admin/xp-adjustments/{id}/delete`）。

XP 是行为聚合值：删帖、取消精华、取消点赞都会自动回收，改分值对全员立即生效。
需要整体对账时用 `POST /admin/trust/recompute`（或 CLI `recompute-xp`），不要手工改库。

## 会话与安全事件入口

`/admin/sessions` 与 `/admin/security` 展示的是**当前登录账号**的会话与安全事件，不是全站列表：

- 会话：设备名、最近活跃时间，可单条撤销或「全部登出」。
- 安全事件：时间、类型、严重级别（`low` / `medium` / `high` / `critical`）、IP、详情、是否已处理。

要处置别人的登录状态，走「改角色 / 封号」（会吊销对方全部会话）；后台没有「踢掉某个用户所有会话」的按钮。

## 验证

1. 改一个用户的角色，让他刷新任意页面：应立即变成未登录或权限不足（会话已吊销）。
2. 对某个普通用户做全局禁言，再用该账号发帖：应返回 403；解禁后恢复。
3. 把某角色的一条权限码取消，重新登录后对应菜单消失；直接调该接口返回 403。
4. 给某个用户调 `+50` XP 并写原因，用户详情「成长」页的分解与调整记录应同步更新。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 改角色返回 403「无法移除最后一个管理员」 | 目标是最后一个持 `role:manage` 的用户 | 先给另一个账号加该权限 |
| 禁言后对方还能发帖 | 设的是版面禁言，或禁言已过期 | 检查 `scope` 与 `expires_at`，改全局永久 |
| 版面级权限设了不生效 | 该权限码不在 8 个版面级码里 | 改用全局设置，或确认码属于版面级 |
| 「成长」页空白 / 403 | 当前账号没有 `level:manage` | 由持该权限的账号操作，或补权限码 |
| 邀请码注册失败 | 码已用完、已过期，或注册模式不是 `invite` | 查 `/admin/invites` 与 `register_mode` |
