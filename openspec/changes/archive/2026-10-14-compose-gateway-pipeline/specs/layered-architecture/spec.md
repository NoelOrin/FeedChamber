# layered-architecture Specification (delta)

## MODIFIED Requirements

### Requirement: 单向分层依赖链

请求处理 MUST 严格遵循单向依赖链：HTTP Request → Gateway（v1 级 `Preflight`：trace → 全局 IP 限流 → 可选凭证解析；protected 级 `Enforce`：必需凭证 → banCheck → 权限 → 安全风控 → 会话跟踪）→ Handler（参数解析、调 service、统一响应封装）→ Service（业务逻辑、返回 `*pkg.AppError`）→ Repository（数据访问接口）→ Database（SQL via GORM）。

各层 MUST 仅依赖下一层接口，不允许跨层调用或反向依赖。凭证解析 MUST 只发生在 `pkg/gateway`；`internal/middleware` 只承担身份确定之后的门控与请求基础设施。

#### Scenario: Handler 仅调用 Service 接口

- WHEN Handler 需要读写数据
- THEN 它 MUST 通过注入的 Service 接口访问，MUST NOT 直接 import `internal/repository` 或 `internal/db`（唯一例外：`/health` 系统探针的基础设施探活）

#### Scenario: 网关不自建业务依赖

- WHEN 网关需要权限/禁言/设置数据
- THEN 它 MUST 经 `Deps` 注入的接口（与 handler 层共用同一实例），MUST NOT 构造第二份 PermissionService 或 BannedService

## ADDED Requirements

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
