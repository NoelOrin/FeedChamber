# module-conventions Specification (delta)

## MODIFIED Requirements

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
