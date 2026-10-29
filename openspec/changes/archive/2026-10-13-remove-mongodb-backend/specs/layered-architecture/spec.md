# layered-architecture Specification (delta)

## MODIFIED Requirements

### Requirement: 单向分层依赖链

请求处理 MUST 严格遵循单向依赖链：HTTP Request → Middleware（JWT / Ban / Permission）→ Handler（解析参数、调 service、封装响应）→ Service（业务逻辑、返回 AppError）→ Repository（数据访问接口）→ Database（SQL via GORM）。

各层 MUST 仅依赖下一层接口，不允许跨层调用或反向依赖。

#### Scenario: Handler 仅调用 Service 接口

- WHEN Handler 处理业务请求
- THEN 它 MUST 调用注入的 Service 接口方法，而非直接访问 Repository 或 Database

#### Scenario: Service 不直接操作数据库驱动

- WHEN Service 执行业务逻辑
- THEN 它 MUST 通过注入的 Repository 接口访问数据，不得直接构造 SQL 查询

### Requirement: 各层职责隔离

- Middleware 层负责认证、封禁检查与权限门控；
- Handler 层负责参数解析、调用 service、统一响应封装与 swag 注解；
- Service 层负责业务逻辑与权限/所有权校验，返回 `*pkg.AppError`；
- Repository 层负责数据访问，定义接口并提供 SQL（GORM）实现；
- Model 层为纯数据模型（仅 GORM 标签），不含业务逻辑。

#### Scenario: 所有权校验在 Service 层实现

- WHEN 更新或删除帖子/评论等资源
- THEN 资源级"作者 OR 有权限"的所有权校验 MUST 在 Service 层进行（R-06b 方案），而非仅依赖路由中间件