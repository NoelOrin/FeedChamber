# module-conventions Specification (delta)

## MODIFIED Requirements

### Requirement: 模块分层文件组织

每个业务模块 MUST 在**所属域**下提供文件，而不是平铺进跨域的层目录：

- 数据模型：`internal/model/xxx.go`（共享内核，纯数据，优先 `NewXxx()` 构造函数）
- 数据访问：`internal/repository/xxx.go`（共享内核，接口 + SQL（GORM）实现，`WrapDBErr` 统一错误）
- 业务逻辑：`internal/<domain>/service/xxx_iface.go` + `internal/<domain>/service/xxx.go`（返回 `*pkg.AppError`）
- HTTP 层：`internal/<domain>/handler/xxx.go`（handler + swag 注解）

`<domain>` MUST 取 `core` / `identity` / `growth` / `engagement` / `content` / `ops` 之一，归属判定见「模块归属域」要求。

Service 接口定义 MUST 优先拆分到 `_iface.go` 文件（与实现分离）；接口 MUST 与它的提供方实现同域，MUST NOT 汇集到跨域的中立接口包。派生/工具型服务可不定义接口，但风格 MUST 与主流一致。

#### Scenario: 新增资源模块的文件落位

- WHEN 新增一个业务资源 Xxx
- THEN MUST 在 model / repository 各建对应文件，并在其所属域的 `service/` 与 `handler/` 建实现与 handler
- AND MUST 在该域的 handler 聚合体注册、在 `router` 的 `routes_<域>.go` 注册路由、在 `iris.go` 的 AutoMigrate 注册模型、`seed.go` 补充种子（如需要）

#### Scenario: 模块不得落回跨域平铺包

- WHEN 新增模块的 service 或 handler 文件
- THEN MUST NOT 把它放进跨域的层目录（迁移完成后不存在 `internal/service/` 与 `internal/handler/`）

### Requirement: 新增模块改动清单

新增功能 MUST 触发下列改动（后端，顺序即依赖方向）：

1. `internal/model/xxx.go` — 数据模型（优先 `NewXxx()` 构造函数）
2. `internal/repository/xxx.go`（接口 + 工厂 + 实现，或接口与 `xxx_sql.go` 实现分文件）— 错误统一 `WrapDBErr`
3. `internal/<domain>/service/xxx_iface.go` + `xxx.go` — 业务逻辑，返回 `*pkg.AppError`
4. `internal/<domain>/handler/xxx.go` — HTTP handler + swag 注解
5. `internal/<domain>/handler/` 的域聚合体 — 增加 handler 字段并在构造函数中接线
6. `server/wiring.go` — 把容器里的服务映射进该域的 handler 聚合体（漏接线是编译错误）
7. `internal/container/container.go` — repo/service 构造与顺序；若该 service 需要后台任务或事件订阅，改 `runtime.go` 的 `Start`，MUST NOT 在 `New` 里起协程
8. `internal/router/routes_<域>.go` — 路由注册（保持既有注册顺序，固定路径在参数路径之前）
9. `server/iris.go` — `AutoMigrate` 追加模型；`server/seed.go` — 种子数据（如需）
10. `make swag` — 重新生成 `app/server/docs/` 与根 `openapi.json`

新增 API 的前端侧 MUST 同步：api-client 的 `types.ts` / `services/xxx.ts` / `constants.ts`（新权限码、设置 key 或错误码），以及 `app/web` 页面组件。

#### Scenario: 新增带后台任务的模块

- WHEN 新模块需要一个 ticker 或 worker
- THEN 启动 MUST 写在 `container.Start`（并登记进 `Runtime` 的具名 cancel 表），MUST NOT 出现在 `container.New` 中

#### Scenario: 前端 service 同步

- WHEN 后端新增受前端调用的接口
- THEN MUST 在 `packages/api-client` 增加对应 service 模块与类型

#### Scenario: 文档与端点清单同步

- WHEN 新增或改名一个端点
- THEN MUST 重跑 `make swag`，使 `openapi.json` 与代码中的 swag 注解一致（该约束此前被遗漏过一次：session-auth 改名 `/auth/refresh_token` → `/auth/refresh` 后未再生成）

## ADDED Requirements

### Requirement: 模块归属域

新增模块 MUST 先判定所属域，判定依据 MUST 是「它依赖谁、谁依赖它」的实测结果，而不是文件名的直觉分类。

归属规则（按优先级）：

1. 只被所有域依赖、自身不依赖任何业务域 → `core`
2. 主体是账号、凭证、身份与授权 → `identity`
3. 主体是用户成长、门槛与等级 → `growth`
4. 主体是用户之间的互动与消息 → `engagement`
5. 主体是内容本身及其组织结构 → `content`
6. 主体是运营、站点配置与运维工具 → `ops`

一个模块 MUST 只归属一个域。当实测依赖导致归属会形成域间环时，MUST 调整归属（通常是下沉到更低序号的域）或把该依赖改为经事件总线传递，MUST NOT 放宽域间单向约束。

#### Scenario: 依赖决定归属

- WHEN 判定一个模块属于 `growth` 还是 `content`
- THEN MUST 以它在依赖图中的实际位置为准：若被 `content` 依赖且不依赖 `content`，归 `growth`
- AND MUST NOT 仅凭「它和内容有关」归入 `content`

#### Scenario: 归属造成环时不得放宽约束

- WHEN 某模块放入目标域会形成域间循环引用
- THEN MUST 改为下沉归属或经事件总线解耦
- AND MUST NOT 在 import 边界测试中为该模块开豁免

### Requirement: 域内 handler 聚合与共享工具

每个域 MUST 提供自己的 handler 聚合体（装配该域 handler 所需的 Service），由 composition root 逐域构造；MUST NOT 用单一跨域巨型聚合体一次性注入全部服务。

跨域共用的 HTTP 辅助（当前用户/角色/会话取值、分页与排序参数解析）MUST 收敛到 `internal/pkg/httpx/`，各域 handler MUST 引用它，MUST NOT 各自复制一份。

#### Scenario: 新增域时装配可读

- WHEN 拆分或新增一个域
- THEN 「该域的 HTTP 层依赖什么」MUST 能在该域聚合体与 `wiring.go` 的对应片段中读完
- AND MUST NOT 需要翻阅一个包含全部服务的跨域结构体

#### Scenario: 共享 HTTP 工具不重复实现

- WHEN 两个域的 handler 都需要解析分页参数
- THEN 两者 MUST 调用 `internal/pkg/httpx/` 中的同一实现
- AND MUST NOT 在各自域内复制该函数
