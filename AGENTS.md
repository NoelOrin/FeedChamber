# OrinForum

Go + SolidJS 社区论坛 monorepo。后端 Iris v12，前端 SolidJS + TanStack Router。

## 架构

```
Request → Middleware (JWT/Ban/Permission) → Handler → Service → Repository → Database
```

每层职责：
- **model/** — 纯数据结构体，GORM + BSON 标签，无业务逻辑
- **repository/** — 数据访问，每个资源一个文件，接口 + SQL实现 + Mongo实现
- **service/** — 业务逻辑，接收 repository 接口，返回 error（pkg.AppError）
- **handler/** — HTTP 层，解析请求、调用 service、用 pkg.Success/HandleError 返回
- **middleware/** — JWT 认证、权限校验、封禁检查
- **router/** — 路由注册，所有端点定义在此

## 关键约定

### Repository 双实现模式
每个 Repository 定义接口，工厂函数根据 db.Store 类型返回 SQL 或 Mongo 实现：
```go
type XxxRepository interface { ... }
func NewXxxRepository(store db.Store) XxxRepository {
    if store.SQL() != nil { return &sqlXxxRepo{db: store.SQL()} }
    return &mongoXxxRepo{store: store.Mongo()}
}
```

### 统一响应格式
所有 handler 使用 `pkg.Success(ctx, data)` 和 `pkg.HandleError(ctx, err)`：
```json
{ "code": 0, "msg": "success", "data": {...} }
```
错误码定义在 `internal/pkg/errors.go`。

### 权限系统
- 13 个权限码定义在 `model/permission.go`
- 启动时 seed 默认权限和角色-权限映射
- 路由使用 `middleware.RequirePermission(model.PermXxx)` 保护
- 权限检查通过 `permissionChecker` 接口注入 middleware

### 新增模块 checklist
添加新功能需改动的文件：
1. `internal/model/xxx.go` — 数据模型
2. `internal/repository/xxx.go` — 接口 + SQL + Mongo
3. `internal/service/xxx.go` — 业务逻辑
4. `internal/handler/xxx.go` — HTTP handler + swag 注解
5. `internal/handler/handler.go` — Handler struct 添加字段，New() 中注册
6. `internal/router/router.go` — 添加路由
7. `server/iris.go` — AutoMigrate 添加模型

## 模块说明

### Auth — 认证 (`handler/auth.go`, `service/auth.go`)
注册、登录、JWT 令牌管理、密码变更。登录返回 access_token (24h) + refresh_token (7d)。

### User — 用户 (`handler/user.go`, `service/user.go`, `repository/user.go`)
用户 CRUD、资料编辑 (username/avatar/bio)、角色变更。密码字段 JSON 序列化时隐藏。

### Post — 帖子 (`handler/post.go`, `service/forum.go`, `repository/post.go`)
帖子 CRUD、搜索、浏览计数。列表/详情返回 PostListItem DTO（含作者、评论数、分类名）。

### Comment — 评论 (`handler/comment.go`, `service/forum.go`, `repository/post.go`)
帖子评论 CRUD，支持 parentId 嵌套。

### Category — 分类 (`handler/category.go`, `service/forum.go`, `repository/post.go`)
论坛分类 CRUD。

### Tag — 标签 (`model/tag.go`, `handler/tag.go`, `service/tag.go`, `repository/tag.go`)
标签系统，帖子多对多关联。通过 SetPostTags 按名称批量设置标签（不存在则自动创建）。

### Like — 点赞 (`model/like.go`, `handler/like.go`, `service/like.go`, `repository/like.go`)
帖子/评论点赞 toggle 模式：已赞则取消，未赞则点赞。

### Favorite — 收藏 (`model/favorite.go`, `handler/favorite.go`, `service/favorite.go`, `repository/favorite.go`)
帖子收藏 toggle 模式，支持按用户列出收藏列表。

### Follow — 关注 (`model/follow.go`, `handler/follow.go`, `service/follow.go`, `repository/follow.go`)
用户关注系统，路由用 `/users/{userId}` (复数)，与 `/user` (单数) 区分。禁止自关注。

### Role — 角色 (`handler/role.go`, `repository/role.go`)
角色 CRUD + 进程级缓存。默认角色：admin、user、ban。

### Permission — 权限 (`model/permission.go`, `handler/permission.go`, `service/permission.go`, `repository/permission.go`)
权限定义 + 角色-权限映射。PermissionService 维护内存缓存，启动时从 DB 加载。

### Middleware (`middleware/auth.go`)
- `JWTAuth()` — 解析 Bearer token，设置 context values
- `BanCheck()` — 拒绝被封禁用户
- `RequirePermission(code)` — 权限码校验
- `RequireOwnerOrPermission(key, code)` — 资源归属 或 权限码 放行

### Database (`db/db.go`, `db/sql.go`, `db/mongo.go`)
Store 接口抽象，支持 SQLite/MySQL/PostgreSQL/MongoDB。SQL 用 GORM，Mongo 用官方驱动。

### Config (`config/config.go`)
环境变量配置：PORT、JWT_SECRET、DB_TYPE、DB_DSN。

### Swagger (`docs/`)
swaggo/swag 生成，启动后访问 `/swagger/`。注解写在 handler 方法上方。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| PORT | 8080 | 服务端口 |
| JWT_SECRET | change-me-in-production | JWT 签名密钥 |
| DB_TYPE | sqlite | 数据库类型 (sqlite/mysql/postgres/mongo) |
| DB_DSN | (空) | 数据库连接字符串 |

## 运行

```bash
# 后端
cd packages/server && go run main.go dev

# 前端
pnpm dev:web

# Swagger 文档
open http://localhost:8080/swagger/
```

## API 端点总览 (47个)

### Public
- `POST /api/v1/auth/register` — 注册
- `POST /api/v1/auth/login` — 登录
- `POST /api/v1/auth/refresh_token` — 刷新令牌
- `POST /api/v1/auth/reset_password` — 重置密码
- `GET /api/v1/health` — 健康检查

### Protected (需要 JWT)
- Auth: logout, refresh, change_password, first_change_password
- User: profile (GET/PUT), list, get, delete, update_role
- Role: list, create, delete
- Permission: list, get_role_perms, sync
- Categories: list, create, delete
- Tags: list, create, delete
- Posts: list, search, get, create, update, delete
- Post Tags: get, set
- Comments: list_by_post, create, delete
- Likes: toggle/get (post + comment)
- Favorites: toggle, get_status, list_by_user
- Follows: follow, unfollow, status, followers, following, counts
