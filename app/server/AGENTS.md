# Server 应用启动

`Start(env)` 是整个应用的启动主流程。

## 文件

| 文件 | 说明 |
|------|------|
| main.go | 入口，chdir 到自身目录后调用 cmd.RootCmd.Execute()；含 swag 注解 |
| cmd/root.go | Cobra 根命令，持久标志 `-e/--env`（子命令：`backup` / `import-discourse` / `recompute-xp`） |
| server/iris.go | `Start(env)` — 应用启动主流程 |
| server/seed.go | 种子数据（角色、权限、管理员、设置） |
| server/migrate.go | 存量库升级：`migrateTrustData`（等级数值化 / `total_xp` 回填）+ `seedTrustDefaults`（XP 规则 / 等级 / 连续签到阶梯） |
| db/migrations/*.sql | 手写迁移/清理脚本，**没有自动执行器**（与 AutoMigrate 并存）：由人按需手动跑。每个脚本自带「背景 / 是否可选 / 幂等性 / 兼容性」四段注释，见 `cleanup_space_watches.sql` |

## 启动顺序

1. 加载 Config
2. 创建 DB Store (SQL)
3. AutoMigrate（注册所有模型）
4. `migrateTrustData` + `migrateSpaceVisibility`（存量库：`trust_level` 字符串 → 数值、`total_points` → `total_xp`、`auth_required` → `visibility`；删除死列 `users.reputation` / `spaces.post_count`。必须早于任何读取 `users` 的种子步骤）
5. Seed 种子数据（角色/权限/管理员/设置/元标记）；`seedTrustDefaults` / `seedGateDefaults` 只在对应配置表为空时写入
6. 构造 PermissionService + SettingService + BannedService + SpaceService
7. 构造 Container → Handler → Router
8. 注册 Swagger
9. 启动 HTTP Server

## 关键设计

- `seed.go` — 种子数据逻辑：角色、权限、管理员、设置、邮箱模板
- `reseedPostsAndContents` 仅在 `env == Dev` 时执行（H-01 门控）
- 权限 seed 幂等：`alreadySeeded` 元标记 + `roleMappingsEmpty` 双重门控（R-04 修复）
- 存储 Provider 在启动时通过 `pkg.BuildStorageConfig()` 按 settings 表 → env → 默认构建（H-04 修复）
- 启动 KV Store Pub/Sub subscriber 用于多实例权限缓存同步（R-07）
