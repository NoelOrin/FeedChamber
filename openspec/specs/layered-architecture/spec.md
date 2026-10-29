# layered-architecture Specification

## Purpose
定义 Feed Chamber 后端请求处理的单向分层依赖链与各层职责边界，确保各层职责清晰隔离、不允许跨层调用。
## Requirements
### Requirement: 单向分层依赖链

请求处理 MUST 严格遵循单向依赖链：HTTP Request → Gateway（v1 级 `Preflight`：trace → 全局 IP 限流 → 可选凭证解析；protected 级 `Enforce`：必需凭证 → banCheck → 权限 → 安全风控 → 会话跟踪）→ Handler（参数解析、调 service、统一响应封装）→ Service（业务逻辑、返回 `*pkg.AppError`）→ Repository（数据访问接口）→ Database（SQL via GORM）。

各层 MUST 仅依赖下一层接口，不允许跨层调用或反向依赖。凭证解析 MUST 只发生在 `internal/gateway`；`internal/middleware` 只承担身份确定之后的门控与请求基础设施。

#### Scenario: Handler 仅调用 Service 接口

- WHEN Handler 需要读写数据
- THEN 它 MUST 通过注入的 Service 接口访问，MUST NOT 直接 import `internal/repository` 或 `internal/db`（唯一例外：`/health` 系统探针的基础设施探活）

#### Scenario: 网关不自建业务依赖

- WHEN 网关需要权限/禁言/设置数据
- THEN 它 MUST 经 `Deps` 注入的接口（与 handler 层共用同一实例），MUST NOT 构造第二份 PermissionService 或 BannedService

### Requirement: 各层职责隔离

- 认证（解析用户与机器凭证）MUST 只在 `internal/gateway` 进行；`internal/middleware` MUST NOT 持有凭证解析代码，只承担身份确定之后的门控（动态登录门控、权限码门控、维护模式）与请求基础设施（trace、日志、限流、超时、体积）。
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

### Requirement: 关联上下文沿依赖链向下传递

trace 上下文 MUST 在网关/中间件层建立一次，并沿依赖链单向向下传递：中间件把 trace 写入 `request.Context()`，Handler MUST 以 `ctx.Request().Context()` 作为下游 ctx 来源，Service 与 Repository MUST 保持 ctx 为首参并继续向下游传递，不得中途替换。

请求路径上 MUST NOT 出现 `context.Background()` / `context.TODO()`；确需脱离请求生命周期的后台任务（flush、健康检查探活、ticker/worker）MUST 使用显式命名的独立 context，并自行作为新 trace 的起点。当前豁免例：`internal/handler/handler.go:85`（计数刷写）、`:116` 与 `:144`（健康检查超时）——其中健康检查仍 MUST NOT 借用请求 ctx。

#### Scenario: Handler 转发的 ctx 携带 trace

- WHEN Handler 调用 `svc.Method(ctx, ...)`
- THEN 该 ctx 来源 MUST 是 `ctx.Request().Context()`，使 Service 层 `logger.WithContext(ctx)` 能取到 `trace_id`

#### Scenario: Service 不自造 context

- WHEN Service 方法需要向下传递超时
- THEN MUST 基于入参 ctx 派生（`context.WithTimeout(ctx, ...)`），MUST NOT 以 `context.Background()` 新建而丢失 trace

#### Scenario: 后台 worker 自成根

- WHEN `webhook.StartWorker`（`internal/service/webhook.go:259`）或每日信任晋升 ticker 执行一轮任务
- THEN MUST 为该轮创建新的 trace 根，并使其日志与任何 HTTP 请求解耦

#### Scenario: EventBus 消费侧不以 Background ctx 起始

- WHEN NATS 消息进入订阅 handler
- THEN `Event.Ctx` MUST 由消息头恢复的 trace 构造，MUST NOT 保留当前的 `context.Background()` 写法（`internal/pkg/eventbus/eventbus.go:123`）

### Requirement: 请求链路的 panic 边界

应用 MUST 在请求链路上安装 panic 恢复边界（`middleware.Recover()`），位置 MUST 在 `Logger` 之后：Iris 先执行 app 级中间件再执行 per-party 中间件，恢复点若排在 `Logger` 之前，被 panic 的请求不会留下完成日志。

