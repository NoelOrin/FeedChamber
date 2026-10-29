# Container 依赖装配

集中构造所有 repository 与 service。**只做构造**：不启动协程、不订阅事件、不写数据。
后台工作单元由 `Start` 单独负责（runtime.go），因此 `New` 在测试里可以安全调用而不会留下跑在自己生命周期外的 goroutine。

## 文件

| 文件 | 内容 |
|------|------|
| deps.go | `Deps`：`New` 的全部输入（配置、基础设施句柄、启动期预建的共享服务） |
| container.go | `Container` 结构体（字段按字母序）+ `New(Deps) *Container` |
| runtime.go | `Runtime` + `(*Container).Start(ctx) (*Runtime, error)` + `Runtime.Shutdown()` |

## 签名

```go
func New(d Deps) *Container                                  // 纯构造
func (c *Container) Start(ctx context.Context) (*Runtime, error) // 订阅事件 + 起后台
func (r *Runtime) Shutdown()                                 // 冲刷浏览量 → 停 worker → 关总线
```

`Deps` 取代了此前 11 个位置参数的 `New(cfg, store, kv, bus, storage, permSvc, settingSvc, bannedSvc, spaceSvc, sessions, secMgr)` —— 位置参数在 11 个同类型入参下极易错位，具名字段让调用点自证。

## 装配顺序

1. **Repositories** — 无内部依赖，全部 `repository.NewXxxRepository(d.Store)`，顺序无关。
2. **Services** — 必须按依赖顺序：
   - 叶子：Invite、Role、Draft、Plugin、Theme、Leaderboard、Badge、AuditLog、Email、MFA、User、Group、Trust（`UserRepo + TrustRepo + SettingSvc + KV`，KV 用于 `trust:sync` 广播）、Gate（`GateRepo + UserRepo + TrustRepo + KV`）、UserBlock、Favorite、Follow、Upload、OAuth、Tag、Announcement、Webhook、Backup、Revision
   - Trust 的消费方用 setter 破环：`PostSvc.SetTrustService` / `LikeSvc.SetTrustService` / `FavoriteSvc.SetTrustService` / `CheckInSvc.SetTrustService` / `ModerationSvc.SetTrustService`（Content 与 Post 走 Deps.Trust / Deps.Gate 直接注入）。
   - Auth（依赖 User/Credential/Meta/Setting/Banned/Invite/Email/MFA/Sessions/Security/EventBus），构造后立即 `service.BindSystemGroupSync(GroupSvc, UserSvc, AuthSvc)`
   - Notification → PM
   - Moderation 必须先于 Post/Content（两者在落库前调它）
   - Watch 先于 Content（Content 需要 WatchSvc）；签名为 `NewWatchService(WatchRepo, TagRepo, EventBus)`——版面关注移除后不再注入 `PostRepo`
   - Content 依赖 PostSvc 的 `PostView`（评论/回复列表复用帖子可见性判定），因此 **Post 必须先于 Content 构造**；Post 构造后立即 `PostSvc.SetTrustService(TrustSvc)`
3. 事务管理器只建一个（`c.Tx = db.NewTransactionManager(d.Store)`），此前每个需要事务的 service 各自 `NewTransactionManager(store)` 建了 9 份。

## 交给 container 之外的共享权威

`Deps` 里的 `Permission` / `Setting` / `Banned` / `Space` / `Sessions` / `Security` 由 `server/iris.go` 在启动期构造，因为它们同时被 middleware、gateway 与业务层使用——必须只有一份实例（M-25）。`Container` 原样持有并把它们命名为 `PermissionSvc` / `SettingSvc` / `BannedSvc` / `SpaceSvc` / `SessionSvc`，由 `container_test.go` 锁住"复用注入实例、不自建"这条回归。

## Start 里的后台单元

| 名称（用于关停归因） | 来源 |
|------|------|
| `view_count_flusher` | `PostSvc.StartViewCountFlusher` |
| `pin_expiry_checker` | `PostSvc.StartPinExpiryChecker` |
| `webhook_delivery` | `WebhookSvc.StartWorker`（先 `WebhookSvc.Subscribe(EventBus)`） |
| `weekly_digest` | `DigestSvc.StartWeeklyTicker` |
| `trust_recompute` | `TrustSvc.StartDailyRecomputeTicker` |
| `motion_resolver` | `MotionSvc.StartResolveTicker` |

外加通知侧的 `service.SubscribeNotifications(EventBus, NotificationSvc)` 返回的 `NotifDispatcher`，
以及文章审核的异步消费者 `ModerationSvc.SubscribeAsync(EventBus)`（`moderation_mode=async` 的发帖事件 →
`handlePostCreated`，命中转待审）。两者都是「订阅」而非 worker，消费跑在 EventBus 自己的 worker 池里。
另外 `Start` 还会启动两个配置同步订阅：`GateSvc.StartSyncSubscriber`（`gate:sync`）与
`TrustSvc.StartSyncSubscriber`（`trust:sync`），任一个失败都会阻断启动。
`Start` 还会 `BadgeSvc.SeedDefaults(ctx)` 补默认徽章——它写库，所以错误必须返回给调用方（此前是构造函数里的 `_ = ...` 静默吞掉）。

## 与 HTTP 层的关系

`Container` 不再被 handler 引用。`server/wiring.go` 的 `newHandler(c)` 把装配结果逐字段映射成 `handler.Deps`（约 38 个服务 + 仅 `/health` 探针需要的 store/kv/cfg），因此：

- 新增 service 被某个 handler 消费时，需要同时改 `handler.Deps` 与 `server/wiring.go`，编译期就会提醒；
- 「HTTP 层到底依赖哪些服务」可以从一个文件读出，而不是散落在 42 个 handler 文件里的 `h.c.XxxSvc`。

后台 worker 的生命周期同样不归 handler：`Handler.Shutdown()` 已删除，关停由 `server/iris.go` 调 `runtime.Shutdown()`。
