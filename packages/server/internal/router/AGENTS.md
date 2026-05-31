# Router 层

路由注册，所有 API 端点定义在 `router.go` 中。

## 约定

- 路由顺序重要：固定路径必须在参数路径之前
- 公开端点注册在 `v1` party 上，无需认证
- 受保护端点注册在 `protected` party 上，经过 `JWTAuth()` + `BanCheck()`
- 权限控制通过 `middleware.RequirePermission(model.PermXxx)` 实现
- 帖子/内容列表等公开端点使用 `OptionalJWTAuth()` + `PostAuthGate()`

## API 结构

```
/api/v1
├── /health                        GET  (public)
├── /settings/public               POST (public)
├── /auth
│   ├── /register                  POST (public)
│   ├── /login                     POST (public)
│   ├── /refresh_token             POST (public)
│   ├── /reset_password            POST (public)
│   ├── /logout                    POST (protected)
│   ├── /refresh                   POST (protected)
│   ├── /change_password           POST (protected)
│   └── /first_change_password     POST (protected)
├── /categories
│   ├── /list                      POST (public)
│   ├── /create                    POST (perm:category:create)
│   ├── /{id}/update               POST (perm:category:create)
│   └── /{id}/delete               POST (perm:category:delete)
├── /tags
│   ├── /list                      POST (public)
│   ├── /create                    POST (perm:post:create)
│   ├── /{id}/update               POST (perm:post:create)
│   └── /{id}/delete               POST (perm:post:delete)
├── /posts
│   ├── /list                      POST (optionalJWT, postAuthGate)
│   ├── /search                    POST (optionalJWT, postAuthGate)
│   ├── /{id}                      GET  (optionalJWT, postAuthGate)
│   ├── /{postId}/contents/list    POST (optionalJWT, commentAuthGate)
│   ├── /{postId}/tags             POST (public)
│   ├── /create                    POST (perm:post:create)
│   ├── /{id}/update               POST (perm:post:update)
│   ├── /{id}/delete               POST (perm:post:delete)
│   ├── /{id}/pin                  POST (perm:post:pin)
│   ├── /{id}/lock                 POST (perm:post:lock)
│   ├── /{postId}/tags/set         POST (perm:post:create)
│   ├── /{postId}/contents/create  POST (perm:comment:create)
│   ├── /{postId}/contents/{id}/update POST (perm:comment:create)
│   ├── /{postId}/contents/{id}/delete POST (perm:comment:delete)
│   ├── /{postId}/favorite         POST (protected)
│   └── /{postId}/favorite/status  POST (protected)
├── /favorites
│   └── /list                      POST (protected)
├── /contents/{contentId}
│   ├── /like                      POST (protected)
│   ├── /like/status               POST (protected)
│   └── /replies                   POST (protected)
├── /user
│   ├── /profile                   GET  (protected)
│   ├── /profile/update            POST (protected)
│   ├── /search                    POST (perm:user:read)
│   ├── /list                      POST (perm:user:read)
│   ├── /{id}                      GET  (perm:user:read)
│   ├── /{id}/delete               POST (perm:user:delete)
│   └── /{id}/role/update          POST (perm:user:update)
├── /users/{userId}
│   ├── /follow                    POST (protected)
│   ├── /unfollow                  POST (protected)
│   ├── /follow/status             POST (protected)
│   ├── /followers                 POST (protected)
│   ├── /following                 POST (protected)
│   └── /counts                    POST (protected)
├── /role
│   ├── /list                      POST (perm:role:read)
│   ├── /create                    POST (perm:role:manage)
│   └── /{id}/delete               POST (perm:role:manage)
├── /permission
│   ├── /list                      POST (perm:role:read)
│   ├── /role                      POST (perm:role:read)
│   └── /sync                      POST (perm:role:manage)
├── /upload
│   ├── /                          POST (protected)
│   ├── /list                      POST (protected)
│   ├── /{id}                      GET  (protected)
│   └── /{id}/delete               POST (protected)
├── /notifications
│   ├── /list                      POST (protected)
│   ├── /unread_count              POST (protected)
│   ├── /{id}/read                 POST (protected)
│   ├── /read_all                  POST (protected)
│   └── /{id}/delete               POST (protected)
├── /reports
│   ├── /create                    POST (perm:report:create)
│   ├── /list                      POST (perm:report:manage)
│   ├── /{id}/status               POST (perm:report:manage)
│   └── /{id}/delete               POST (perm:report:manage)
└── /admin
    ├── /stats                     POST (perm:role:manage)
    ├── /settings                  POST (perm:role:manage)
    ├── /settings/{key}            POST (perm:role:manage)
    ├── /settings/{key}/update     POST (perm:role:manage)
    └── /settings/{key}/delete     POST (perm:role:manage)
```

## 注意事项

- 所有列表接口均为 POST，请求体传 `{ page, pageSize, ...filters }`
- 所有列表响应格式：`{ code, msg, data, pagination: { currentPage, pageSize, totalItems, totalPages } }`
