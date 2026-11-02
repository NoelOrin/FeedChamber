# Tasks

## 1. 死代码与重复实现清理

- [x] 1.1 删除 `middleware.JWTAuth` / `OptionalJWTAuth` / `BanCheck` / `RequireRole` / `RequireOwnerOrPermission`（生产调用数逐个实测为 0），`auth.go` 头注释改为真实分工
- [x] 1.2 删除 `Deps.Blacklist` 字段与 `TokenRevoker` 接口（无赋值点也无读取点）
- [x] 1.3 合并 `internal/middleware/client_ip.go` 与 `pkg/gateway/client_ip.go`（22 行逐字相同）为 `pkg.ClientIP`
- [x] 1.4 删除 `pkg/gateway` 的 `pkgSuccess` 复制品，13 处改调 `pkg.Success`
- [x] 1.5 登出两份逐字相同实现合一：保留有 3 项测试的 `SessionManager.LogoutHandler`，删 `authHandler.Logout`，swag 注解搬到真正挂载的方法
- [x] 1.6 删除 master_access_token 时代的 cookie helpers 与常量、空的 `internal/handler/cookie.go`；`ReadSessionCookie` 改为网关实际调用（此前只有测试在用）
- [x] 1.7 删除 `pkg/jwt.SigningKey`（注释称用于 JWKS，实际 JWKS 走 RSA；暴露 HS256 密钥只是误用面）
- [x] 1.8 删除其余零调用者符号：`Space.IsRoot` / `ComputeDepth` / `FlattenSpaceTree`、`GetSpaceRolePermissionsWithInheritance`、`OAuthManager.ProviderNames`、`MaskedKey`、`CheckInRepository.GetLatestByUser`
- [x] 1.9 用 `/tmp/deadclassify.py` 全仓扫描导出符号引用数，43 个候选逐个判定（区分真死码、GORM/Prometheus 接口实现、仅测试使用、包内自用）

## 2. 响应信封与错误码

- [x] 2.1 新增 `pkg.FailStatus`；`errToHTTPStatus` 补 `5004→504`、`5005→503`
- [x] 2.2 `SUCCESS` 文案由 `"成功"` 改为协议字面量 `"success"`（对齐 `api-style` spec 与 `scripts/stub-api.mjs`；确认前端只在失败分支读 `msg`）
- [x] 2.3 网关 87 处单行 + 6 处多行 `StopWithJSON({"code","message"})` 改 `pkg.Fail`，私有号段（1004/1020/1021/109x/4001/4002/5002/404）映射回共享表，逐条带映射表转换、未映射计数为 0
- [x] 2.4 保留 RFC 形状：token 响应、`oauthError()`、JWKS、Discovery 元数据未改写
- [x] 2.5 `maintenance` 的 `{"code":5030}` → `MAINTENANCE(5005)`；`request_timeout` 的字符串码 → `REQUEST_TIMEOUT(5004)`
- [x] 2.6 网关 banCheck 的 `404 + 空 body` → `FailStatus(404, USER_BANNED)`（状态不变，补上原因）
- [x] 2.7 `packages/api-client` 的 `ErrorCode` 补 `BODY_TOO_LARGE` / `REQUEST_TIMEOUT` / `MAINTENANCE`

## 3. panic 边界

- [x] 3.1 新增 `internal/middleware/recovery.go`，挂在 `Logger` 之后并说明顺序理由
- [x] 3.2 指标 `feedchamber_http_panics_total`（无标签）注册进独立 Registry
- [x] 3.3 3 项 httptest：handler panic、party 级（网关）panic、正常请求不受影响

## 4. 装配层重构

- [x] 4.1 `container` 拆 `deps.go` / `container.go` / `runtime.go`：`New(Deps)` 纯构造，`Start(ctx) (*Runtime, error)` 起后台
- [x] 4.2 `Deps` 具名字段取代 11 个位置参数；`Handler` 与 `Runtime` 分工明确
- [x] 4.3 事务管理器 9 份 → 1 份（`c.Tx`）
- [x] 4.4 `_ = BadgeSvc.SeedDefaults(ctx)` → 由 `Start` 返回错误、`iris.go` Fatal
- [x] 4.5 `ModerationService` 构造函数直接收 bus，删除 `SetEventBus` 类型断言注入
- [x] 4.6 `handler.Deps` 显式列出 38 个服务；新增 `server/wiring.go` 做容器→HTTP 层映射；`handler` 不再 import `container`
- [x] 4.7 `Dashboard` 从 `h.c.XxxSvc` 的 locator 用法改为独立 `homeHandler`（三依赖），`Handler` 不再持有容器
- [x] 4.8 `Handler.Shutdown()` 删除；`iris.go` 关停改调 `runtime.Shutdown()`

