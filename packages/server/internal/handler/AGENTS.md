# Handler 层

HTTP 接口层，解析请求、调用 service、返回统一 JSON 响应。

## 约定

- 每个文件定义 `XxxHandler` 接口 + 私有实现 struct
- 构造函数 `newXxxHandler(svc XxxService) XxxHandler`
- 分页列表用 `pkg.SuccessPaginated(ctx, data, page, pageSize, total)` 返回
- 非分页响应用 `pkg.Success(ctx, data)` 返回
- 错误统一用 `pkg.HandleError(ctx, err)`
- 参数获取：路径参数 `ctx.Params().Get("id")`，查询参数 `ctx.URLParam("q")`，请求体 `ctx.ReadJSON(&req)`
- 分页参数通过 `readListParams(ctx)` 获取，返回 `ListParams{Page, PageSize}`
- 时间范围通过 `readTimeRange(ctx, p)` 获取
- 当前用户从 JWT context 获取：`ctx.Values().GetString("user_uuid")`
- 每个方法上方必须有 swag 注解 (@Summary, @Description, @Tags, @Param, @Success, @Router, @Security)

## 文件列表

| 文件 | Handler | 方法 |
|------|---------|------|
| handler.go | Handler (聚合) + Health | 所有 handler 的依赖注入 |
| auth.go | AuthHandler | Register, Login, Logout, RefreshToken, GetRefreshToken, ChangePassword, FirstChangePassword, ResetPassword |
| user.go | UserHandler | GetProfile, GetByID, List, Search, UpdateProfile, UpdateRole, Delete |
| post.go | PostHandler | List, GetByID, Create, Update, Delete, Search, TogglePin, ToggleLock |
| content.go | ContentHandler | ListByPost, ListReplies, Create, Update, Delete |
| category.go | CategoryHandler | List, Create, Update, Delete |
| tag.go | TagHandler | List, Create, Update, Delete, GetPostTags, SetPostTags |
| like.go | LikeHandler | ToggleContentLike, GetContentLikeStatus |
| favorite.go | FavoriteHandler | Toggle, GetStatus, ListByUser |
| follow.go | FollowHandler | Follow, Unfollow, GetStatus, GetFollowers, GetFollowing, GetCounts |
| role.go | RoleHandler | List, Create, Delete |
| permission.go | PermissionHandler | ListPermissions, GetRolePermissions, SyncRolePermissions |
| upload.go | UploadHandler | Upload, List, GetFile, Delete |
| notification.go | NotificationHandler | List, CountUnread, MarkRead, MarkAllRead, Delete |
| report.go | ReportHandler | Create, List, UpdateStatus, Delete |
| admin.go | AdminHandler | GetStats |
| setting.go | SettingHandler | Get, GetAll, GetPublic, Set, Delete |

## 添加新 Handler

1. 定义接口 + 私有 struct + 构造函数
2. 实现方法，每个方法加 swag 注解
3. 在 `handler.go` 的 Handler struct 添加字段，`New()` 中注册
