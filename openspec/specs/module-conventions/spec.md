# module-conventions Specification

## Purpose
定义 Feed Chamber 业务模块的分层文件组织约定与新增模块的改动清单，确保新增功能以一致方式落入既有架构。
## Requirements
### Requirement: 模块分层文件组织

每个业务模块 MUST 在对应层提供文件：`internal/model/xxx.go`（纯数据模型，优先 `NewXxx()` 构造函数）、`internal/repository/xxx.go`（接口 + SQL（GORM）实现，`WrapDBErr` 统一错误）、`internal/service/xxx_iface.go` + `internal/service/xxx.go`（业务逻辑，返回 `*pkg.AppError`）、`internal/handler/xxx.go`（HTTP handler + swag 注解）。

Service 接口定义 MUST 优先拆分到 `_iface.go` 文件（与实现分离）；派生/工具型服务可不定义接口，但风格 MUST 与主流一致。

#### Scenario: 新增资源模块的文件落位

- WHEN 新增一个业务资源 Xxx
- THEN MUST 在 model/repository/service/handler 各层分别建立对应文件，并在 `handler.go` 的 Handler struct 注册、`router.go` 注册路由、`iris.go` 的 AutoMigrate 注册模型、`seed.go` 补充种子（如需要）

### Requirement: 新增模块改动清单

新增功能 MUST 触发下列改动（后端，顺序即依赖方向）：

1. `internal/model/xxx.go` — 数据模型（优先 `NewXxx()` 构造函数）
2. `internal/repository/xxx.go`（接口 + 工厂 + 实现，或接口与 `xxx_sql.go` 实现分文件）— 错误统一 `WrapDBErr`
3. `internal/service/xxx_iface.go` + `internal/service/xxx.go` — 业务逻辑，返回 `*pkg.AppError`
4. `internal/handler/xxx.go` — HTTP handler + swag 注解
5. `internal/handler/handler.go` — `Deps` 增加所需服务字段、`Handler` 增加 handler 字段、`New()` 中接线
6. `server/wiring.go` — 把容器里的服务映射进 `handler.Deps`（漏接线是编译错误）
7. `internal/container/container.go` — repo/service 构造与顺序；若该 service 需要后台任务或事件订阅，改 `runtime.go` 的 `Start`，MUST NOT 在 `New` 里起协程
8. `internal/router/router.go` — 路由注册（保持既有注册顺序，固定路径在参数路径之前）
9. `server/iris.go` — `AutoMigrate` 追加模型；`server/seed.go` — 种子数据（如需）
10. `make swag` — 重新生成 `app/server/docs/` 与根 `openapi.json`

新增 API 的前端侧 MUST 同步：api-client 的 `types.ts` / `services/xxx.ts` / `constants.ts`（新权限码、设置 key 或错误码），以及 `app/web` 或 `app/dashboard` 页面组件。

#### Scenario: 新增带后台任务的模块

- WHEN 新模块需要一个 ticker 或 worker
- THEN 启动 MUST 写在 `container.Start`（并登记进 `Runtime` 的具名 cancel 表），MUST NOT 出现在 `container.New` 中

#### Scenario: 前端 service 同步

- WHEN 后端新增受前端调用的接口
- THEN MUST 在 `packages/api-client` 增加对应 service 模块与类型，Dashboard 经 `@/services/backend` adapter 调用

#### Scenario: 文档与端点清单同步

- WHEN 新增或改名一个端点
- THEN MUST 重跑 `make swag`，使 `openapi.json` 与代码中的 swag 注解一致（该约束此前被遗漏过一次：session-auth 改名 `/auth/refresh_token` → `/auth/refresh` 后未再生成）

### Requirement: 用户标识统一为 ULID

所有用户标识 MUST 使用 `User.ID`（ULID），不得再使用历史 `UUID` 字段。前端路由参数 `userId` 与 Dashboard 查询参数 MUST 映射为 `User.ID`。

#### Scenario: 用户标识解析

- WHEN 接口接收用户标识输入
- THEN MUST 经统一解析器支持 ID 或历史 UUID 输入，并归一化为 `User.ID`
