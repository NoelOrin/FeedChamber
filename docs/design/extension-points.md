# Feed Chamber 扩展点指南

## 原则

- **官方模块优先**：新增能力以 Go 源码模块形式合入 monorepo。
- **事件驱动解耦**：业务 Service 只 `Publish` 事件；通知 / Webhook / 徽章等订阅者自行处理。
- **不支持第三方热加载**：不提供 `.so` / 脚本沙箱 / 任意插件运行时。扩展请走 PR + 编译发布。

## 事件目录

常量定义：`app/server/internal/pkg/eventbus/eventbus.go` (含 events.go 别名)

| 常量 | 值 | 典型发布方 |
|------|----|------------|
| `EventPostCreated` | `post.created` | PostService |
| `EventPostClosed` | `post.closed` | PostService.SetClosed |
| `EventContentCreated` | `content.created` | ContentService |
| `EventContentLiked` | `content.liked` | LikeService |
| `EventContentReplied` | `content.replied` | ContentService |
| `EventContentWatched` | `content.watched` | WatchService |
| `EventUserRegistered` | `user.registered` | AuthService（`EventUserCreated` 为历史别名） |
| `EventUserFollowed` | `user.followed` | FollowService |
| `EventUserMentioned` | `user.mentioned` | mention helper |
| `EventReportCreated` | `report.created` | ReportService |
| `EventModerationDone` | `moderation.decided` | 审核决策（预留/按需发布） |

订阅方应 **只引用上述常量**，禁止散落字符串字面量。

### 两种订阅语义

- `Subscribe` — 队列订阅（NATS queue group）。**集群内每个事件只被消费一次**，用于通知、Webhook 这类「一次就够」的消费者。
- `SubscribeBroadcast` — 广播订阅（无 queue group）。**集群内每个订阅方都收到**，用于插件钩子这类「多个消费者都要知道」的场景。

两者互不影响：广播订阅不写入 `handlers`/`subs`，队列订阅的语义保持不变。

## 如何新增官方模块

1. **model**（如需持久化）  
2. **repository**（接口 + SQL + Mongo）  
3. **service**（`*_iface.go` + 实现，错误返回 `*pkg.AppError`）  
4. 在 `container.New` 注册依赖，并在需要时 `bus.Subscribe(...)`  
5. **handler + router**（RPC：写操作 `POST` + 动词后缀 `/list|/create|/{id}/update|/{id}/delete`；只读查询 MAY `GET`）  
6. 前端：`packages/api-client` 类型与 service；Web/Dashboard 页面按需  
7. 文档 / seed / 权限码  

### 订阅示例

```go
// 队列订阅：集群内每个事件只会有一个消费者处理
bus.Subscribe(eventbus.EventPostCreated, func(e eventbus.Event) {
    // handle payload
})

// 广播订阅：集群内每个订阅方都会收到（插件钩子用这个）
bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {
    // handle payload
})
```

### 发布示例

```go
bus.Publish(eventbus.Event{
    Type: eventbus.EventPostCreated,
    Ctx:  ctx,
    Payload: map[string]any{"post_id": id},
})
```

## 容器注册表

所有官方模块在 `app/server/internal/container/container.go` 装配：

- Repository 无环依赖先创建  
- Service 按依赖顺序创建  
- EventBus 创建后立刻挂订阅者（Notification / Webhook 等）  
- 后台 ticker（digest / trust promotion / view flush）在 container 启动，handler.Shutdown 取消  

## 反模式

- 在业务层硬编码通知文案发送（应走 EventBus）  
- 为插件引入动态 `plugin.Open`  
- REST 风格业务路由  
- 跨层调用（handler 直访 repository）  
