# module-conventions Specification (delta)

## MODIFIED Requirements

### Requirement: 模块分层文件组织

每个业务模块 MUST 在对应层提供文件：`internal/model/xxx.go`（纯数据模型，优先 `NewXxx()` 构造函数）、`internal/repository/xxx.go`（接口 + SQL（GORM）实现，`WrapDBErr` 统一错误）、`internal/service/xxx_iface.go` + `internal/service/xxx.go`（业务逻辑，返回 `*pkg.AppError`）、`internal/handler/xxx.go`（HTTP handler + swag 注解）。

Service 接口定义 MUST 优先拆分到 `_iface.go` 文件（与实现分离）；派生/工具型服务可不定义接口，但风格 MUST 与主流一致。

#### Scenario: 新增资源模块的文件落位

- WHEN 新增一个业务资源 Xxx
- THEN MUST 在 model/repository/service/handler 各层分别建立对应文件，并在 `handler.go` 的 Handler struct 注册、`router.go` 注册路由、`iris.go` 的 AutoMigrate 注册模型、`seed.go` 补充种子（如需要）