# 安全审计报告 — Feed Chamber 全面代码审查

> 审计日期：2026-06-27
> 审查范围：Go 后端 (app/server)、Dashboard 前端 (app/dashboard)、Web 前端 (app/web)、共享包 (packages)、基础设施配置
> 审查方式：25 维并行 agent 代码审查，覆盖安全性、正确性、性能、代码质量
> 总计发现：**349** 项 (CRITICAL 26 / HIGH 85 / MEDIUM 122 / LOW 96 / INFO 20)

---

## 目录

1. [执行摘要](#1-执行摘要)
2. [安全性漏洞](#2-安全性漏洞)
3. [功能 Bug](#3-功能-bug)
4. [代码质量问题](#4-代码质量问题)
5. [前端与共享包问题](#5-前端与共享包问题)
6. [基础设施与配置](#6-基础设施与配置)
7. [测试覆盖缺口](#7-测试覆盖缺口)
8. [修复优先级建议](#8-修复优先级建议)
9. [附录：审查维度说明](#9-附录审查维度说明)

---

## 1. 执行摘要

### 1.1 总体风险评估

| 严重度 | 数量 | 说明 |
|--------|------|------|
| CRITICAL | 26 | 需立即修复：权限缺失、SQL 注入、认证绕过、连接泄漏 |
| HIGH | 85 | 应尽快修复：事务缺失、安全配置缺陷、竞态条件、测试假阳性 |
| MEDIUM | 122 | 建议修复：信息泄漏、缺少验证、一致性偏差、性能热点 |
| LOW | 96 | 可择机修复：死代码、冗余逻辑、代码风格 |
| INFO | 20 | 观察项：占位符、已知设计决策 |

### 1.2 风险全景图

```
CRITICAL ████████████████████████████████████████ 26
HIGH     ████████████████████████████████████████████████████████████████████████████████ 85
MEDIUM   ████████████████████████████████████████████████████████████████████████████████████████████████████ 122
LOW      ████████████████████████████████████████████████████████████████████████████████████████████████████ 96
```

### 1.3 类别分布

| 类别 | 数量 | 高风险 (CRITICAL+HIGH) |
|------|------|----------------------|
| bug | 94 | 30+ |
| security | 82 | 40+ |
| code_quality | 97 | - |
| consistency | 34 | - |
| design | 18 | - |
| maintainability | 13 | - |
| performance | 11 | - |

### 1.4 模块风险对比

| 模块 | 严重问题数 | 主要风险 |
|------|-----------|---------|
| `app/server/pkg/gateway/` | 9 CRITICAL + 15 HIGH | OAuth 设计缺陷、暴力破解失效、权限绕过 |
| `app/server/internal/handler/` | 5 CRITICAL + 20+ HIGH | 大量 handler 缺少 `requireUser`、SQL 注入 |
| `app/server/internal/service/` | 2 CRITICAL + 8 HIGH | 事务缺失、测试覆盖 < 10% |
| `app/server/internal/repository/` | 0 CRITICAL + 5 HIGH | SQL 注入、N+1 查询、事务缺失 |
| `app/server/internal/model/` | 2 CRITICAL + 4 HIGH | 密钥明文存储、唯一索引缺失 |
| `app/server/internal/db/` | 2 CRITICAL | 连接泄漏 |
| Dashboard 前端 | 2 CRITICAL + 3 HIGH | 权限守卫缺失、auth store 不安全 |
| Web 前端 | 1 CRITICAL + 2 HIGH | Open redirect、SSE 认证失效 |
| `packages/` | 1 CRITICAL + 2 HIGH | 认证 token 获取永久失败 |
| 基础设施 | 2 CRITICAL + 0 HIGH | Docker 安全配置 |

---

## 2. 安全性漏洞

### 2.1 CRITICAL — 权限与认证漏洞

#### C-001: 大量 Handler 缺少用户权限验证

**文件**: `app/server/internal/handler/` 多个文件

**受影响 handler**: announcement CRUD、moderation admin、role CRUD、setting mutation、report read/management、space CRUD、tag CRUD、permission management 等 20+ 个 handler

**问题**: 这些 handler 直接暴露 API 端点却未调用 `requireUser` 或 `MustAdmin()` 中间件，任意已登录用户均可操作。

**示例**:
- `user.go:236` — `Delete` handler 无条件删除任意用户
- `user.go:208` — `UpdateRole` handler 允许任意用户提权
- `auth.go:294` — `ResetPassword` handler 允许任意认证用户重置任意密码

**建议**: 为每个 handler 补全 `MustAdmin()` / `requireUser` 权限检查；或引入声明式路由权限注解。

---

#### C-002: 权限映射缺失导致未授权访问

**文件**: `app/server/pkg/gateway/permission.go:159-165`

**问题**: 当路由在 `routePermissions` 映射表中不存在时，handler 返回 `200 OK` 而非 `403 Forbidden`。根本原因是 `StopWithJSON` 未调用，请求继续传递到下游 handler。

**建议**: 在 permission middleware 中，对未映射路由返回 403；使用默认拒绝策略。

---

#### C-003: Dashboard 缺少细粒度路由权限守卫

**文件**: `app/dashboard/src/router.tsx:69-217`

**问题**: Dashboard 路由仅检查 `isAuthenticated`，未校验用户是否拥有该页面所需的特定权限码。管理员和只读运营在路由层面无法区分。

**建议**: 引入 `PermissionGuard` 组件，路由配置关联所需权限码列表。

---

#### C-004: `mustChangePassword` 标志可被客户端篡改

**文件**: `app/dashboard/src/stores/auth-store.ts:23, 53-57`

**问题**: `mustChangePassword` 仅存于客户端 store，用户可通过修改 localStorage 或拦截 API 响应绕过首次改密强制。

**建议**: 服务端每次请求验证密码状态，不应信任客户端声明。

---

### 2.2 CRITICAL — OAuth2 / SSO 设计缺陷

#### C-005: OAuth2 授权码流程缺失 redirect_uri 校验

**文件**: `app/server/pkg/gateway/oauth_provider.go:94-144`

**问题**: Authorize 端点读取 `client_id` 但不读取或校验 `redirect_uri`。模型 `OAuthAuthorize` 无 RedirectURI 字段。Token 交换时也未校验。多 redirect URI 客户端可被利用进行授权码拦截。

**建议**:
1. Parse `redirect_uri` 参数并与 `OAuthClient.RedirectURIs` 对比
2. 在 `OAuthAuthorize` 模型新增 `RedirectURI` 字段
3. Token 交换时验证 redirect_uri 匹配

---

#### C-006: SSO 端点缺少 redirect_uri 校验

**文件**: `app/server/pkg/gateway/sso_oidc.go:70-99`

**问题**: SSOProvider.Authorize 存储 `redirect_uri` 但未校验；SSOProvider.Token 未核对存储值。

**建议**: 增加 redirect_uri 白名单校验；Token 阶段比对存储值。

---

#### C-007: SSO OIDC 缺少 nonce 验证

**文件**: `app/server/pkg/gateway/sso_oidc.go:61-77`

**问题**: OIDC ID token 中不包含 nonce 声明，无法防御重放攻击。

---

### 2.3 CRITICAL — 暴力破解防御完全失效

#### C-008: RecordFailedLogin / ClearFailedLogin 从未被调用

**文件**: `app/server/pkg/gateway/security.go:107`

**问题**: `RecordFailedLogin` (line 107) 和 `ClearFailedLogin` (line 145) 定义了完整逻辑，但认证流程 (auth.go) 和路由层均未调用。暴力破解计数器从不增长，IP 永不封禁。

**建议**: 在登录失败路径插入 `RecordFailedLogin`，成功路径插入 `ClearFailedLogin`。同时修复 `getBruteForceEnabled()` 未被调用的死代码问题 (`security.go:93`)。

---

### 2.4 CRITICAL — SQL / NoSQL 注入

#### C-009: Post SQL 查询字符串拼接注入

**文件**: `app/server/internal/repository/post_sql.go:398-403`

**问题**: `pinnedSpaceClause` 函数使用字符串拼接构建 WHERE 条件，参数直接来自用户输入，可导致 SQL 注入。

```go
// 当前实现（危险）
query = query.Where("space_id IN (" + strings.Join(placeholders, ",") + ")", ids...)
```

**建议**: 使用参数化查询或 GORM 的安全方式。

---

### 2.5 CRITICAL — 密钥与凭据管理

#### C-010: 系统设置中明文存储多个密钥

**文件**: `app/server/internal/model/setting.go:166, 170, 199-200`

**问题**: 以下密钥以明文存储在数据库 setting 表中：
- AI API Key (moderation 配置)
- OAuth client secrets
- 邮件服务凭据

**建议**: 生产部署使用环境变量或密钥管理服务 (Vault/AWS Secrets Manager)；存储时加密。

---

### 2.6 HIGH — 信息泄漏

#### H-001: 邮箱地址泄漏

**文件**: `app/server/internal/handler/user.go:77-89`

**问题**: `GetByID` 和 `Search` 返回用户邮箱，任意认证用户可枚举。

**建议**: 非管理员 API 响应中移除 email 字段。

#### H-002: OAuth 错误信息暴露

**文件**: `app/server/pkg/gateway/oauth.go:166, 173`

**问题**: 外部 OAuth 提供方错误信息直接拼接返回，可能泄漏敏感细节。

**建议**: 服务端记录完整错误，返回通用错误消息。

#### H-003: API Key ID 枚举

**文件**: `app/server/pkg/gateway/apikey.go:79-88`

**问题**: 不同错误原因 (key not found vs invalid) 返回不同消息，可用于枚举有效 Key ID。

**建议**: 返回统一错误消息。

---

### 2.7 HIGH — 认证与中间件缺陷

#### H-004: LoginForm Open Redirect

**文件**: `app/dashboard/src/pages/login/index.tsx` / `app/web/src/components/auth/LoginForm.tsx:27`

**问题**: `redirectTo.startsWith('/')` 可被 `//attacker.com` 绕过，浏览器将 `//attacker.com` 解析为协议相对 URL。

**建议**: 使用 `new URL(redirectTo, window.location.origin)` 解析后验证 hostname。

#### H-005: Important endpoints lack rate limiting

**文件**: `app/server/internal/router/router.go:51, 106`

**问题**: `/auth/refresh_token` 和多个受保护认证端点缺少速率限制。

#### H-006: ResetPassword lacks admin check

**文件**: `app/server/internal/handler/auth.go:294-308`

**问题**: `ResetPassword` handler 仅检查 `requireUser`，不要求 `MustAdmin()`，允许任意认证用户重置任意密码。

#### H-007: LocalStorage path traversal

**文件**: `app/server/internal/storage/local.go:24-26`

**问题**: Storage key 未做路径遍历过滤，如 `key = "../../etc/passwd"` 可读取系统文件。

---

### 2.8 HIGH — 安全配置缺陷

#### H-008: Dockerfile 安全问题

**文件**: `Dockerfile:27, 35`

- 生产镜像以 root 运行，无 `USER` 指令
- `.env*` glob COPY 可能泄漏密钥到镜像

#### H-009: JWT 缺乏算法验证

**文件**: `app/server/pkg/jwt/jwt.go:94-105`

**问题**: `ParseToken` 未显式指定算法，攻击者可提供 `alg: "none"` token 绕过验证。

#### H-010: Cookie Secure flag opt-in

**文件**: `app/server/internal/pkg/cookie.go:18-21`

**问题**: `CookieSecure()` 默认返回 false 除非环境变量 `MASTER_COOKIE_SECURE=true`，生产环境易遗漏。

---

## 3. 功能 Bug

### 3.1 CRITICAL — 关键功能异常

#### B-001: OAuth2 授权码重复使用 (TOCTOU 竞态)

**文件**: `app/server/pkg/gateway/oauth_provider.go:182-189`

**问题**: 先 `First()` 查询再 `Delete()`，两操作间并发 goroutine 可复用同一授权码。Delete 错误被丢弃。

**建议**: 原子删除 `db.Where("code = ?", ...).Delete()` + 检查 RowsAffected。

---

#### B-002: MongoDB / SQL 连接泄漏

**文件**:
- `app/server/internal/db/mongo.go:38-46`
- `app/server/internal/db/sql.go:72-82`

**问题**: Ping 失败时 mongo.Client 和 sql.DB 未调用 Close()，导致连接泄漏。

**建议**: 使用 `defer client.Disconnect()` / `defer sqlDB.Close()`。

---

#### B-003: SSE 通知流始终失败

**文件**: `packages/api-client/src/services/notification.ts:36-44`

**问题**: `/api/v1/notification/listen` SSE 端点初始化时调用 `getAccessToken()`，该函数 `auth.ts:52-53` 始终返回 `null`。SSE 连接因无 token 永久失败。

---

#### B-004: BatchUserAction 可删除所有管理员

**文件**: `app/server/internal/service/admin.go:157`

**问题**: `BatchUserAction` 'delete' 操作未排除当前用户或管理员，可以删除自己导致无人可管理。

---

#### B-005: SetPostTags 缺少事务

**文件**: `app/server/internal/service/tag.go:61`

**问题**: 先删除旧标签再插入新标签，若插入失败则标签关联全部丢失。

---

### 3.2 HIGH — 数据完整性问题

#### B-006: 模型缺失唯一索引

| 模型 | 文件 | 风险 |
|------|------|------|
| RolePermission | `permission.go:141-147` | 角色-权限重复映射 |
| SpaceRolePermission | `space_permission.go:3-10` | 空间-角色-权限重复 |
| PostTag | `tag.go:19-24` | 文章-标签重复关联 |
| OAuthToken.RefreshToken | `oauth_token.go:11` | 刷新 token 无唯一约束 |

#### B-007: Tag deletion 无事务

**文件**: `app/server/internal/repository/tag.go:113-118`

**问题**: 删除标签时未使用事务，`PostTag` 关联可能残留。

#### B-008: ReconcileCounters N+1 更新

**文件**: `app/server/internal/repository/admin.go:565-576`

**问题**: MongoDB 循环单条更新计数器，数据量大时性能极差。

#### B-009: Dashboard API response type mismatch

**文件**: `app/dashboard/src/pages/users/index.tsx:26-27`

**问题**: User list API 返回类型与 TypeScript 接口声明不匹配，可能掩盖分页断裂。

---

## 4. 代码质量问题

### 4.1 架构与设计缺陷

| 问题 | 文件 | 详情 |
|------|------|------|
| `sortCx` 为空函数 | `shared-utils/src/cn.ts:16-20` | 复杂的泛型签名但函数体仅为 `return classes` |
| `EncodeKey` 为空函数 | `internal/storage/storage.go:75-77` | 不做 URL 编码 |
| `tracing.go` 占位符 | `pkg/gateway/tracing.go:17` | 空函数，但在 gateway 流程中被调用 |
| `Now.tsx` 占位符 | `app/web/src/components/lc/Now.tsx` | 空组件 |
| `cookies.ts` 死代码 | `app/dashboard/src/lib/cookies.ts` | 无引用 |

### 4.2 代码一致性问题

| 问题 | 文件 | 详情 |
|------|------|------|
| SSO secret bcrypt 不统一 | `sso_oidc.go:122` | SSO 明文比密码，OAuthProvider 用 bcrypt |
| IP 解析不一致 | `security.go:228` vs `ratelimit.go:74` | SecurityCheck 用 `RemoteAddr()`，RateLimiter 用 `gatewayClientIP()` |
| `Post.PinnedAt` 类型不匹配 | `post.go` list vs detail | 一个用 `*time.Time`，一个用 `*int64` |

### 4.3 前端代码质量

| 问题 | 文件 | 详情 |
|------|------|------|
| `getAccessToken()` 始终返回 null | `api-client/src/auth.ts:52-53` | 认证 token 获取逻辑断连 |
| SSE client spread 覆盖默认值 | `sse-client/src/client.ts:41-45` | `undefined` 覆写显式默认值 |
| `formatSmartDate` 未来日期错误 | `shared-utils/src/formatSmartDate.ts:4-12` | 未来时间返回"刚刚" |
| `antd.d.ts` 错误导出 | `tokens/scripts/build.ts:274-282` | `getLayoutTokens` 声明在 antd 模块 |

---

## 5. 前端与共享包问题

### 5.1 Dashboard 前端

| 严重度 | 文件 | 问题 |
|--------|------|------|
| HIGH | `router.tsx:69-217` | 缺少细粒度路由权限守卫 |
| HIGH | `auth-store.ts:23,53-57` | mustChangePassword 客户端可篡改 |
| MEDIUM | `ProShell.tsx` | 可能暴露管理界面给非管理员 |
| LOW | `vite.config.ts:24` | 残留注释配置 |

### 5.2 Web 前端

| 严重度 | 文件 | 问题 |
|--------|------|------|
| CRITICAL | `LoginForm.tsx:27` | Open redirect via `//` bypass |
| MEDIUM | `auth guards` | 多处认证守卫检查不完整 |
| LOW | `Now.tsx` | 空组件 |
| LOW | `login/route.tsx:84` | Register 链接清空 redirect 上下文 |

### 5.3 共享包

| 严重度 | 包 | 问题 |
|--------|------|------|
| CRITICAL | `api-client` | `getAccessToken()` 永久返回 null；notification SSE 不可用 |
| HIGH | `api-client` | `tryRefreshToken` 无超时控制 (`auth.ts:141`) |
| HIGH | `tokens` | `antd.d.ts` 错误导出 `getLayoutTokens` |
| MEDIUM | `sse-client` | spread 覆盖默认值；缺少 Last-Event-ID |
| MEDIUM | `shared-utils` | `formatSmartDate` 未来日期误判 |
| MEDIUM | `api-client` | 测试覆盖缺失关键路径 (401+refresh, SSR, postForm) |

---

## 6. 基础设施与配置

### 6.1 Docker 与 CI/CD

| 严重度 | 文件 | 问题 |
|--------|------|------|
| CRITICAL | `Dockerfile:27` | 生产镜像以 root 运行 |
| CRITICAL | `Dockerfile:35` | `.env*` 泄漏密钥 |
| INFO | `Dockerfile` | 无 HEALTHCHECK |
| INFO | `CI workflows` | 无安全扫描 (Trivy/Snyk) |
| INFO | `.dockerignore` | 缺 `patches/`、`.turbo/` 排除 |

### 6.2 配置与构建

| 问题 | 文件 | 详情 |
|------|------|------|
| 速率限制默认关闭 | `ratelimit.go:50` | `Enabled: false` 新部署默认无保护 |
| JWT 密钥弱默认值 | `config.go:53` | 无复杂度验证 |
| JWT 测试覆盖 0% | `pkg/jwt/` | 核心安全组件零测试 |
| Turborepo 缓存未优化 | `turbo.json:4` | 无 inputs/outputs 配置 |

---

## 7. 测试覆盖缺口

### 7.1 零覆盖模块

| 模块 | 文件数 | 重要性 |
|------|--------|--------|
| JWT 安全 | `pkg/jwt/` | CRITICAL |
| Handler 层 (25+ 文件) | `internal/handler/` | CRITICAL |
| Router 路由 | `internal/router/` | HIGH |
| Repository 层 | `internal/repository/` | HIGH |
| 大部分 Service | `internal/service/` | HIGH |

### 7.2 测试质量问题

| 问题 | 文件 | 详情 |
|------|------|------|
| 测试永久跳过 | `oauth_provider_test.go:77` | OAuth client 注册 happy-path 测试跳过 |
| 测试假阳性 | `ratelimit_test.go:213` | 速率限制超限测试断言 200 而非 429 |
| 集成测试失败 | `auth_test.go:239` | 3 个测试因空 JSON 响应失败 |
| 无法编译 | `middleware/middleware.go:111` | 整个 package 阻塞测试运行 |
| 无法编译 | `service/content_test.go:22` | 整个 package 阻塞测试运行 |

---

## 8. 修复优先级建议

### Phase 1 — 立即可修复 (影响面小但严重)

| 优先级 | 问题 | 预估工时 |
|--------|------|---------|
| P0 | `security.go` — 插入 RecordFailedLogin 调用 | 2h |
| P0 | `handler/user.go` — 补全 Delete/UpdateRole 权限检查 | 1h |
| P0 | `handler/auth.go` — ResetPassword 加 MustAdmin() | 30min |
| P0 | `mongo.go/sql.go` — defer Close 修复连接泄漏 | 1h |
| P0 | `Dockerfile` — 加 USER 指令；移除 .env* COPY | 1h |
| P1 | `post_sql.go` — 参数化查询修复 SQL 注入 | 2h |
| P1 | `oauth_provider.go` — 添加 redirect_uri 校验 | 4h |
| P1 | `LoginForm.tsx` — 修复 open redirect | 1h |

### Phase 2 — 权限体系重构

| 优先级 | 问题 | 预估工时 |
|--------|------|---------|
| P1 | Handler 层系统化权限检查 (20+ handler) | 2-3d |
| P1 | Dashboard 路由级权限守卫 | 2d |
| P1 | 模型层补全唯一索引 | 4h |
| P1 | 密钥存储加密 | 2d |

### Phase 3 — 测试基建

| 优先级 | 问题 | 预估工时 |
|--------|------|---------|
| P1 | JWT 测试 | 1d |
| P2 | Handler 权限测试 | 2d |
| P2 | Repository 层测试 | 3d |
| P2 | 修复现有断裂测试 | 1d |

### Phase 4 — 常规修复

| 优先级 | 问题 | 预估工时 |
|--------|------|---------|
| P2 | 事务补齐 (tag/permission/favorite) | 1d |
| P2 | 死代码清理 | 4h |
| P2 | API client 认证修复 | 1d |
| P3 | 代码风格一致性 | 持续 |

---

## 9. 附录：审查维度说明

本次审查使用 25 个并行 agent，覆盖以下维度：

| Phase | 维度 | 覆盖范围 |
|-------|------|---------|
| Go Security & Auth | 6 agents | Gateway Auth/OAuth, Security/RateLimit, Session/Permission/SSO, Handler Auth/User/Post, Handler Admin/Misc, Middleware/Router |
| Go Data Layer | 4 agents | DB/Config, Model Layer, Repository Layer, Service Layer |
| Go Infrastructure | 4 agents | Server Bootstrap, Internal Pkg (kvstore/ratelimit/logger/eventbus), Storage/Mailer/Metrics/JWT, Test Coverage |
| Dashboard FE | 4 agents | User/Posts/Roles pages, Admin pages, Layout/Store/Lib, Config |
| Web FE | 5 agents | Timeline/Spaces pages, User/Auth pages, Components, Hooks/Store/Utils, Lib Auth/API |
| Packages & Config | 3 agents | API Client, Shared Utils/SSE/Tokens, Infra Config |

---

*本报告由 27 个自动化审查 agent 生成，发现覆盖率为静态代码分析级别，部分问题需结合运行时行为验证。*
