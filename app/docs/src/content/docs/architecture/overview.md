---
title: 架构总览
description: 请求链路、模块划分与关键设计取舍
sidebar:
  order: 1
---

## 请求链路

请求严格单向流过以下各层，不允许跨层调用：

```text
HTTP Request
  → Gateway / Middleware
      Preflight: trace → 全局限流 → 可选凭证解析
      Enforce:   必需凭证 → banCheck → 权限 → 安全风控 → 会话跟踪
    → Handler   解析参数、调 service、封装响应
      → Service  业务逻辑、返回 *pkg.AppError
        → Repository  数据访问接口
          → Database  SQL via GORM
```

## 目录结构

```text
app/server/
├── main.go              入口，chdir 到自身目录后调 cmd.RootCmd
├── cmd/                 Cobra：backup / import-discourse / recompute-xp
├── server/              Start(env) 启动编排 + seed + wiring
└── internal/
    ├── config/          环境变量
    ├── db/              Store 抽象 + GORM 实现
    ├── model/           纯数据模型（GORM 标签，无业务）
    ├── repository/      每资源一个文件：接口 + SQL 实现
    ├── service/         业务逻辑（*_iface.go + 实现）
    ├── handler/         HTTP 层
    ├── gateway/         认证与授权子系统（会话 / API Key / OAuth / OIDC / 限流）
    ├── middleware/      身份确定后的门控与请求基础设施
    ├── router/          路由注册，按域拆分
    ├── container/       依赖注入聚合
    ├── authz/           权限判定的单一实现点
    └── pkg/             响应信封、错误码、KV、限流、事件总线、日志
```

## 关键设计取舍

### 1. 写操作一律 POST

不是 REST。理由：这个项目的接口绝大多数是「动作」而不是「资源状态转移」，
`POST /posts/{id}/pin` 比 `PATCH /posts/{id}` 更能表达语义，也避免客户端在
`PUT`/`PATCH`/`DELETE` 之间做无意义的选择。

### 2. 凭证是服务端会话，不是 JWT

`sid` 存在 `user_sessions` 表，KV 做 read-through 缓存。
**「凭证是否有效」等价于「会话行是否存在」**——登出 / 改密 / 改角色 / 封号都直接删行，
不需要黑名单机制。唯一保留 JWT 的地方是 OIDC 的 `id_token`（规范强制）。

### 3. 依赖装配分两步

`container.New()` 只构造对象图（不起协程、不订阅事件、不写库）；
`(*Container).Start(ctx)` 才订阅事件并启动后台 worker，返回的 `*Runtime` 负责关停。

handler 层不引用 container：它消费 `handler.Deps` 里显式列出的服务，
映射写在 `server/wiring.go`，所以「HTTP 层依赖什么」可在一个文件里读完。

### 4. 权限判定的单一实现点

`internal/authz.Allowed` 是唯一判据：

| 凭证来源 | 判据 |
|----------|------|
| 会话 | 只看角色权限表 |
| API Key | scope **且** 角色权限（scope 是凭证上限） |
| OAuth 客户端 | 只认 scope |

网关、路由级 `RequirePermission`、MCP Enforcer 三处都调它。此前三处各写一份且互不一致，
结果是「admin 的只读 Key 能通过 MCP 调用写工具」这类真实提权路径。

### 5. 事件解耦通知

业务 service 发布事件（`content.replied`、`content.liked`…），
NotificationService 订阅并创建通知。MCP 与 HTTP 共用同一批 service，
不为机器入口开审核旁路。

## 数据与基础设施

| 组件 | 作用 |
|------|------|
| PostgreSQL / SQLite / MySQL | 主存储，GORM 抽象，三家后端可切 |
| NATS JetStream | KV（会话缓存、限流窗口、设置、OAuth state）+ 事件总线 |
| 本地磁盘 / S3 | 上传文件 |
| Prometheus 9091 | 指标，独立 listener，不经 nginx |
| 结构化日志 | JSON，`trace_id` / `span_id` / `route` / `latency_ms` |
