# Server 应用启动

`Start(env)` 是整个应用的启动主流程。

## 启动顺序

1. 加载 Config
2. 创建 DB Store (SQL)
3. AutoMigrate（注册所有模型）
4. Seed 种子数据（角色/权限/管理员/设置/元标记）
5. 构造 PermissionService + SettingService + BannedService + SpaceService
6. 构造 Container → Handler → Router
7. 注册 Swagger
8. 启动 HTTP Server

## 关键设计

- `seed.go` — 种子数据逻辑：角色、权限、管理员、设置、邮箱模板
- `migrate.go` — 存量库一次性升级：`migrateTrustData`（等级数值化 / `total_xp` 回填）、`migrateSpaceVisibility`（`auth_required` → `visibility`）、`migrateModerationMode`（旧 `post_approval` 布尔 → `moderation_mode`，缺行时 `true`→`manual`、其余→`async`；**必须在 `syncDefaultSettings` 之前**，否则默认值会先补成 async）
- `seed_spaces.go` — 基础版面（8 个一级版面，按 slug 幂等补齐，不覆盖管理员改动）
- `seed_tags.go` — 基础标签库（28 个常用标签，按 name 幂等补齐）
- 版面/标签 seed 在**所有环境**执行，门控为 `!alreadySeeded || 对应表为空`（首次启动补齐、表清空可恢复、被删数据不复活）
- `reseedPostsAndContents` 仅在 `env == Dev` 时执行（H-01 门控），会给 mock 帖子挂标签并重算版面 `post_count`
- 权限 seed 幂等：`alreadySeeded` 元标记 + `roleMappingsEmpty` 双重门控（R-04 修复）
- 存储 Provider 在启动时通过 `pkg.BuildStorageConfig()` 按 settings 表 → env → 默认构建（H-04 修复）
- 启动 KV Store Pub/Sub subscriber 用于多实例权限缓存同步（R-07）

## 文件

| 文件 | 说明 |
|------|------|
| iris.go | `Start(env)` 主流程 |
| seed.go | 种子数据（角色、权限、管理员、设置、内容） |
| seed_spaces.go | 基础版面定义与幂等补齐（`ensureSeedSpaces`） |
| seed_tags.go | 基础标签定义与幂等补齐（`ensureSeedTags`） |
