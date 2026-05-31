# Model 层

纯数据结构体定义，无业务逻辑。

## 约定

- 每个文件定义一个或多个相关的结构体
- SQL 标签用 `gorm:"..."`，MongoDB 标签用 `bson:"..."`
- JSON 序列化用 `json:"..."`，敏感字段加 `json:"-"` (如 Password)
- 自定义表名通过 `TableName() string` 方法
- 种子数据用包级变量 (如 `DefaultRoles`, `DefaultPermissions`)
- 缓存用包级 map + `LoadXxxCache()` 函数 (如 roleCache)
- DTO/ListItem 结构体用于列表查询的扁平化输出

## 文件列表

| 文件 | 模型 | 说明 |
|------|------|------|
| user.go | User | 用户，含 UUID Hook |
| post.go | Post, PostListItem | 帖子实体 + 帖子列表 DTO (含作者/分类名/统计) |
| post.go | Category | 分类 |
| post.go | Content, ContentListItem | 内容实体 + 内容列表 DTO |
| role.go | Role | 角色 + 进程级缓存 |
| permission.go | Permission, RolePermission | 权限定义 + 角色-权限关联 |
| tag.go | Tag, PostTag | 标签 + 帖子-标签多对多 |
| like.go | ContentLike | 内容点赞记录 |
| favorite.go | PostFavorite | 收藏记录 |
| follow.go | UserFollow | 关注关系 |
| upload.go | Upload | 上传文件记录 |
| notification.go | Notification | 通知 |
| report.go | Report | 举报 |
| setting.go | Setting | 系统设置 (key-value) |
