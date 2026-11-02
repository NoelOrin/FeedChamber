---
title: 内容管线
description: 审核四种模式、审核队列与留痕、举报处置、禁言与封号、可见性三档与门控规则
---

一条内容从提交到公开要过三道独立关卡：**审核模式**决定它是否需要人工过目，**可见性**
决定谁能看见它，**门控规则**决定这个用户当前是否被允许写。举报、禁言与封号是事后处置。

## 前置条件

- 审核队列与举报：`report:manage` 权限，后台 `/admin/moderation` 与 `/admin/reports`。
- 门控规则：`gate:manage` 权限，后台 `/admin/gates`。
- 禁言与封号：`banned:manage`、`user:update`，后台 `/admin/banned` 与用户详情页。
- 审核模式的开关在 `/admin/settings?tab=content`。

## 审核的四种模式

模式存在设置键 `moderation_mode` 里，四选一，**默认 `async`**。

| 模式 | 何时判定 | 行为 |
|------|----------|------|
| `off` | 不判定 | 发布即公开，不跑规则，也不写审核记录 |
| `async` | 发布**后**异步 | 先公开，后台跑规则；命中就把状态改回待审，交人工判断 |
| `external` | 发布**前**同步 | 调外部审核服务；服务不可用或超时则自动退回 `async` |
| `manual` | 发布即待审 | 所有内容进队列，必须人工通过才公开 |

几个容易踩的点：`async` 命中后**只把内容挪进队列**，不会直接删帖或拒稿，最终判断权在人；
`external` 判 `block` 时接口返回参数错误并留审核记录，判 `flag` / `replace` 则落库为待审；
旧布尔开关 `post_approval` 只在 `moderation_mode` 从未落库时生效（等价 `manual`），
在后台选过模式后即被忽略——新部署不要再用它。

## 审核队列与判定留痕

入口 `/admin/moderation`，接口 `POST /api/v1/admin/moderation/pending` 取队列，
`POST /api/v1/admin/moderation/{id}/approve` 与 `/reject` 做判定。
注意这两个动作的参数是**审核单 id**，不是帖子 id。

每张审核单都留有判定依据，事后可复盘：

| 字段 | 内容 |
|------|------|
| `target_type` / `target_id` | 帖子或评论，以及对应 id |
| `status` | `pending` / `approved` / `rejected` / `auto_pass` |
| `action` | `allow` / `block` / `replace` / `flag` |
| `strategy` | 记录来源：`manual`、`async`、`external`、`gate`、`tl0` |
| `matched_rules` | 命中的规则，JSON 数组 |
| `original_body` / `filtered_body` | 原文与替换后文本 |
| `reviewer_id` / `reviewed_at` | 谁在什么时候判的 |

一条重要的设计：**凡是进入待审的内容都会建审核单**——人工模式与门控的 `require_review`
都会落单，否则队列为空，审核动作也没有对象可操作。

## 举报

用户举报走 `POST /api/v1/reports/create`（每 IP 每分钟 10 次），后台在 `/admin/reports`，
详情页 `/admin/reports/detail?id=`。

| 状态 | 含义 |
|------|------|
| `pending` | 待处理（新建举报的默认值） |
| `resolved` | 已处理，采纳 |
| `dismissed` | 已处理，驳回 |

处理动作是 `POST /api/v1/reports/{id}/status`，可附带 `action` 与 `note` 记录说明。举报
本身不改内容状态：删帖、禁言、封号仍要各自动作，举报单只负责留痕。

## 禁言与封号

两套机制，作用面完全不同：

| 机制 | 存储 | 效果 |
|------|------|------|
| 禁言 | `banned` 表，支持 `global` 与 `space` 两级 | 全局禁言在登录阶段就返回 1015；已签发的会话在受保护端点上一律 404（信封里仍是 1015） |
| 封号 | 角色改为 `ban` | 登录被拒，且改角色会吊销该用户全部会话 |

禁言时长二选一：`is_permanent=true`，或给 `duration_minutes` / `expires_at`，过期自动
失效、不用手工解禁。版面级禁言只影响该版面内的写操作；全局禁言影响所有受保护端点，
但公开内容仍可读。接口：`POST /api/v1/user/{id}/banned`、`/unbanned`，版面级是
`/user/{id}/space/{space_id}/banned` 与 `/unbanned`；后台列表在 `/admin/banned`。

## 可见性三档

可见性挂在版面上：`spaces.visibility ∈ public | login | level`，配合 `spaces.min_level`。

| 档位 | 谁能看 |
|------|--------|
| `public` | 所有人，含游客（游客按 TL0 计） |
| `login` | 已登录用户；等效于等级 1 |
| `level` | 等级 ≥ `min_level` 的用户 |

