---
title: 实时事件（SSE）
description: 通知流与通用流的端点、事件格式、15 秒心跳、客户端重连与 401 刷新、反向代理读超时处理
---

站内实时推送走 Server-Sent Events，只有一个方向：服务端 → 客户端。两个端点用途不同，
鉴权相同（会话 sid），权限码分别是 `notification:self` 与 `realtime:self`。

| 端点 | 推送内容 |
|------|----------|
| `POST /api/v1/notifications/stream` | 未读通知数 |
| `POST /api/v1/realtime/stream` | 通用事件信封（可订阅频道） |

两者都是 `POST`。浏览器原生 `EventSource` 既不能带自定义头也不能发 `POST`，所以要用
`fetch` 读 `response.body`——`@feedchamber/sse-client` 做的就是这件事。

## 通知流

```bash
curl -N -X POST http://localhost:8080/api/v1/notifications/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Accept: text/event-stream'
```

连上后立刻收到当前未读数，之后每次变化再推一次；payload 只有未读数：

```
data: {"unread":3}

: keepalive

data: {"unread":4}
```

字段名是 `unread`，与 `/notifications/unread_count` 返回的 `{"count":n}` 不同，别共用
一个解析器。

### 心跳

空闲的通知流一个字节都不产生，任何带读超时的中间层都会把它切断（nginx 默认
`proxy_read_timeout` 是 60 秒）。因此处理器每 **15 秒**写一行 SSE 注释：

```
: keepalive
```

注释行以 `:` 开头，按规范客户端必须忽略；前端解析器也只认 `data: ` 开头的行，所以它
不会被误当成一个空事件。

如果 KV 订阅失败（例如 NATS 短暂不可用），端点不会直接断开，而是退化为按设置
`sse_polling_interval`（默认 5 秒）轮询未读数并推同样的 payload。客户端行为不变。

## 通用流

```bash
curl -N -X POST http://localhost:8080/api/v1/realtime/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Accept: text/event-stream' -H 'Content-Type: application/json' \
  -d '{"channels":["notif:01J…"]}'
```

频道名只允许**自己的通知频道** `notif:<你的用户 ID>`；别人的频道被静默丢弃，一个都不
合法时自动回落到自己的频道。事件带 `id:`：

```
id: 7
data: {"type":"message","channel":"notif:01J…","payload":{},"id":7}

id: 8
data: {"type":"message","channel":"heartbeat","payload":{"ts":1793000000},"id":8}
```

:::note[两种流的心跳形态不同]
通知流用 SSE **注释行** `: keepalive` 保活；通用流把心跳当成**一条普通事件**
（`channel: "heartbeat"`）推出去，客户端要先按 `channel` 过滤再处理业务事件。
:::

## 客户端：重连与 401 刷新

`SSEClient` 的可配置项：`url`、`method`（默认 `POST`）、`withCredentials`、`getToken`、
`refreshToken`、`onData`、`onStatusChange`、`maxRetries`（默认 5）、`resumeOnVisible`。

重连是**指数退避**：首次 3 秒，每次 ×1.5，封顶 30 秒，最多 5 次。任何一次连接成功都
把退避重置回 3 秒。收到 401 时：调一次 `refreshToken()`；成功则安排重连（新 sid 可能
已写入 cookie）；失败则清本地登录态并停止重连。

:::caution[刷新是单用轮换]
`/auth/refresh` 会作废旧 sid。Bearer 模式的客户端必须在刷新成功后换用新 token，否则
重连会被 `1014` 拒绝，表现为"一直连不上"。
:::

Web 端的用法（`app/web/lib/hooks/use-notification-sse.ts`）：

```ts
const client = new SSEClient({
  url: '/api/v1/notifications/stream',
  method: 'POST',
  withCredentials: true,           // HttpOnly cookie 认证，JS 读不到 token
  refreshToken: () => tryRefreshToken(),
  onData: (data) => {
    if (typeof data.unread === 'number') setUnread(data.unread);
  },
  maxRetries: 5,
});
client.start();   // 组件挂载
client.stop();    // 组件卸载，避免路由切换后残留连接
```

不要用原生 `EventSource`：它不能发 `POST`、不能带 `Authorization`，断线后还会以固定
间隔无限重连，与这里的退避策略冲突。

## 反向代理的 60 秒问题

nginx 默认 `proxy_read_timeout` 是 60 秒，空闲的通知流会在 61 秒左右被切断，浏览器随即
重连，看起来像"每分钟抖一次"。部署里给两条流单独加了正则 location：

```nginx
location ~ "^/api/v1/(notifications|realtime)/stream$" {
    proxy_pass http://backend:8080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering    off;
    proxy_cache        off;
    proxy_read_timeout 3600s;
}
```

`proxy_buffering off` 与超时同等重要：开着缓冲时 nginx 会攒小事件直到缓冲区满，实时
推送变成批量到达。正则 location 优先于 `/api/v1/` 前缀，所以两条流不会继承 60 秒。
有了 15 秒心跳后，这段配置是**兜底**而不是唯一机制。

## 多标签页

服务端不限制同一用户开几条流，也没有跨标签页去重：三个标签页就是三条连接、三份重连
抖动。约定是**每个标签页最多一条**：订阅放在布局层（如通知铃铛）而不是每个页面各建
一条；组件卸载时 `stop()`；标签页切到后台可停流，用 `resumeOnVisible` 或自行监听
`document.visibilitychange` 恢复。真要全站一条连接就用 `BroadcastChannel` 由一个
leader 标签页收流再广播——这属于应用层选择，客户端库不提供。

## 验证

```bash
curl -sN -X POST http://localhost:8080/api/v1/notifications/stream \
  -H "Authorization: Bearer $TOKEN" -H 'Accept: text/event-stream' --max-time 3
```

先应收到一次未读数；再让另一个用户回复你的帖子，流里立刻出现新的 `data: {"unread":1}`；
空闲 15 秒能看到 `: keepalive`。指标侧看
`feedchamber_sse_connections{channel="notification"}` 的连接数，断开即减，见
[可观测性](/operations/observability/)。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 大约 60 秒断一次 | 代理读超时或心跳未生效 | 检查 SSE location 与 `proxy_buffering off` |
| 立刻 401 / 403 | 没带凭证，或权限码缺失 | 确认 cookie / `Authorization` 与 `notification:self` |
| 事件成批到达 | 中间层启用了缓冲 | 关掉 `proxy_buffering` |
| 重连几次后彻底停 | 连续失败到达 `maxRetries` | 检查后端与 NATS，或调大上限 |
| 刷新后一直 401 | Bearer 客户端没换新 sid | 采纳 `/auth/refresh` 的新 token |
