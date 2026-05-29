# Repository 层

数据访问层，每个资源一个文件，统一接口 + 双实现模式。

## 约定

- 每个文件定义 `XxxRepository` 接口
- 工厂函数 `NewXxxRepository(store db.Store)` 根据后端类型返回实现
- SQL 实现用 `sqlXxxRepo{db db.SQLStore}`，通过 `r.db.DB().WithContext(ctx)` 操作
- Mongo 实现用 `mongoXxxRepo{store db.MongoStore}`，通过 `r.store.Collection("xxx")` 操作
- 所有方法第一个参数为 `context.Context`
- 错误直接返回，由 service 层包装为 AppError

## 文件列表

| 文件 | 接口 | 核心方法 |
|------|------|---------|
| user.go | UserRepository | FindByID/UUID/Email/Username, Create, Update, Delete, List |
| post.go | PostRepository | List, ListWithDetails, FindByID, FindByIDWithDetails, Search, Create, Update, Delete, IncrementViewCount |
| post.go | CategoryRepository | List, Create, Delete |
| post.go | CommentRepository | ListByPost, Create, Delete |
| role.go | RoleRepository | List, GetByName, Create, CreateIfNotExists, Delete |
| permission.go | PermissionRepository | List, GetByCode, CreateIfNotExists, GetRolePermissions, GetAllRolePermissions, SyncRolePermissions |
| tag.go | TagRepository | List, GetByName, Create, CreateIfNotExists, Delete, AddPostTag, RemovePostTags, GetTagsByPostID, GetPostsByTagID |
| like.go | LikeRepository | TogglePostLike, ToggleCommentLike, HasPostLiked, HasCommentLiked, CountPostLikes, CountCommentLikes |
| favorite.go | FavoriteRepository | Toggle, HasFavorited, ListByUser, CountByUser |
| follow.go | FollowRepository | Follow, Unfollow, IsFollowing, GetFollowers, GetFollowing, CountFollowers, CountFollowing |

## 添加新 Repository

1. 定义接口
2. 实现 `sqlXxxRepo` (GORM)
3. 实现 `mongoXxxRepo` (bson.M 查询)
4. 工厂函数判断 `store.SQL() != nil`
