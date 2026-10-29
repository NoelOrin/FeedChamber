# SSE Client 框架无关 SSE 客户端

实时消息推送客户端，通过 Server-Sent Events 接收通知未读数。

## 类

```typescript
class SSEClient {
    constructor(baseURL: string, getToken: () => string | null)
    connect(): void
    disconnect(): void
    onMessage(handler: (data: any) => void): void
    onError(handler: (error: Error) => void): void
}
```

## 特性

- Bearer token 鉴权
- 401 自动刷新 token + 重连
- 指数退避重连（3s → 30s，最多 5 次，FE-10 修复）
- ReadableStream 解析（浏览器原生 fetch API）
- AbortController 取消

## Web 端集成

通过 `packages/web/src/hooks/useNotificationSSE.ts` SolidJS adapter hook 调用。

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。
