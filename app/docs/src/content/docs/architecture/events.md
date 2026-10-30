---
title: 事件与缓存
description: 事件总线类型全集、通知扇出去重、读穿缓存
sidebar:
  order: 4
---

## 事件总线

基于 NATS core pub/sub，每个事件类型用**队列订阅**，集群内每事件只消费一次。
业务 service 发布，NotificationService 订阅。

类型全集（`internal/pkg/eventbus/events.go` 是唯一权威）：

| 事件 | 触发时机 |
|------|----------|
| `post.created` | 发帖 |
| `post.closed` | 关帖 |
| `content.created` | 评论创建 |
| `content.liked` | 点赞 |
| `content.replied` | 回复某条评论 |
| `content.watched` | 关注话题有新内容 |
| `user.created` / `user.registered` | 注册（历史别名，Auth 双发） |
| `user.followed` | 关注 |
| `user.mentioned` | @提及 |
| `report.created` | 举报 |
| `moderation.decided` | 审核结论 |
| `motion.created` / `motion.resolved` | 动议发起 / 决议 |

**新增事件**必须同时加进常量块与 `AllEventTypes()`：漏了会丢零值指标序列，
而告警规则里的 `absent()` 正是靠它发现「事件静默消失」。

## 通知扇出的唯一硬约束

**一条评论只产生一条通知。**

同一次评论可能同时命中三条扇出路径（帖子作者 / @提及 / 关注话题）。
实现用一个共享的 `notified` 集合按「作者 > 提及 > 关注」优先级去重，
集合随后传给提及与关注扇出。新增任何扇出步骤都必须接收并遵守同一个集合，
否则用户会重复收到通知。

## 读穿缓存

热点只读路径接了 KV 读穿缓存：

| 路径 | 缓存内容 | TTL | 失效时机 |
|------|----------|-----|----------|
| 帖子列表 | **仅匿名**列表页（全部过滤条件进 key） | 10s | 帖子增删改、状态变更、版面写操作 |
| 标签列表 | 标签分页 | 60s | 标签增删改、设置帖子标签 |
| 版面树 / 扁平列表 | 树形与扁平 | 60s | 版面增删改 |

:::danger[登录态一律不进共享缓存]
登录列表会注入已读状态、拉黑作者排除与等级可见性——缓存它就是跨用户串数据。
实现上只在 `viewerID` 缺失时读写缓存。新增过滤条件时必须把它加进缓存 key，
否则不同过滤会命中同一条缓存。
:::

## 会话与限流的 KV 使用

| 用途 | key 形状 | 说明 |
|------|----------|------|
| 会话缓存 | `sess:<sid>` | read-through，权威仍是 `user_sessions` 表行 |
| 端点限流 | `rl:<ip>:<policy>` | 滑动窗口近似（SetNX 起窗 + Incr + 比较） |
| 全局限流 | `rl:ratelimit:<ip>` | 每 IP 上限 |
| API Key 限流 | `apikey_rl:<keyID>` | 每 Key 预算 |
| OAuth state | 前缀 `oauth:` | CSRF state |
| 设置 / 禁言 | `setting:<key>` / ban 键 | 短 TTL 缓存 |

键形状由 `EndpointKey` / `GlobalKey` 单点生成并有测试钉住——**改形状等于清零正在计数的窗口**。
