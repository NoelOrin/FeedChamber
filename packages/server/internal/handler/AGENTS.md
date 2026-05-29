# Handler 层

HTTP 接口层，解析请求、调用 service、返回统一 JSON 响应。

## 约定

- 每个文件定义 `XxxHandler` 接口 + 私有实现 struct
- 构造函数 `newXxxHandler(svc XxxService) XxxHandler`
- 响应统一用 `pkg.Success(ctx, data)` 和 `pkg.HandleError(ctx, err)`
- 参数获取：路径参数 `ctx.Params().Get("id")`，查询参数 `ctx.URLParam("q")`，请求体 `ctx.ReadJSON(&req)`
- 当前用户从 JWT context 获取：`ctx.Values().GetString("user_uuid")`
- 每个方法上方必须有 swag 注解 (@Summary, @Description, @Tags, @Param, @Success, @Router, @Security)

## 文件列表

| 文件 | Handler | 方法数 |
|------|---------|--------|
| handler.go | Handler (聚合) + Health | 所有 handler 的依赖注入 |
| auth.go | AuthHandler | 8: Register, Login, Logout, RefreshToken, GetRefreshToken, ChangePassword, FirstChangePassword, ResetPassword |
| user.go | UserHandler | 6: GetProfile, GetByID, List, UpdateProfile, UpdateRole, Delete |
| post.go | PostHandler | 6: List, GetByID, Create, Update, Delete, Search |
| comment.go | CommentHandler | 3: ListByPost, Create, Delete |
| category.go | CategoryHandler | 3: List, Create, Delete |
| tag.go | TagHandler | 5: List, Create, Delete, GetPostTags, SetPostTags |
| like.go | LikeHandler | 4: TogglePostLike, ToggleCommentLike, GetPostLikeStatus, GetCommentLikeStatus |
| favorite.go | FavoriteHandler | 3: Toggle, GetStatus, ListByUser |
| follow.go | FollowHandler | 6: Follow, Unfollow, GetStatus, GetFollowers, GetFollowing, GetCounts |
| role.go | RoleHandler | 3: List, Create, Delete |
| permission.go | PermissionHandler | 3: ListPermissions, GetRolePermissions, SyncRolePermissions |

## 添加新 Handler

1. 定义接口 + 私有 struct + 构造函数
2. 实现方法，每个方法加 swag 注解
3. 在 `handler.go` 的 Handler struct 添加字段，`New()` 中注册
