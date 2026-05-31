# Repository 层

数据访问层，每个资源一个文件，统一接口 + 双实现模式（SQL/MongoDB）。

## 约定

- 每个文件定义 `XxxRepository` 接口
- 工厂函数 `NewXxxRepository(store db.Store)` 根据后端类型返回实现
- SQL 实现用 `sqlXxxRepo{db db.SQLStore}`，通过 `r.db.DB().WithContext(ctx)` 操作
- Mongo 实现用 `mongoXxxRepo{store db.MongoStore}`，通过 `r.store.Collection("xxx")` 操作
- 所有方法第一个参数为 `context.Context`
- 列表接口统一签名为 `List(ctx, offset, limit) (items, error)`
- 计数接口统一签名为 `Count(ctx) (int, error)` 或 `CountXxx(ctx, filters...) (int, error)`
- 错误直接返回，由 service 层包装为 AppError

## 文件列表

| 文件 | 接口 | 核心方法 |
|------|------|---------|
| user.go | UserRepository | FindByID, FindByUUID, FindByEmail, FindByUsername, Create, Update, Delete, List |
| post.go | PostRepository | List, ListWithDetails, CountWithDetails, FindByID, FindByIDWithDetails, Search, CountSearch, Create, Update, Delete, IncrementViewCount, IncrementCommentCount, DecrementCommentCount |
| post.go | CategoryRepository | List, Count, FindByID, Create, Update, Delete |
| post.go | ContentRepository | ListByRoot, CountByRoot, ListByParent, CountByParent, FindByID, Create, Update, Delete |
| tag.go | TagRepository | List, Count, FindByID, GetByName, Create, CreateIfNotExists, Update, Delete, AddPostTag, RemovePostTags, GetTagsByPostID, GetPostsByTagID |
| role.go | RoleRepository | List, GetByName, Create, CreateIfNotExists, Delete |
| permission.go | PermissionRepository | List, GetByCode, CreateIfNotExists, GetRolePermissions, GetAllRolePermissions, SyncRolePermissions |
| like.go | LikeRepository | ToggleContentLike, HasContentLiked, CountContentLikes |
| favorite.go | FavoriteRepository | Toggle, HasFavorited, ListByUser, CountByUser |
| follow.go | FollowRepository | Follow, Unfollow, IsFollowing, GetFollowers, GetFollowing, CountFollowers, CountFollowing |
| upload.go | UploadRepository | Create, FindByID, ListByUser, Delete |
| notification.go | NotificationRepository | Create, ListByUser, CountUnread, MarkRead, MarkAllRead, Delete |
| report.go | ReportRepository | Create, List, FindByID, UpdateStatus, Delete |
| setting.go | SettingRepository | Get, GetAll, Set, Delete |

## 添加新 Repository

1. 定义接口（列表方法需同时定义 `List` + `Count`）
2. 实现 `sqlXxxRepo` (GORM)
3. 实现 `mongoXxxRepo` (bson.M 查询)
4. 工厂函数判断 `store.SQL() != nil`
