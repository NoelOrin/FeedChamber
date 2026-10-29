# Pkg 通用工具包

通用工具函数和子包，无业务逻辑。被其他所有层引用。

## 子包与文件

| 文件/目录 | 说明 |
|-----------|------|
| response.go | 统一响应封装 `Success()`, `HandleError()`, `SuccessPaginated()`, `Fail()` |
| errors.go | 错误码系统 `ErrCode`, `AppError`, `errToHTTPStatus` 映射 |
| id.go | ULID 生成（`ulid.Make()`) |
| code.go | 加密安全随机数字验证码生成 `GenerateNumericCode(n)` |
| pagination.go | 分页参数规范化 `NormalizePagination()` / `NormalizePaginationCustom()` |
| settings.go | `BuildStorageConfig()` — 按 settings 表 → env → 默认优先级构建 Storage config |
| kvstore/ | KV Store 抽象层（NATS JetStream） |
| readcache/ | JSON 读穿缓存（KV 之上）：TTL + 前缀失效 + 命中/未命中指标，用于热点只读数据 |
| natsembedded/ | 进程内 nats-server（含 JetStream）启停封装。**仅测试用**（`internal/pkg/testnats`）：生产路径已剥离，见下 |
| ratelimit/ | 滑动窗口限流器 |
| eventbus/ | NATS 事件总线 |
| tracing/ | W3C `traceparent` 解析/生成 + context 传递（无 OTel 依赖） |
| logger/ | 结构化请求日志 |

## kvstore/

Factory `New(cfg)` 创建唯一实现：NATS JetStream KV。
`NATS_URL` 必须是外部 NATS（`config.Validate` 在启动前拦空值）。

生产路径**不再**启动进程内 nats-server（2026-10-28 剥离）：KV 与 EventBus 的生命周期不该绑在应用进程上，否则进程重启即丢 JetStream 队列、多实例也无法共享会话缓存与限流窗口。`natsembedded` 包保留仅供单元测试使用，避免 CI 要多起一个外部服务。

```go
type Store interface {
    Get(ctx, key) (string, error)
    Set(ctx, key, value, ttl) error
    Delete(ctx, key) error
    Exists(ctx, key) (bool, error)
    IncrBy(ctx, key, increment, ttl) (int64, error)
    Publish(ctx, channel, message) error
    Subscribe(ctx, channel) (<-chan []byte, error)
    Keys(ctx, pattern) ([]string, error)
}
```

Pub/Sub 机制：NATS core pub/sub（subject 前缀 `nats.kv.`），跨实例。
业务 key 在写入 JetStream KV 前统一编码为合法 key，对外接口仍使用原始 key。

已接入：会话 read-through 缓存、限流器、通知 Pub/Sub、未读计数缓存、View Count 缓冲、OAuth State、Settings/Ban 缓存、权限多实例同步（Pub/Sub）。OAuth 令牌撤销不再走 KV（改为 `oauth_tokens.revoked` 列）。

## readcache/

读多写少数据的 JSON 读穿缓存，落在 kvstore 之上（`readcache.New(kv, ttl)`）。

- **nil 安全**：`kv == nil` 或 `ttl <= 0` 时返回 nil，nil Cache 的 Get 恒 miss、Put/Invalidate 是空操作——调用方不需要到处判空。
- **分钟级 TTL**：KV 的 per-key TTL 存储精度是秒，亚秒 TTL 会被写成"当前秒"立即过期，`New` 里统一向上取整到秒。
- **key 用哈希**：`Key(prefix, parts...)` 把参数序列化后取 SHA-256 前 32 hex。直接拼参数会把用户输入带进 JetStream key（字符集受限、长度无上限）。
- **前缀失效**：`InvalidatePrefix` 供写路径按类清空（帖子写 → `cache:postlist:`，版面写 → 树/扁平 + 帖子列表）。
- **指标**：`orinforum_readcache_hits_total{name}` / `_misses_total{name}`，`name` 只取固定枚举（`post_list` / `tag_list` / `space_tree` / `space_flat`）。

已接入：匿名帖子列表（10s）、标签列表（60s）、版面树/扁平（60s）。**登录态列表不进共享缓存**——它带已读状态、拉黑排除与等级可见性，跟着人变。

## ratelimit/

滑动窗口限流器，基于 KV Store 实现。中间件用法：`RateLimit(limiter, name, limit, window)`。

## eventbus/

基于 NATS core pub/sub 的分布式事件总线。每个事件类型使用队列订阅，集群内每个事件只消费一次；
消费后仍会分发到当前进程的所有本地 Handler。业务 Service 发布 `EventContentReplied`/`EventContentLiked` 等，NotificationService 订阅后创建通知。

## logger/

结构化请求日志，输出 status/latency_ms/trace_id/span_id/user_uuid/ip/route。请求级实例由 `middleware.Trace` + `middleware.Logger` 写入 `request.Context()`，业务层用 `logger.FromContext(ctx)` 取用；`Config{Level,Format,Output}` 经 `server/iris.go` 的 `logger.SetGlobal(logger.New(cfg.Log))` 生效，`SetGlobal` 同时同步 `logrus.StandardLogger()`，使约 156 处存量裸 `logrus.X()` 调用一并受 `LOG_LEVEL/LOG_FORMAT/LOG_OUTPUT` 管辖（`New` 本身无全局副作用，`NewNop` 不触碰标准 logger）。

## tracing/

自研 W3C Trace Context（`00-{32hex trace}-{16hex span}-{2hex flags}`），不引入 OpenTelemetry/collector。`ParseTraceparent` 严格只收 `00` 版本与小写十六进制，畸形值返回 false 由调用方新生成；`FromTraceID` 用于沿用 nginx `$request_id`（同形同宽）。trace id 只进日志与响应头，MUST NOT 作为指标标签。EventBus 经 `PublishMsg` 消息头传播 trace，消费端在**新的 background ctx** 上重建（不继承发布方取消信号）。