## 5. 网关依赖与身份契约

- [x] 5.1 `gateway.Deps.KV` 由 `any` 改为 `kvstore.Store`，删除 6 处运行时断言及其"失败即静默降级"分支
- [x] 5.2 删除 `sessionKV` 与 `kvClient` 两份重复的三方法接口
- [x] 5.3 测试替身 embed `baseKV`（`kvstub_test.go`）；两条"传错类型返回 nil"的测试替换为 `validateState` 的行为测试（无 KV fail-closed、state 命中即消费、提供方必须匹配）
- [x] 5.4 `gateway.Deps` 在 `server/iris.go` 一次构造，复用给 SessionStore / SecurityManager / Gateway
- [x] 5.5 `router.Register` 不再接收 `*gorm.DB`、不再就地装配网关；全仓对注入服务的 unchecked 类型断言降为 0
- [x] 5.6 新增 `internal/pkg/ctxkey.go`（8 个键常量 + 2 个取值常量），55 处生产调用点改常量；测试保留字面量以钉住线格式

## 6. 文档纠偏

- [x] 6.1 `middleware/AGENTS.md` 重写（旧版描述已删除的 JWT 栈与从未存在的 `auth_gate.go`）
- [x] 6.2 `handler/AGENTS.md`：删 `metrics.go`/`cookie.go` 行，补"不在 handler 层的端点"，分页签名与身份来源改正，方法清单降级为"代表方法"
- [x] 6.3 `repository/AGENTS.md`：规模改实测、删 11 个不存在的文件名、写明两种布局并存与网关自持数据访问的例外
- [x] 6.4 `container/AGENTS.md` 重写为 New/Start 两步契约
- [x] 6.5 `router/AGENTS.md`：网关由 composition root 传入、去掉 `OptionalJWTAuth` 旧约定
- [x] 6.6 `docs/ARCHITECTURE.md`：SolidJS→React/vinext、分层图与决策速查补会话/信封/可观察性
- [x] 6.7 根 `AGENTS.md`：结构与中间件段重写、Auth 模块说明改服务端会话模型、错误码分段补 4xxx、启动命令改 `-e dev`
- [x] 6.8 `docs/backend-overview.md` 补 §9.1 可观察性（上一 change 的 10.5）与 §13 的 2026-10-14 条目
- [x] 6.9 `make swag` 补齐 session-auth 遗漏的 `openapi.json` / `docs/` 再生成

## 7. 验证

- [x] 7.1 `go build ./... && go vet ./... && go test -count=1 ./...` 全绿（每步改动后各跑一次）
- [x] 7.2 `feedchamber_test.sh -all` 113/113 通过（信封改动后、装配重构后、身份键改动后各一次）
- [x] 7.3 实机冒烟：成功信封 `msg:"success"`、网关 401/429 带中文原因、维护模式 503+5005、登出清 cookie、API Key 创建与 `X-API-Key` 鉴权、SIGTERM 优雅关停
- [x] 7.4 前端引用面核对：无任何消费者引用被废弃的网关私有码；`.msg` 仅在失败路径读取

## 8. 后续独立提案（不在本 change 范围内，故不列为任务）

- 8.1 `pkg/gateway` 直接持有 `*gorm.DB` 与 `model` 做数据访问（session_store / security / apikey / oauth_provider），且同包混合 `iris.Context` 处理器 —— 属"传输与存储融合"，迁移到 `internal/repository` 需动约 1500 行并设计接口边界，另案提案
- 8.2 repository 层两种文件布局并存（18 个域 `xxx.go`+`xxx_sql.go`、20 个域单文件）未强制统一 —— 只在 `repository/AGENTS.md` 标注为待收敛项
- 8.3 `middleware.Deps` 与 `gateway.Deps` 的同名窄接口（SettingReader / PermissionChecker / BanChecker）保留 —— consumer-side 小接口是 Go 惯例，合并需要一个共享 types 包，等于新增一层，与本项目"修接缝而非加层"的取向相悖
- 8.4 `CtxUserID` 与各表 `user_uuid` 列名仍是 UUID 时代的遗留名（实际存 `User.ID`，ULID）—— 改键名无风险，改列名需迁移，本轮只在 `ctxkey.go` 记录
- 8.5 `AutoMigrate` 与手写 `db/migrations/*.sql` 并存的"迁移来源二元"问题未处理
