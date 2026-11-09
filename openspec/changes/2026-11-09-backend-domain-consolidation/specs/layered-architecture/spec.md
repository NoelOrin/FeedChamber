# layered-architecture Specification (delta)

## MODIFIED Requirements

### Requirement: 单向分层依赖链

请求处理 MUST 严格遵循单向依赖链：HTTP Request → Gateway（v1 级 `Preflight`：trace → 全局 IP 限流 → 可选凭证解析；protected 级 `Enforce`：必需凭证 → banCheck → 权限 → 安全风控 → 会话跟踪）→ Handler（参数解析、调 service、统一响应封装）→ Service（业务逻辑、返回 `*pkg.AppError`）→ Repository（数据访问接口）→ Database（SQL via GORM）。

各层 MUST 仅依赖下一层接口，不允许跨层调用或反向依赖。凭证解析 MUST 只发生在 `internal/gateway`；`internal/middleware` 只承担身份确定之后的门控与请求基础设施。

代码组织 MUST 同时满足两个正交的维度：**领域**（业务归属）与**层**（技术职责）。业务代码 MUST 落在 `app/server/internal/<domain>/` 下，域内再按 `service/` 与 `handler/` 分层；MUST NOT 把所有域的同类文件平铺进单一 `internal/service` 或 `internal/handler` 包。

域集合为：`core`（T1）、`identity`（T2）、`growth`（T3）、`engagement`（T4）、`content`（T5）、`ops`（T6）。域与模块的对应关系 MUST 在 `docs/ARCHITECTURE.md` 与 `docs/backend-overview.md` 中可见。

`internal/model` 与 `internal/repository` 是**共享内核**，MUST 保持扁平：它们是全站共用的数据词汇表与数据访问接口，MUST NOT 按域拆分，也 MUST NOT 承载业务判断。跨层传递的身份信息 MUST 使用 `internal/pkg/ctxkey.go` 声明的键常量（`CtxUserID`/`CtxUsername`/`CtxRole`/`CtxSession`/`CtxClaims`/`CtxAuthMethod`/`CtxAPIKeyScopes`/`CtxClientScopes`）与取值常量（`AuthMethodAPIKey`/`AuthMethodOAuthClient`），MUST NOT 在调用点重复字符串字面量——该契约由网关写入、middleware/handler/service 读取，拼错不会编译报错但会表现为"已登录仍被判未登录"。

#### Scenario: Handler 仅调用 Service 接口

- WHEN Handler 需要读写数据
- THEN 它 MUST 通过注入的 Service 接口访问，MUST NOT 直接 import `internal/repository` 或 `internal/db`（唯一例外：`/health` 系统探针的基础设施探活）

#### Scenario: 网关不自建业务依赖

- WHEN 网关需要权限/禁言/设置数据
- THEN 它 MUST 经 `Deps` 注入的接口（与 handler 层共用同一实例），MUST NOT 构造第二份 PermissionService 或 BannedService

#### Scenario: 域内分层不被扁平化

- WHEN 新增或迁移一个业务模块
- THEN 它的 service 代码 MUST 位于 `internal/<domain>/service/`，handler 代码 MUST 位于 `internal/<domain>/handler/`
- AND MUST NOT 继续落在 `internal/service/` 或 `internal/handler/` 这两个跨域平铺包中

#### Scenario: 共享内核不按域拆分

- WHEN 某个域需要 `User`、`Post` 或 `UserRepository`
- THEN 它 MUST 从 `internal/model` / `internal/repository` 引用
- AND MUST NOT 为把类型挪进本域而复制该类型，也 MUST NOT 让 `model`/`repository` 反向 import 任何域

### Requirement: 依赖装配集中化

所有 repository 与 service 实例 MUST 通过 `container.New(container.Deps)` 集中实例化。`Deps` MUST 为具名字段结构，MUST NOT 使用位置参数列表。

`container.New` MUST 只做构造：MUST NOT 启动 goroutine、MUST NOT 订阅事件、MUST NOT 写数据。后台工作单元与事件订阅 MUST 由 `(*Container).Start(ctx) (*Runtime, error)` 建立，关停 MUST 经 `Runtime.Shutdown()`；构造函数的错误（如 seed 失败）MUST 返回给启动流程而非静默丢弃（`_ = err` 形式被禁止）。

HTTP 层的依赖 MUST 以显式的 `handler.Deps` 服务列表传入，由 composition root（`server/wiring.go`）从容器映射；`internal/handler` MUST NOT import `internal/container`，MUST NOT 把容器当 service locator 使用。各域的 handler 聚合体 MUST 在 `wiring.go` 中逐域构造，MUST NOT 通过单一巨型聚合体一次性注入全部服务。后台协程的生命周期 MUST NOT 挂在 HTTP 层类型上。

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

### Requirement: 域间依赖方向由编译期边界强制

域间引用 MUST 单向，顺序为 `core → identity → growth → engagement → content → ops`：低序号域 MUST NOT import 高序号域。

跨域协作 MUST 经提供方域导出的 Service 接口，MUST NOT 直接持有对方域的 repository，MUST NOT 复制对方域的类型到本域。

该约束 MUST 由静态 import 边界测试强制执行，测试失败 MUST 输出「违规文件 → 被禁包 → 规则名」。仅靠文档或约定说明不算满足本条。

约束的范围 MUST 排除技术底座（`internal/pkg`、`config`、`db`、`storage`、`mailer`、`metrics`）、共享内核（`model`、`repository`）、网关与插件子系统（`gateway`、`middleware`、`plugin`、`mcp`），它们可被任意域引用。

#### Scenario: 反向依赖被拒绝

- WHEN `internal/growth/service` 中的一个文件 import 了 `internal/content/service`
- THEN import 边界测试 MUST 失败并指出该文件违反了 `growth → content` 的反向引用

#### Scenario: 域内 service 不反向依赖本域 handler

- WHEN `internal/<domain>/service` 中的文件 import 了 `internal/<domain>/handler`
- THEN import 边界测试 MUST 失败

#### Scenario: 跨域直连仓储被拒绝

- WHEN 某域需要另一个域的实体数据
- THEN 它 MUST 调用对方域的 Service 接口，MUST NOT 直接注入对方域的 repository 实现

### Requirement: 领域重组的搬迁保持不变性

按领域重组目录属于结构性变更，MUST 满足下列全部不变量；任一项不满足即视为搬运引入了语义变更，MUST 回退该提交而非修改判据。

- 端点集合 MUST 不变：迁移前后 `openapi.json` 的 sha256 MUST 相等。
- 路由注册数 MUST 不变。
- 生产代码行数净变化 MUST 为 0（仅允许 package 声明与 import 行变化）。
- 数据库 schema 与迁移 MUST 不变。
- 权限码集合与默认角色映射 MUST 不变。

搬迁 MUST 以「一次一个域」提交，MUST NOT 出现跨域的大批量单次提交；任一中间状态 MUST 通过 `go build ./...`。

#### Scenario: 搬迁保持 openapi 一致

- WHEN 一个域的搬迁提交合入
- THEN `make swag` 产出的根 `openapi.json` MUST 与搬迁前字节级一致

#### Scenario: 搬迁不改变行为

- WHEN 在全新 SQLite 库上启动并运行全量 API 脚本
- THEN 通过项 MUST 与搬迁前的基线相同，MUST NOT 因目录调整而出现新增失败

#### Scenario: 中间状态可编译

- WHEN 迁完 `growth` 域但尚未迁 `content` 域
- THEN `go build ./...` 与 `go vet ./...` MUST 通过
