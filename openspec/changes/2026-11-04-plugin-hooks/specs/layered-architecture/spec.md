## ADDED Requirements

### Requirement: 插件子系统不得跨层访问数据

`internal/plugin` MUST 只消费 composition root 注入的服务与接口，MUST NOT 引用 `internal/repository`，
MUST NOT 直接持有 `*gorm.DB`。

插件的进程管理、IPC 与钩子分发 MUST 各自收敛在职责单一的组件内，
钩子调用方 MUST NOT 直接依赖子进程实现细节。

插件经宿主 API 的写入 MUST 与管理员操作走同一业务入口，MUST NOT 另起一条写入路径。

#### Scenario: 插件子系统不引用仓储层

- WHEN 检查 `internal/plugin` 的 import 图
- THEN 该包 MUST NOT 出现 `feedchamber/internal/repository`

#### Scenario: 插件写入复用业务入口

- WHEN 插件经宿主 API 修改一篇帖子的状态
- THEN 该修改 MUST 经由与后台管理相同的业务层入口
- AND MUST 触发与管理员操作一致的副作用（如审计、事件）
