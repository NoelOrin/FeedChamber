## Purpose

定义 Feed Chamber 业务模块的分层文件组织约定与新增模块的改动清单，确保新增功能以一致方式落入既有架构。

## ADDED Requirements

### Requirement: 模块分层文件组织

每个业务模块 MUST 在对应层提供文件：`internal/model/xxx.go`（纯数据模型，优先 `NewXxx()` 构造函数）、`internal/repository/xxx.go`（接口 + SQL + Mongo，`WrapDBErr` 统一错误）、`internal/service/xxx_iface.go` + `internal/service/xxx.go`（业务逻辑，返回 `*pkg.AppError`）、`internal/handler/xxx.go`（HTTP handler + swag 注解）。

Service 接口定义 MUST 优先拆分到 `_iface.go` 文件（与实现分离）；派生/工具型服务可不定义接口，但风格 MUST 与主流一致。

#### Scenario: 新增资源模块的文件落位

- WHEN 新增一个业务资源 Xxx
- THEN MUST 在 model/repository/service/handler 各层分别建立对应文件，并在 `handler.go` 的 Handler struct 注册、`router.go` 注册路由、`iris.go` 的 AutoMigrate 注册模型、`seed.go` 补充种子（如需要）

### Requirement: 新增模块改动清单

新增功能 MUST 触发下列改动（后端）：model → repository → service(_iface + 实现) → handler(+swag) → handler.go 注册 → router 路由 → iris.go AutoMigrate → seed → container（若 service 有依赖）。

新增 API 的前端侧 MUST 同步：api-client 的 `types.ts` / `services/xxx.ts` / `constants.ts`（新权限码或设置 key），以及 `app/web` 或 `app/dashboard` 页面组件。

#### Scenario: 前端 service 同步

- WHEN 后端新增受前端调用的接口
- THEN MUST 在 `packages/api-client` 增加对应 service 模块与类型，Dashboard 经 `@/services/backend` adapter 调用

### Requirement: 用户标识统一为 ULID

所有用户标识 MUST 使用 `User.ID`（ULID），不得再使用历史 `UUID` 字段。前端路由参数 `userId` 与 Dashboard 查询参数 MUST 映射为 `User.ID`。

#### Scenario: 用户标识解析

- WHEN 接口接收用户标识输入
- THEN MUST 经统一解析器支持 ID 或历史 UUID 输入，并归一化为 `User.ID`
