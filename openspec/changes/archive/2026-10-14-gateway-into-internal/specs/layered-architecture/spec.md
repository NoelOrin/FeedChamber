## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: 包位置按消费者边界选择

后端 Go 包 MUST 位于 `app/server/internal/` 下；`app/server/pkg/` 只允许存放确有模块外 import 方的包。判据是实测的 import 关系，不是"将来可能被复用"的意图——同模块内 `internal/` 与 `pkg/` 的可达范围完全相同，把只被内部消费的包放在 `pkg/` 不带来任何收益，只让 `internal/pkg`（通用工具）与 `pkg`（外部候选）两个名字互相混淆。

网关层据此于 2026-10-14 由 `pkg/gateway` 迁入 `internal/gateway`（迁移时实测消费者只有 `server/iris.go` 与 `internal/router/router.go`）。该迁移只改变"谁能 import"的声明位置，不改变包名、符号、文件划分或运行结构。

现存唯一例外：`app/server/pkg/jwt`，其消费者（`internal/gateway`、`internal/pkg/settings.go`、`server/iris.go`）同样全在模块内，按本条 SHOULD 迁入 `internal/`；因尚未确认是否留给外部 OIDC 集成方复用，本轮先保留并在 `docs/ARCHITECTURE.md` 记录。

#### Scenario: 新包默认落在 internal

- WHEN 新增一个只被本模块消费的包
- THEN MUST 建在 `app/server/internal/` 下，MUST NOT 放入 `app/server/pkg/`

#### Scenario: 放进 pkg/ 需给出消费者证据

- WHEN 某包申请位于 `app/server/pkg/` 下
- THEN MUST 能指出至少一个 `feedchamber` 模块之外的 import 方；指不出时评审 MUST 要求移入 `internal/`
