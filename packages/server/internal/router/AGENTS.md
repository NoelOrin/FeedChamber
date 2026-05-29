# Router 层

路由注册，所有 API 端点定义在此文件中。

## 结构

```
/api/v1
├── /health                    GET    (public)
├── /auth
│   ├── /register              POST   (public)
│   ├── /login                 POST   (public)
│   ├── /refresh_token         POST   (public)
│   ├── /reset_password        POST   (public)
│   ├── /logout                POST   (protected)
│   ├── /refresh               POST   (protected)
│   ├── /change_password       POST   (protected)
│   └── /first_change_password POST   (protected)
├── /user
│   ├── /profile               GET    (protected)
│   ├── /profile               PUT    (protected)
│   ├── /list                  GET    (perm:user:read)
│   ├── /{id}                  GET    (perm:user:read)
│   ├── /{id}                  DELETE (perm:user:delete)
│   └── /{id}/role             PUT    (perm:user:update)
├── /role
│   ├── /list                  GET    (perm:role:read)
│   ├── /create                POST   (perm:role:manage)
│   └── /{id}                  DELETE (perm:role:manage)
├── /permission
│   ├── /list                  GET    (perm:role:read)
│   ├── /role                  GET    (perm:role:read)
│   └── /sync                  POST   (perm:role:manage)
├── /categories
│   ├── /                      GET    (protected)
│   ├── /                      POST   (perm:category:create)
│   └── /{id}                  DELETE (perm:category:delete)
├── /tags
│   ├── /                      GET    (protected)
│   ├── /                      POST   (perm:post:create)
│   └── /{id}                  DELETE (perm:post:delete)
├── /posts
│   ├── /                      GET    (protected)
│   ├── /search                GET    (protected)
│   ├── /{id}                  GET    (protected)
│   ├── /                      POST   (perm:post:create)
│   ├── /{id}                  PUT    (perm:post:update)
│   ├── /{id}                  DELETE (perm:post:delete)
│   ├── /{postId}/tags         GET    (protected)
│   ├── /{postId}/tags         POST   (perm:post:create)
│   ├── /{postId}/comments/    GET    (protected)
│   ├── /{postId}/comments/    POST   (perm:comment:create)
│   ├── /{postId}/comments/{id} DELETE (perm:comment:delete)
│   ├── /{postId}/like         POST   (protected)
│   ├── /{postId}/like         GET    (protected)
│   ├── /{postId}/favorite     POST   (protected)
│   └── /{postId}/favorite     GET    (protected)
├── /favorites                 GET    (protected)
├── /comments/{commentId}
│   ├── /like                  POST   (protected)
│   └── /like                  GET    (protected)
└── /users/{userId}
    ├── /follow                POST   (protected)
    ├── /follow                DELETE (protected)
    ├── /follow                GET    (protected)
    ├── /followers             GET    (protected)
    ├── /following             GET    (protected)
    └── /counts                GET    (protected)
```

## 注意事项

- `/user` (单数) = 当前用户操作，`/users/{userId}` (复数) = 他人用户操作 (关注等)
- 路由顺序重要：`/search` 必须在 `/{id}` 之前
- 所有 protected 路由经过 JWTAuth() + BanCheck()
