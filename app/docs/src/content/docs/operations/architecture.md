---
title: 架构总览
description: 一次请求经过哪几层、代码放在哪、以及几个不要随手改的设计取舍
---

这页给两类人看：要读懂代码的维护者，和需要判断「问题出在哪一层」的运维。
前半是请求链路与目录，后半是几个刻意做成现在这样的取舍——动它们之前先读完原因。

## 请求链路

`/api/v1` 下的请求经过固定的单向链，层与层之间不反向依赖：

```text
HTTP Request
  └─ /api/v1 中间件
       PathBodyLimit(1MB) → RequestTimeout(30s)
       → gw.Preflight() → MaintenanceGate
     └─ 受保护路由   gw.Enforce()
        └─ Handler      解析参数 → 调 service → 写信封
           └─ Service   业务规则 → *pkg.AppError
              └─ Repository  接口 + SQL（GORM）
                 └─ Database   SQLite / PostgreSQL / MySQL
```

路由只在 `app/server/internal/router/` 注册，按域拆成 `routes_*.go`，`router.go`
只留中间件链与各段函数调用。全局中间件顺序不能调：`Trace` 必须在 `Logger`
之前，`Recover` 必须在 `Logger` 之后，否则 panic 连一行请求日志都没有。

## 网关的两段

网关不是单个中间件，而是**两段各自幂等的流水线**：

| 段 | 挂在哪 | 做什么 |
|----|--------|--------|
| `Preflight()` | `/api/v1` 全部 | 端点名写回 trace、全局每 IP 限流（`rate_limit_enabled`，默认关闭）、**可选**凭证解析 |
| `Enforce()` | 受保护路由 | 必需凭证 → 封禁检查 → 权限 → 安全风控 → 会话活跃跟踪 |

两段在 `iris` context 上打阶段标记（`gw_stages`，位掩码：tracing / ratelimit /
credential / enforce），谁先跑过就不再跑第二遍。这不是防御性冗余，而是修过一个真实
缺陷：`Preflight` 对受保护路由本来就生效，若再整体跑一遍，同一个
`rl:ratelimit:<ip>` 键会被计两次——限流开着时，60 次/分钟的上限会在第 30 次请求就
返回 429，同时每次请求多一次会话查询与滑动续期。

封禁检查排在权限之前，且**被封禁的请求返回 404 而不是 403**：探测者无法区分
「无权」与「不存在」，而信封里仍带具体原因给已认证的调用方。

`/mcp` 与 `/.well-known/` 挂在应用根，拿不到这两段保护——它们的鉴权、封禁、
权限、限流由 `internal/mcp` 自己完成。

## 目录结构

```text
app/server/
├── main.go              # 入口：chdir 到自身目录，再交给 cmd.RootCmd
├── cmd/                 # Cobra：backup / import-discourse / recompute-xp
├── server/
│   ├── iris.go          # Start(env)：启动主流程
│   ├── wiring.go        # container → handler.Deps 的映射
│   ├── seed.go          # 角色、权限、管理员、设置种子
│   └── migrate.go       # 存量库升级（等级数值化、total_xp 回填）
├── internal/
│   ├── handler/         # HTTP 层：解析、调 service、写信封
│   ├── service/         # 业务逻辑，返回 *pkg.AppError
│   ├── repository/      # 数据访问：接口 + SQL 实现
│   ├── gateway/         # 认证 / 权限 / 限流 / OAuth / SSO / API Key / 会话
│   ├── middleware/      # 身份确定之后的门控与请求基础设施
│   ├── container/       # 依赖装配（New）与后台任务（Start）
│   ├── metrics/         # Prometheus 注册表
│   └── pkg/             # 信封、错误码、KV、事件总线、限流、日志
└── db/migrations/       # 手写 SQL（无自动执行器）
```

## 启动顺序

`server/iris.go` 的 `Start(env)` 顺序是硬约束，改动其中一步要重读它的注释：