关键是**沿祖先链取最严**：子版面继承父版面策略，`level` 档多级继承取最大的 `min_level`；
单篇帖子还能用 `posts.min_level` 覆盖，与版面策略取更高者。

六个内容出口共用同一套判定：列表、详情、搜索、sitemap、RSS、批量查询——不会出现「列表里
看不见、直接拼 URL 却能打开」。详情被拒返回 403 并告知「需要等级 X，当前等级 Y」，未发布
或不存在的资源返回 404。作者本人始终能看自己的帖子，持 `report:manage` 的审核者不受等级
门槛限制。`level` 档版面不进 sitemap——爬虫是匿名访问者，收录打不开的地址没有意义。

## 门控规则

入口 `/admin/gates`，权限码 `gate:manage`。规则结构是**优先级 + 能力域 + 条件（JSON
数组）+ 动作**。五个能力域：读内容（`content_view`）、发帖（`post_create`）、评论
（`comment_create`）、外链（`external_link`）、限流（`rate_limit`）；每个域有一个显式
默认动作，没有规则命中时就执行它（外链域默认 `deny`）。谓词是固定枚举，后台表单化配置，
不提供自由表达式：

| 谓词 | 说明 |
|------|------|
| `level` / `xp` / `author_level` | 用户等级、XP、作者等级，支持 `eq ne gt gte lt lte` |
| `space_id` / `tag` | 版面、标签，支持 `eq ne in` |
| `is_author` / `is_anonymous` / `email_verified` / `banned` / `exiled` / `contains_external_link` | 布尔判断，只支持 `eq` |
| `window_metric` | 窗口指标：活跃天数、发帖数、获赞数、成立举报数 |

动作只有四个：`allow`、`deny`、`require_review`（转待审）、`rate_limit`（按动作参数限流）。
求值顺序是**优先级降序，首个命中的规则生效**；同一能力域内优先级不允许重复，保存时就会
报冲突。所有条件之间是 AND——空条件列表的规则永远不会命中，这是有意的保护。

首启会 seed 两条外链规则：TL0 禁止外链、等级 ≥1 允许，演示「域默认 deny + 显式放行」的
写法，改之前先看清方向。后台「以用户试算」调 `POST /api/v1/admin/gates/simulate`，用的
就是线上同一个求值函数，结果与真实请求一致。每次命中都会写一条带 trace id 的结构化日志
（域名、动作、规则 id、优先级）——排查「为什么这条帖子进了待审」从这里入手。

## 机器凭证写入内容

API Key 与 MCP 工具写入的帖子、评论，走**完全相同**的审核模式与门控路径，没有特权通道：
不会因为来自机器就被放行，也不会被强制成草稿。「机器凭证写高风险内容强制草稿」只存在于
站点自定义（JS/HTML 片段、首页 HTML 模块、自定义页），见
[站点自定义](/configuration/customization/)。两处别混。

## 验证

```bash
# 1. 看当前审核模式（管理员接口，该键不属于公开设置）
curl -s -X POST http://localhost:8080/api/v1/admin/settings/moderation_mode \
  -H 'Authorization: Bearer <sid>' | jq '.data'

# 2. 发一条帖子，看返回的状态
curl -s -X POST http://localhost:8080/api/v1/posts/create \
  -H 'Authorization: Bearer <sid>' -H 'Content-Type: application/json' \
  -d '{"title":"hello","content":"first post","space_id":""}' | jq '.data.status'

# 3. 审核队列（需要 report:manage）
curl -s -X POST http://localhost:8080/api/v1/admin/moderation/pending \
  -H 'Authorization: Bearer <sid>' -H 'Content-Type: application/json' -d '{}' | jq '.data | length'
```

`manual` 模式下第 2 步应是 `pending`，`off` / `async` 下应是 `approved`。
再打开一个无痕窗口访问 `level` 档版面，应当看到 403 提示所需等级。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 发帖成功但全站看不到 | 模式是 `manual`，或门控 `require_review` 命中 | 到 `/admin/moderation` 审核；在 `/admin/gates` 试算确认 |
| `async` 模式下帖子突然消失 | 异步规则命中，状态被改回待审 | 属预期；审核通过即恢复 |
| 审核队列一直是空的 | 内容根本没进待审（模式是 `off` / `async` 且未命中） | 先确认 `moderation_mode`，再看门控动作 |
| `external` 模式下发帖变慢 | 同步调外部服务，超时后回退 `async` | 属预期；嫌慢就换回 `async` |
| 用户能登录但什么都打不开 | 全局禁言：受保护端点统一 404 | 查 `/admin/banned`，解禁后让用户重新登录 |
| 子版面明明设了 public 却要求登录 | 父版面是 `login`，沿祖先链取了最严 | 逐级检查祖先版面的 `visibility` |
| 规则保存报「优先级已被占用」 | 同域内优先级重复 | 换一个优先级数字 |
