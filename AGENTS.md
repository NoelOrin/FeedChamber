# OrinForum

Go + SolidJS 社区论坛 monorepo。后端 Iris v12，前端 SolidJS + TanStack Router。

## 项目结构

```
packages/server/
├── main.go                    # 入口，chdir 到自身目录后调用 cmd.RootCmd
├── cmd/root.go                # Cobra 根命令，分发 dev / prod 子命令
├── server/
│   ├── iris.go                # Start(env) — 应用启动主流程
│   └── seed.go                # 角色、权限、管理员、设置等种子数据
├── internal/
│   ├── config/                # 环境变量加载 (PORT/JWT_SECRET/DB_TYPE/DB_DSN/Storage)
│   ├── db/                    # 数据库抽象层
│   │   ├── db.go              #   Store 接口 + 工厂 (sqlite/mysql/postgres/mongo)
│   │   ├── sql.go             #   GORM 实现
│   │   └── mongo.go           #   MongoDB 实现
│   ├── model/                 # 纯数据模型 (GORM + BSON 标签，无业务)
│   │   ├── user.go role.go permission.go
│   │   ├── post.go content.go category.go tag.go
│   │   ├── like.go favorite.go follow.go
│   │   ├── upload.go notification.go report.go setting.go
│   ├── repository/            # 数据访问层 (每资源一个文件，接口+SQL实现+Mongo实现)
│   ├── service/               # 业务逻辑层 (接收 repo 接口，返回 *pkg.AppError)
│   ├── handler/               # HTTP 层
│   │   ├── handler.go         #   Handler 聚合体 + New() 依赖装配
│   │   ├── params.go          #   分页/排序等通用请求解析
│   │   ├── auth.go user.go role.go permission.go
│   │   ├── post.go category.go tag.go content.go
│   │   ├── like.go favorite.go follow.go
│   │   ├── upload.go notification.go report.go
│   │   ├── admin.go setting.go
│   ├── middleware/
│   │   ├── auth.go            # JWTAuth / OptionalJWTAuth / BanCheck
│   │   │                      # RequirePermission / RequireOwnerOrPermission
│   │   │                      # PostAuthGate / CommentAuthGate (动态鉴权)
│   │   └── middleware.go      # CORS / Logger
│   ├── router/router.go       # 所有路由注册
│   ├── pkg/                   # 通用工具
│   │   ├── response.go        #   统一响应 Success / HandleError / 分页
│   │   ├── errors.go          #   ErrCode + AppError + 错误码映射
│   │   ├── jwt.go             #   JWT 签发与解析
│   │   └── id.go              #   ULID 生成
│   └── storage/               # 文件存储抽象 (local / S3)
├── docs/                      # swaggo 生成的 Swagger 文档
├── db/                        # SQLite 数据库文件目录
├── uploads/                   # 本地上传目录
├── go.mod / go.sum
├── Makefile / .air.toml       # 热重载 (air) 与构建脚本
└── package.json               # pnpm workspace 集成
```

## 开发架构

### 分层模式

请求处理严格遵循单向依赖链：

```
HTTP Request
  → Middleware (JWT / Ban / Permission)
    → Handler (解析参数、调 service、封装响应)
      → Service (业务逻辑、返回 AppError)
        → Repository (数据访问接口)
          → Database (SQL via GORM / Mongo via 官方驱动)
```

各层职责清晰隔离，不允许跨层调用。

### 关键设计

**1. Repository 双实现**  
每个 repository 文件定义接口，工厂函数根据 `store.SQL()` 是否为 nil 决定返回 SQL 实现还是 Mongo 实现。Service 层只依赖接口，不关心底层存储。示例模式：

```go
type UserRepository interface { ... }
func NewUserRepository(store db.Store) UserRepository {
    if store.SQL() != nil { return &sqlUserRepo{db: store.SQL()} }
    return &mongoUserRepo{store: store.Mongo()}
}
```

**2. 统一响应**  
所有 handler 用 `pkg.Success(ctx, data)` 或 `pkg.SuccessPaginated(ctx, data, total)` 返回成功，用 `pkg.HandleError(ctx, err)` 返回失败。响应体固定为：

```json
{ "code": 0, "msg": "success", "data": {...} }
```

错误码分段：`1xxx` 认证、`2xxx` 参数、`3xxx` 资源、`5xxx` 服务端。`errToHTTPStatus` 自动把业务码映射到合适的 HTTP status。

**3. 依赖装配**  
`server/iris.go` 的 `Start(env)` 是整个应用的组装点：构造 `config → db.Store → AutoMigrate → seed → storage.Provider → Handler → Router → Listen`。`handler.New()` 集中完成所有 repository 和 service 的实例化，注入到 Handler 聚合体。

**4. 权限系统**  
启动时 seed 13 个权限码和默认角色-权限映射，`PermissionService` 在内存中维护缓存。middleware 通过 `SetPermissionChecker()` 注入 checker 实例，`RequirePermission(code)` 在路由级拦截。`PostAuthGate / CommentAuthGate` 支持通过 setting 动态切换公开/鉴权。