1. 加载配置并 `Validate()`——`NATS_URL`、`DB_TYPE`、`METRICS_PORT` 的问题在这里 `Fatal`
2. 打开数据库，`AutoMigrate` 注册全部模型（当前 69 个）
3. 存量数据升级（等级字符串 → 数字、`total_points` → `total_xp`、删除死列）
4. 种子数据：角色、管理员、设置、XP 与门控默认值
5. 连 NATS，建 KV Store 与 EventBus
6. 构造被网关与中间件共享的服务（权限、设置、禁言、版面、会话、安全）
7. `container.New` 装配对象图 → `container.Start` 起后台任务
8. `handler.New` + `router.Register` → 监听端口

## 装配为什么分两段

`container.New(Deps)` 只构造对象图：**不起协程、不订阅事件、不写库**。
`(*Container).Start(ctx)` 才订阅事件并启动六个后台 worker（浏览量落库、置顶过期、
webhook 投递、周报、XP 每日重算、动议到期解析）。返回的 `*Runtime` 负责关停，
顺序是先刷浏览量、再停 worker、最后关 EventBus——浏览量缓存在 KV 里，必须在关掉
KV 之前落进 SQL。

这样分的好处很直接：测试里可以放心 `New` 一个容器，不会留下跑出测试生命周期的
goroutine；而「哪些东西只有生产才该活」集中在一个文件里，读一篇就够。

## Handler 依赖什么，一个文件读完

`handler` 层不引用 `container`。它消费 `handler.Deps` 里显式列出的服务，
映射写在 `server/wiring.go`——所以「HTTP 层到底用了哪些 service」不用 grep，
读一个文件就完。新增服务漏接线是编译错误，不是运行时 500。

## 权限只有一个判据

`internal/authz.Allowed(cred, perms, code)` 是唯一的权限判定点，网关、路由级
`RequirePermission` 与 MCP 都调它。按凭证来源选判据：

| 凭证来源 | 判据 |
|----------|------|
| 用户会话 | 角色权限表 |
| API Key | scope **且** 角色权限（scope 是这把 Key 的上限） |
| OAuth 客户端 | 只看 scope（客户端不是用户，没有角色） |
| 空权限码 | 登录即可，恒放行 |
| 未知来源 | 拒绝 |

五种默认角色 `admin` / `moderator` / `user` / `pending_user` / `ban`，平台级权限码
61 个；版面级权限另有 `space_role_permissions`，沿祖先链继承。会话与 API Key 走
不同判据这点必须一致，否则 admin 的只读 Key 能穿过写接口——这正是把三份判定合并成
一份的原因。

## 几个刻意的取舍

- **写操作一律 `POST` + 动作后缀**，`PUT` / `PATCH` / `DELETE` 不用；`GET` 只承载
  只读查询。见 [API 约定](/api/conventions/)。
- **会话不是 JWT**：凭证是 64 位十六进制 sid，`user_sessions` 行存在即有效。
  登出、改密、改角色、封号都等于删行，不需要黑名单。见 [认证](/api/authentication/)。
- **通知靠事件解耦**：service 发 `content.replied` 这类事件，通知服务订阅后落库并推
  SSE。发帖流程不认识通知表，通知故障也不回滚发帖。
- **MCP 与 HTTP 共用 service**：`/mcp` 的工具调的是同一批 service，权限、封禁、
  限流复用同一份实例。旁路一套实现会立刻出现「网关说封了、MCP 说没封」。
- **同一资源只留一种读取方法**：`GET` 与 `POST` 双读入口会让缓存、权限与指标各自漂移。

## 验证

```bash
# 依赖三项都通
curl -s localhost:8080/api/v1/health

# 路由与权限判定的回归测试
cd app/server && go test ./internal/router/ ./internal/authz/
```

想知道某条请求走了哪条链，看日志里的 `route` 字段——它是路由模板而不是原始路径，
所以 `/api/v1/posts/01J...` 会显示成 `/api/v1/posts/{id}`。

## 下一步

- 数据怎么落、事件怎么走：[数据与事件](/operations/data-and-events/)
- 指标、日志与告警：[观测与指标](/operations/observability/)
- 命令与退出行为：[CLI](/reference/cli/)
