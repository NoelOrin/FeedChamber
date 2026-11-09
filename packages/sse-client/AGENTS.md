# SSE Client 框架无关 SSE 客户端

框架无关的 SSE 订阅客户端，通过 Server-Sent Events 接收通知未读数。不认识 React / vinext，宿主经 `app/web/lib/hooks/use-notification-sse.ts` 包一层。

## API

`new SSEClient(options: SSEClientOptions)`，用 `start()` / `stop()` 控制生命周期，`isConnected()` 查状态。

`SSEClientOptions` 关键字段：
- `url` / `method`（默认 `POST`）
- `getToken` + `refreshToken`：Bearer 模式；`withCredentials: true` 时改用 HttpOnly cookie，不注入 Authorization 头
- `onData(data)` / `onError({ code, msg })` / `onStatusChange(status)` 回调
- `maxRetries`（默认 5）、`resumeOnVisible`

## 特性

- Bearer token 或 HttpOnly cookie 两种鉴权模式
- 401 自动刷新 token + 重连；刷新失败清 token 并停流
- 指数退避重连（3s → 30s，最多 5 次，FE-10 修复）
- ReadableStream 解析（浏览器原生 fetch API），只认 `data: ` 行；服务端的 `: keepalive` 注释行被忽略
- AbortController 取消；`visibilitychange` 可恢复连接
- **非 OK 响应**：把统一信封里的 `code` 经 `onError` 上报。所有正业务码都要上报（含 5xxx，如维护期 `MAINTENANCE=5005`）——曾用 `code < 5000` 过滤，导致维护期通知铃被 5005 静默掐断、无任何提示。

## 测试

`pnpm --filter @feedchamber/sse-client test`（vitest）。`src/client.test.ts` 钉住 5xxx/4xxx 错误信封都会触发 `onError`。

## Web 端集成

`app/web/lib/hooks/use-notification-sse.ts` 用 `withCredentials: true` 订阅 `/api/v1/notifications/stream`，只在登录态 `start()`，组件卸载时 `stop()`。

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。