**5. 路由组织**  
`router/router.go` 按 `/api/v1` 前缀分组：public 路由直接挂载，protected 路由统一应用 `JWTAuth() + BanCheck()`，再按资源模块 `PartyFunc` 细分。权限粒度直接标注在路由上。

## 架构

## 新增模块 checklist
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
用户 CRUD、资料编辑 (username/avatar/bio)、角色变更、禁言/解禁。密码字段 JSON 序列化时隐藏。

### Banned — 禁言 (`handler/banned.go`, `service/banned.go`, `repository/banned.go`)
用户禁言管理，支持指定禁言时长（永久或临时），过期后自动解禁。

### Post — 帖子 (`handler/post.go`, `service/forum.go`, `repository/post.go`)
帖子 CRUD、搜索、浏览计数。列表/详情返回 PostListItem DTO（含作者、评论数、分类名）。

### Content — 评论 (`handler/content.go`, `service/forum.go`, `repository/post.go`)
帖子评论 CRUD，支持 parentId 嵌套。独立 handler，支持内容点赞和回复列表。

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

### Upload — 上传 (`model/upload.go`, `handler/upload.go`, `service/upload.go`, `repository/upload.go`, `storage/`)
文件上传抽象层，支持本地存储和 S3。提供预签名签名、上传确认、文件列表和删除。

### Notification — 通知 (`model/notification.go`, `handler/notification.go`, `service/notification.go`, `repository/notification.go`)
用户通知系统，支持列表、未读计数、标记已读、批量标记已读和删除。

### Report — 举报 (`model/report.go`, `handler/report.go`, `service/report.go`, `repository/report.go`)
内容举报系统，支持创建、列表和状态更新（待处理/已处理/已关闭）。

### Admin — 管理 (`handler/admin.go`, `service/admin.go`, `repository/admin.go`)
管理员统计接口，支持系统统计和设置批量操作。

### Setting — 设置 (`model/setting.go`, `handler/setting.go`, `service/setting.go`, `repository/setting.go`)
系统设置管理，支持公开设置读取、所有设置获取、单个设置读写和批量设置。

### Announcement — 公告 (`model/announcement.go`, `handler/announcement.go`, `service/announcement.go`, `repository/announcement.go`)
公告系统，支持 markdown 内容、草稿/发布状态、置顶排序、过期时间、浏览计数。显示组件类型：toast(1)/alert(2)/modal(3)，toast 支持自定义位置（默认右上角）。

### Middleware (`middleware/auth.go`)
- `JWTAuth()` — 解析 Bearer token，设置 context values
- `OptionalJWTAuth()` — 可选 JWT，未登录时匿名访问
- `BanCheck()` — 拒绝被封禁用户
- `RequirePermission(code)` — 权限码校验
- `RequireOwnerOrPermission(key, code)` — 资源归属 或 权限码 放行
- `PostAuthGate()` / `CommentAuthGate()` — 通过 setting 动态切换公开/鉴权
- `CORS()` / `Logger()` — 跨域和请求日志

### Database (`db/db.go`, `db/sql.go`, `db/mongo.go`)
Store 接口抽象，支持 SQLite/MySQL/PostgreSQL/MongoDB。SQL 用 GORM，Mongo 用官方驱动。`BackendType` 枚举：`sqlite` / `mysql` / `postgres` / `mongo`。

### Config (`config/config.go`)
环境变量配置：`PORT`、`JWT_SECRET`、`DB_TYPE`、`DB_DSN`、`Storage`（local / s3）。

### Storage (`storage/storage.go`, `storage/local.go`, `storage/s3.go`)
文件存储 Provider 接口，支持本地和 S3 两种实现。

### Swagger (`docs/`)
swaggo/swag 生成，启动后访问 `/swagger/`。注解写在 handler 方法上方。

### CLI (`cmd/root.go`)
Cobra CLI，支持 `dev` 和 `prod` 子命令。`.env` / `.env.dev` / `.env.prod` 多环境配置。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| PORT | 8080 | 服务端口 |
| JWT_SECRET | change-me-in-production | JWT 签名密钥 |
| DB_TYPE | sqlite | 数据库类型 (sqlite/mysql/postgres/mongo) |
| DB_DSN | (空) | 数据库连接字符串 |
| STORAGE_TYPE | local | 存储类型 (local / s3) |
| S3_ENDPOINT | (空) | S3 endpoint (仅 s3 模式) |
| S3_BUCKET | (空) | S3 bucket (仅 s3 模式) |
| S3_REGION | (空) | S3 region (仅 s3 模式) |
| S3_ACCESS_KEY_ID | (空) | S3 access key (仅 s3 模式) |
| S3_SECRET_ACCESS_KEY | (空) | S3 secret key (仅 s3 模式) |

## API 测试

项目根目录有 `orinforum_test.sh` 全量 API 测试脚本。启动服务器后运行：

