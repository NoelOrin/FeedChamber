# backend-service-design Specification (delta)

## REMOVED Requirements

### Requirement: Repository 接口与双实现

移除原因：MongoDB 后端已下线，Repository 不再需要按存储后端双实现；`db.Store` 接口移除 `Mongo()`，`BackendMongo` 常量删除。

## ADDED Requirements

### Requirement: Repository 接口与 SQL 实现

每个数据资源 MUST 在 `repository/` 中定义接口；工厂函数 MUST 返回基于 `store.SQL()` 的 SQL（GORM）实现，不再存在 Mongo 分支。Service 层 MUST 仅依赖接口，不感知底层存储。

#### Scenario: 工厂返回 SQL 实现

- WHEN 应用启动并构造 Repository
- THEN `NewXxxRepository(store)` MUST 返回 `sqlXxxRepo{db: store.SQL()}` 实现

#### Scenario: Service 不感知存储

- WHEN Service 调用 Repository
- THEN 它 MUST 仅通过接口方法访问数据，编译期不依赖具体实现类型

### Requirement: 数据库后端取值域

`db.Store` MUST 仅支持 SQL 后端，`DB_TYPE` MUST 限于 `sqlite` / `mysql` / `postgres`；`Store` 接口 MUST 暴露 `SQL()` 供 Repository 获取 GORM 句柄，且 MUST NOT 暴露 `Mongo()`。

#### Scenario: 拒绝已下线的后端

- WHEN 配置 `DB_TYPE=mongo`
- THEN 应用启动 MUST 失败并返回「unknown database backend」错误