# Service 层

业务逻辑层，接收 repository 接口，处理业务规则，返回 `error`（通常是 `*pkg.AppError`）。

## 约定

- 每个文件定义 `XxxService` 接口 + 私有实现 struct
- 构造函数 `NewXxxService(repo XxxRepo) XxxService`
- 错误用 `pkg.NewAppError(pkg.XXX, msg...)` 返回
- 分页逻辑在 service 层处理 (page/pageSize → offset/limit)
- 列表方法返回三元组 `(items, totalItems, error)`，handler 层据此构造分页响应
- UUID 生成在 service 层 (`uuid.NewString()`)
- 密码哈希在 service 层 (`bcrypt.GenerateFromPassword`)

## 文件列表

| 文件 | 服务 | 依赖 |
|------|------|------|
| auth.go | AuthService | UserRepository |
| user.go | UserService | UserRepository |
| forum.go | PostService, CategoryService, ContentService | PostRepository, CategoryRepository, ContentRepository |
| permission.go | PermissionService | PermissionRepository (带线程安全内存缓存) |
| tag.go | TagService | TagRepository |
| like.go | LikeService | LikeRepository |
| favorite.go | FavoriteService | FavoriteRepository |
| follow.go | FollowService | FollowRepository |
| upload.go | UploadService | UploadRepository |
| notification.go | NotificationService | NotificationRepository |
| report.go | ReportService | ReportRepository |
| setting.go | SettingService | SettingRepository |

## PermissionService 特殊说明

使用 `sync.RWMutex` 维护 `map[string]map[string]struct{}` 缓存 (role → permission codes)。
- `LoadCache()` 启动时从 DB 加载
- `HasPermission()` 读锁查询
- `SyncRolePermissions()` 写锁更新