恢复后 MUST 返回标准信封的 `500 / code 5001`；堆栈与 panic 值 MUST 只写入日志（`panic`/`method`/`path`/`stack` 字段），MUST NOT 出现在响应体中。响应已开始写出时 MUST NOT 追加或伪造信封。恢复计数 MUST 暴露为无标签指标 `feedchamber_http_panics_total`（panic 文本与路由为无界取值，不得成为标签）。

#### Scenario: handler panic 不打断连接

- WHEN 业务 handler 或网关中间件 panic
- THEN 客户端 MUST 收到 `500 {"code":5001,…}` 而不是连接被 `net/http` 丢弃，且该请求 MUST 仍产生一条带 `trace_id` 的完成日志

### Requirement: 网关流水线分段可组合且每段恰好执行一次

网关流水线 MUST 拆为可组合段：`Preflight()`（trace + 全局 IP 限流 + 可选凭证解析）与 `Enforce()`（必需凭证 + banCheck + 权限 + 安全风控 + 会话跟踪），并允许 `Handler()` 作为二者组合用于单 party 场景。每段对同一请求 MUST 恰好执行一次（以请求级上下文标记实现幂等），因此挂载位置的差异 MUST NOT 造成二次执行。

同一请求 MUST NOT 两次经过全局限流计数：两次累加会使标称阈值减半，掩盖真实配额。

当 Preflight 已解析出身份时，Enforce MUST NOT 再次查询会话存储（避免每请求两次 `user_sessions` 读取与重复的滑动续期判定）；API Key 凭证 MUST NOT 被 Preflight 视为已解析。

#### Scenario: protected 请求只计数一次

- WHEN 全局限流启用（`rate_limit_enabled=true`）且一个受保护请求依次经过 v1 级 Preflight 与 protected 级 Enforce
- THEN 该 IP 的限流计数器 MUST 只增加 1

#### Scenario: 误挂组合入口仍不重复计数

- WHEN 调用方在已挂 Preflight 的 v1 之下、把组合入口 `Handler()` 挂到 protected party
- THEN 限流与凭证解析 MUST 仍各执行一次，MUST NOT 因重复挂载而放宽或收紧配额

#### Scenario: 幂等不得放过匿名请求

- WHEN 一个受保护请求没有任何有效凭证
- THEN Enforce MUST 拒绝（`401 / code 1001`），MUST NOT 因 Preflight 已运行而视为通过

### Requirement: 包位置按消费者边界选择

后端 Go 包 MUST 位于 `app/server/internal/` 下；`app/server/pkg/` 只允许存放确有模块外 import 方的包。判据是实测的 import 关系，不是"将来可能被复用"的意图——同模块内 `internal/` 与 `pkg/` 的可达范围完全相同，把只被内部消费的包放在 `pkg/` 不带来任何收益，只让 `internal/pkg`（通用工具）与 `pkg`（外部候选）两个名字互相混淆。

网关层据此于 2026-10-14 由 `pkg/gateway` 迁入 `internal/gateway`（迁移时实测消费者只有 `server/iris.go` 与 `internal/router/router.go`）。该迁移只改变"谁能 import"的声明位置，不改变包名、符号、文件划分或运行结构。

本条此前的唯一例外 `app/server/pkg/jwt` 已不复存在：该包于 2026-10-18 随 OAuth2/OIDC access token 的 opaque 化整包删除，其中唯一被规范强制保留的 JWT 用途——OIDC `id_token` 的 RS256 签发与签名密钥管理——已内联进 `internal/gateway/sso.go`。因此 `app/server/pkg/` 目录下 MUST NOT 再存在任何包；将来若确实出现模块外 import 方，再按本条重新申请位置，不以"曾经有过"为据。

#### Scenario: 新包默认落在 internal

- WHEN 新增一个只被本模块消费的包
- THEN MUST 建在 `app/server/internal/` 下，MUST NOT 放入 `app/server/pkg/`

#### Scenario: 放进 pkg/ 需给出消费者证据

- WHEN 某包申请位于 `app/server/pkg/` 下
- THEN MUST 能指出至少一个 `feedchamber` 模块之外的 import 方；指不出时评审 MUST 要求移入 `internal/`

#### Scenario: 现存例外已清空

- WHEN 审查 `app/server/pkg/` 的目录内容
- THEN MUST 为空（或不存在）；任何重新出现在此处的包 MUST 附模块外消费者证据，否则评审 MUST 要求移入 `internal/`
