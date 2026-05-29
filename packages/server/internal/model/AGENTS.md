# Model 层

纯数据结构体定义，无业务逻辑。

## 约定

- 每个文件定义一个或多个相关的结构体
- SQL 标签用 `gorm:"..."`，MongoDB 标签用 `bson:"..."`
- JSON 序列化用 `json:"..."`，敏感字段加 `json:"-"` (如 Password)
- 自定义表名通过 `TableName() string` 方法
- 种子数据用包级变量 (如 `DefaultRoles`, `DefaultPermissions`)
- 缓存用包级 map + `LoadXxxCache()` 函数 (如 roleCache)

## 文件列表

| 文件 | 模型 | 说明 |
|------|------|------|
| user.go | User | 用户，含 UUID Hook |
| post.go | Post, Category, Comment | 帖子/分类/评论 |
| post.go | PostListItem | 帖子列表 DTO (含作者/评论数/分类名) |
| role.go | Role | 角色 + 进程级缓存 |
| permission.go | Permission, RolePermission | 权限定义 + 角色-权限关联 |
| tag.go | Tag, PostTag | 标签 + 帖子-标签多对多 |
| like.go | PostLike, CommentLike | 点赞记录 |
| favorite.go | PostFavorite | 收藏记录 |
| follow.go | UserFollow | 关注关系 |
