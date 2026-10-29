## Purpose

定义 Feed Chamber 后端请求处理的单向分层依赖链与各层职责边界，确保各层职责清晰隔离、不允许跨层调用。

## ADDED Requirements

### Requirement: 单向分层依赖链

请求处理 MUST 严格遵循单向依赖链：HTTP Request → Middleware（JWT / Ban / Permission）→ Handler（解析参数、调 service、封装响应）→ Service（业务逻辑、返回 AppError）→ Repository（数据访问接口）→ Database（SQL via GORM / Mongo via 官方驱动）。

各层 MUST 仅依赖下一层接口，不允许跨层调用或反向依赖。

#### Scenario: Handler 仅调用 Service 接口

- WHEN Handler 处理业务请求
- THEN 它 MUST 调用注入的 Service 接口方法，而非直接访问 Repository 或 Database

#### Scenario: Service 不直接操作数据库驱动

- WHEN Service 执行业务逻辑
- THEN 它 MUST 通过注入的 Repository 接口访问数据，不得直接构造 SQL 或 Mongo 查询

### Requirement: 各层职责隔离

- Middleware 层负责认证、封禁检查与权限门控；
- Handler 层负责参数解析、调用 service、统一响应封装与 swag 注解；
- Service 层负责业务逻辑与权限/所有权校验，返回 `*pkg.AppError`；
- Repository 层负责数据访问，定义接口并提供 SQL/Mongo 双实现；
- Model 层为纯数据模型（GORM + BSON 标签），不含业务逻辑。

#### Scenario: 所有权校验在 Service 层实现

- WHEN 更新或删除帖子/评论等资源
- THEN 资源级"作者 OR 有权限"的所有权校验 MUST 在 Service 层进行（R-06b 方案），而非仅依赖路由中间件

### Requirement: 依赖装配集中化

所有 repository 与 service 实例 MUST 通过 `container.New()` 集中实例化，并在 `handler.New(deps)` 中注入到 Handler 聚合体；应用启动主流程 `Start(env)` MUST 按 `config → db.Store → AutoMigrate → seed → PermissionService → SettingService → BannedService → Handler → Router → Listen` 顺序组装。

#### Scenario: 启动按顺序装配

- WHEN 应用以 dev/prod 模式启动
- THEN `Start(env)` MUST 先完成数据库与种子数据，再构造服务与 Handler，最后注册路由并监听
