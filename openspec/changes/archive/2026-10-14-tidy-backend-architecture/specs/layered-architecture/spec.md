# layered-architecture Specification (delta)

## MODIFIED Requirements

### Requirement: 各层职责隔离

- 认证（解析用户与机器凭证）MUST 只在 `pkg/gateway` 进行；`internal/middleware` MUST NOT 持有凭证解析代码，只承担身份确定之后的门控（动态登录门控、权限码门控、维护模式）与请求基础设施（trace、日志、限流、超时、体积）。
- Handler 层负责参数解析、调用 service、统一响应封装与 swag 注解；MUST NOT 直接调用 Repository 或持有装配容器。
- Service 层负责业务逻辑与权限/所有权校验，返回 `*pkg.AppError`。
- Repository 层负责数据访问，定义接口并提供 SQL（GORM）实现。
- Model 层为纯数据模型（仅 GORM 标签），不含业务逻辑。
- 跨层传递的身份信息 MUST 使用 `internal/pkg/ctxkey.go` 声明的键常量（`CtxUserID`/`CtxUsername`/`CtxRole`/`CtxSession`/`CtxClaims`/`CtxAuthMethod`/`CtxAPIKeyScopes`/`CtxClientScopes`）与取值常量（`AuthMethodAPIKey`/`AuthMethodOAuthClient`），MUST NOT 在调用点重复字符串字面量——该契约由网关写入、middleware/handler/service 读取，拼错不会编译报错但会表现为"已登录仍被判未登录"。

#### Scenario: 所有权校验在 Service 层实现

- WHEN 更新或删除帖子/评论等资源
- THEN 资源级"作者 OR 有权限"的所有权校验 MUST 在 Service 层进行（R-06b 方案），而非仅依赖路由中间件

#### Scenario: 身份键改名不破坏线上格式

- WHEN 修改某个 ctx 键的字符串值
- THEN 使用常量的生产代码 MUST 一并生效，而测试与外部契约仍以字面量钉住线格式

### Requirement: 依赖装配集中化

所有 repository 与 service 实例 MUST 通过 `container.New(container.Deps)` 集中实例化。`Deps` MUST 为具名字段结构，MUST NOT 使用位置参数列表。

`container.New` MUST 只做构造：MUST NOT 启动 goroutine、MUST NOT 订阅事件、MUST NOT 写数据。后台工作单元与事件订阅 MUST 由 `(*Container).Start(ctx) (*Runtime, error)` 建立，关停 MUST 经 `Runtime.Shutdown()`；构造函数的错误（如 seed 失败）MUST 返回给启动流程而非静默丢弃（`_ = err` 形式被禁止）。

HTTP 层的依赖 MUST 以显式的 `handler.Deps` 服务列表传入，由 composition root（`server/wiring.go`）从容器映射；`internal/handler` MUST NOT import `internal/container`，MUST NOT 把容器当 service locator 使用。后台协程的生命周期 MUST NOT 挂在 HTTP 层类型上。

网关实例 MUST 在 composition root 构造并作为参数交给路由层：`gateway.Deps` 在 `server/iris.go` 一次构造、复用给 `SessionStore` / `SecurityManager` / `Gateway`；`router.Register` MUST NOT 接收 `*gorm.DB` 或就地 `gateway.New(...)`，也 MUST NOT 对注入的服务做 unchecked 类型断言。

`Start(env)` MUST 按 `config → db.Store → AutoMigrate → seed → NATS/KV/EventBus → 共享服务 → container.New → Start → gateway/middleware deps → handler.New → router.Register → Listen` 顺序组装。

#### Scenario: 构造容器不产生副作用

- WHEN 测试调用 `container.New`
- THEN MUST 不启动任何后台协程，也不因未调用 `Start` 而泄漏 goroutine 或写入数据

#### Scenario: 关停可归因

- WHEN 应用收到 SIGINT/SIGTERM
- THEN MUST 经 `Runtime.Shutdown()` 先冲刷浏览量缓冲、再按名称取消每个 worker、最后关闭事件总线与通知分发

#### Scenario: 启动按顺序装配

- WHEN 应用以 dev 或 prod 环境启动
- THEN `Start(env)` MUST 先完成数据库与种子数据，再构造容器与 Handler，最后注册路由并监听

## ADDED Requirements

### Requirement: 请求链路的 panic 边界

应用 MUST 在请求链路上安装 panic 恢复边界（`middleware.Recover()`），位置 MUST 在 `Logger` 之后：Iris 先执行 app 级中间件再执行 per-party 中间件，恢复点若排在 `Logger` 之前，被 panic 的请求不会留下完成日志。

恢复后 MUST 返回标准信封的 `500 / code 5001`；堆栈与 panic 值 MUST 只写入日志（`panic`/`method`/`path`/`stack` 字段），MUST NOT 出现在响应体中。响应已开始写出时 MUST NOT 追加或伪造信封。恢复计数 MUST 暴露为无标签指标 `feedchamber_http_panics_total`（panic 文本与路由为无界取值，不得成为标签）。

#### Scenario: handler panic 不打断连接

- WHEN 业务 handler 或网关中间件 panic
- THEN 客户端 MUST 收到 `500 {"code":5001,…}` 而不是连接被 `net/http` 丢弃，且该请求 MUST 仍产生一条带 `trace_id` 的完成日志

### Requirement: 单向分层依赖链

请求处理 MUST 严格遵循单向依赖链：HTTP Request → Gateway / Middleware（trace → 全局限流 → 会话 sid 或 API Key → banCheck → 权限 → 安全 → 会话跟踪 / 门控）→ Handler（参数解析、调 service、统一响应封装）→ Service（业务逻辑、返回 `*pkg.AppError`）→ Repository（数据访问接口）→ Database（SQL via GORM）。

各层 MUST 仅依赖下一层接口，不允许跨层调用或反向依赖。凭证解析（旧文档写作 "JWT"）MUST 只发生在 `pkg/gateway`；用户侧凭证是服务端会话 id，`internal/middleware` 不再承担认证职责。

#### Scenario: Handler 仅调用 Service 接口

- WHEN Handler 需要读写数据
- THEN 它 MUST 通过注入的 Service 接口访问，MUST NOT 直接 import `internal/repository` 或 `internal/db`（唯一例外：`/health` 系统探针的基础设施探活）

#### Scenario: 网关不自建业务依赖

- WHEN 网关需要权限/禁言/设置数据
- THEN 它 MUST 经 `Deps` 注入的接口（与 handler 层共用同一实例），MUST NOT 构造第二份 PermissionService 或 BannedService