```bash
# 全量测试（建议在新的 SQLite 数据库上运行）
cd packages/server && DB_TYPE=sqlite DB_DSN=/tmp/of_test.db go run main.go &
sleep 4
bash ../orinforum_test.sh -all

# 按模块测试
bash ../orinforum_test.sh -auth -post -user
```

详细用法见 `.agents/skills/api-test/SKILL.md`。

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
- `GET /api/v1/health` — 健康检查
- `POST /api/v1/settings/public` — 获取公开设置

### Auth (`POST /api/v1/auth`)
- `/register` — 注册
- `/login` — 登录
- `/refresh_token` — 刷新令牌
- `/reset_password` — 重置密码

### Public 查询 (`POST /api/v1`)
- `/categories/list` — 分类列表
- `/tags/list` — 标签列表
- `/announcements/published` — 已发布公告列表

### Public 详情 (`GET /api/v1`)
- `/announcements/{id}` — 公告详情

### Protected (`POST /api/v1` + JWT)
- `/auth/logout` — 登出
- `/auth/refresh` — 刷新令牌
- `/auth/change_password` — 修改密码
- `/auth/first_change_password` — 首次修改密码

### User (`POST /api/v1/user`)
- `/profile` — 获取/更新资料
- `/search` — 搜索用户
- `/list` — 用户列表
- `/{id}` — 获取用户
- `/{id}/delete` — 删除用户
- `/{id}/role/update` — 更新角色
- `/{id}/banned` — 禁言用户
- `/{id}/unbanned` — 解禁用户
- `/{id}/banned/status` — 获取禁言状态

### Role (`POST /api/v1/role`)
- `/list` — 角色列表
- `/create` — 创建角色
- `/{id}/delete` — 删除角色

### Permission (`POST /api/v1/permission`)
- `/list` — 权限列表
- `/role` — 获取角色权限
- `/sync` — 同步角色权限

### Category (`POST /api/v1/categories`)
- `/create` — 创建分类
- `/{id}/update` — 更新分类
- `/{id}/delete` — 删除分类

### Tag (`POST /api/v1/tags`)
- `/create` — 创建标签
- `/{id}/update` — 更新标签
- `/{id}/delete` — 删除标签

### Post (`POST /api/v1/posts`)
- `/list` — 帖子列表
- `/search` — 搜索帖子
- `/{id}` — 获取帖子
- `/create` — 创建帖子
- `/{id}/update` — 更新帖子
- `/{id}/delete` — 删除帖子
- `/{id}/pin` — 置顶/取消置顶
- `/{id}/lock` — 锁定/解锁
- `/{postId}/tags/set` — 设置帖子标签

### Content (`POST /api/v1/contents/{contentId}`)
- `/like` — 点赞/取消点赞
- `/like/status` — 查询点赞状态
- `/replies` — 获取回复列表

### Content CRUD (`POST /api/v1/posts/{postId}/contents`)
- `/list` — 帖子评论列表
- `/create` — 创建评论
- `/{id}/update` — 更新评论
- `/{id}/delete` — 删除评论

### Favorites (`POST /api/v1/favorites`)
- `/list` — 用户收藏列表

### Post Favorites (`POST /api/v1/posts/{postId}/favorite`)
- `/` — 收藏/取消收藏
- `/status` — 查询收藏状态

### Follow (`POST /api/v1/users/{userId}`)
- `/follow` — 关注
- `/unfollow` — 取消关注
- `/follow/status` — 查询关注状态
- `/followers` — 获取粉丝列表
- `/following` — 获取关注列表
- `/counts` — 获取关注数/粉丝数

### Upload (`POST /api/v1/upload`)
- `/` — 上传文件
- `/presign` — 获取预签名签名
- `/confirm` — 确认上传
- `/list` — 文件列表
- `/{id}` — 获取文件
- `/{id}/delete` — 删除文件

### Notification (`POST /api/v1/notifications`)
- `/list` — 通知列表
- `/unread_count` — 未读数
- `/{id}/read` — 标记已读
- `/read_all` — 全部标记已读
- `/{id}/delete` — 删除通知

### Report (`POST /api/v1/reports`)
- `/create` — 创建举报
- `/list` — 举报列表
- `/{id}/status` — 更新举报状态
- `/{id}/delete` — 删除举报

### Admin (`POST /api/v1/admin`)
- `/stats` — 系统统计
- `/banned/list` — 禁言记录列表
- `/announcements/list` — 公告列表（含草稿）
- `/announcements/create` — 创建公告
- `/announcements/{id}/update` — 更新公告
- `/announcements/{id}/delete` — 删除公告
- `/announcements/{id}/publish` — 发布公告
- `/announcements/{id}/unpublish` — 撤回公告
- `/settings` — 获取所有设置
- `/settings/batch` — 批量设置
- `/settings/{key}` — 获取设置
- `/settings/{key}/update` — 更新设置
- `/settings/{key}/delete` — 删除设置
