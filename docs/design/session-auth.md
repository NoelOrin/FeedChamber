# 设计：用户凭证从 JWT 迁移到服务端 Session

> 状态：已实施（2026-10-13 起，随 vinext 迁移一并落地；用真后端 + `feedchamber_test.sh` 验收）
> 前置：前端 cookie-only 刷新已实现（迁移计划 §4b 的「请求层统一」部分，现已合入主干）

## 1. 现状与真实问题

不是"无状态 JWT"，而是**已经有一套服务端会话表，但它只是 JWT 的附属记录**：

| 组件 | 现状 |
|---|---|
| `model.UserSession`（表 `user_sessions`） | 已有 `SessionID`(=JWT 的 JTI)、`UserUUID`、`IP`、`UserAgent`、`DeviceName`、`Location`、`LastActive`、`CreatedAt`、`ExpiresAt` |
| `internal/gateway/session.go`（321 行） | `TrackingMiddleware` 从 claims 派生 JTI 建行；吊销 = 往 KV 写 `bl:<sid>`；`ListSessions`/`RevokeSession`/`LogoutAll` 读该表 |
| `internal/service/token_blacklist.go`（82 行） | 同上，`bl:` 前缀黑名单 |
| `internal/middleware/auth.go`（358 行） | 凭证优先级 `Authorization: Bearer` > `master_access_token` cookie，解析 JWT，再查 `bl:` 判吊销 |

所以系统**已经在为服务端状态付成本**（每请求一次 KV 吊销查询 + 一张会话表 + 一套 SessionManager），却没拿到 session 的收益：没有滑动过期、token 签发后 24h 内天然有效、必须靠双 token 轮换。

## 2. 关键约束（实测得出，决定了设计形状）

1. **`feedchamber_test.sh` 的 31 个鉴权调用全部走 `Authorization: Bearer <access_token>`，零 cookie**，且从登录响应体取 `access_token`。
2. **`pkg/jwt` 被 15 个文件引用，承担三种职责**，不能整体删除：
   - 用户会话 `GenerateToken`/`GenerateRefreshToken` → 本次替换
   - OAuth 客户端凭证 `GenerateOAuthClientToken` → 当时判定**保留**（见下方 2026-10-18 补记：现已改 opaque，不再是 JWT）
   - OIDC SSO 的 id_token 签名（`sso_oidc.go`）→ **保留**，外部依赖方要靠 JWKS 验签，必须自包含

   > **2026-10-18 补记**：上面「保留 client JWT」的结论已被推翻。OAuth2/OIDC 的 access token 全部改为 opaque（`of_at_` + 32B base64url，服务端只存 SHA-256 hex 于 `oauth_tokens.access_token`，撤销翻 `revoked` 列）；`pkg/jwt` 整包删除，JWT 只剩 OIDC `id_token`（RS256，签名私钥 PEM 持久化在 settings `sso_oidc_signing_key`，`kid` 由公钥推导，TTL 走 `sso_id_token_ttl`）。因此本文「用户 JWT 已废弃、client JWT 保留」不再是现状，现状是**所有 access token 均已 opaque**，设计记录见 [2026-10-18-oauth-opaque-tokens-design.md](../superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md)。

**由此得出的核心设计**：让 `Authorization: Bearer` 携带**不透明 session id** 而非 JWT。字段名 `access_token` 不变、值变成 sid → 约束 1 自动满足，测试脚本一行不改；同时用户凭证真正变成单一服务端类型。

## 3. 目标模型

```
凭证 = 256bit 随机 sid（不透明，无结构，不可自证）
存储 = user_sessions 表（权威） + KV `sess:<sid>`（读穿缓存，TTL = 剩余寿命）
出示 = cookie `master_session`(HttpOnly) 或 `Authorization: Bearer <sid>`
吊销 = 删行 + 删 KV 键（即时生效，无需黑名单）
续期 = 剩余寿命 < 50% 时滑动重设 TTL 并重发 Set-Cookie
```

`master_auth=1` 标志 cookie **保留**——前端 `isAuthenticated()` 与 `proxy.ts` 守卫靠它做廉价的"是否可能已登录"判断，避免每次导航都打后端。

## 4. 改动清单

| # | 文件 | 改动 |
|---|---|---|
| 1 | `internal/model/user_session.go` | 加 `Username`、`Role` 两列（反范式，避免每请求 join users 表）；`SessionID` 注释从 "JWT ID (JTI)" 改为不透明凭证 |
| 2 | `internal/gateway/session_store.go`（新） | `CreateSession` / `LookupSession`(KV 读穿) / `TouchSession`(节流) / `RevokeSession` / `RevokeAllForUser` |
| 3 | `internal/pkg/cookie.go` | `master_session` HttpOnly cookie 读写；删 `master_refresh_token` 相关 |
| 4 | `internal/handler/auth.go` | Login/Register 生成 sid → 建会话 → 下发两个 cookie → 响应体 `access_token` = sid；Logout 删会话 + 清 cookie；`RefreshToken` 改为轮换 sid（新 sid、吊销旧的） |
| 5 | `internal/middleware/auth.go` | `JWTAuth` → `SessionAuth`：取 sid（Bearer 优先，其次 cookie）→ `LookupSession` → 写同样的 context key（`username`/`user_id`/`user_role`/`claims`），**下游 handler 无感**；滑动续期在此处触发 |
| 6 | `internal/gateway/session.go` | `TrackingMiddleware` 不再从 claims 派生 JTI（建会话已提前到 Login）；`blacklistJWT` 删除 |
| 7 | `internal/service/token_blacklist*.go` | **删除**（吊销 = 删行） |
| 8 | `internal/router/router.go` | 移除 `/auth/refresh_token`；`/auth/refresh` 语义改为轮换 |
| 9 | `server/iris.go` | 中间件装配改名 |
| 10 | `internal/gateway/sso.go` / `oauth_provider.go` | 确认其 token 校验路径不走 `SessionAuth`（它们有自己的凭证体系） |
| 11 | 前端 `api-client` | `/auth/refresh_token` 消失后，`tryRefreshToken` 改指 `/auth/refresh`；`AuthResponse.refresh_token` 字段移除 |

## 5. 为什么这次能安全做

- **context key 不变**：`middleware` 继续写 `username`/`user_id`/`user_role`/`claims`，所以 40+ handler 与全部 service 层零改动。
- **Bearer 契约不变**：测试脚本与移动端继续 `Authorization: Bearer <token>`，只是 token 换了种类。
- **可分片交付**：切片 1–2（model + store + 测试）不接线即可独立合并，零风险；3–9 必须一次切换（两种用户凭证并存会扩大攻击面）。

## 6. 迁移与回滚

- **迁移期**：老用户手里的 JWT 会全部失效 → 需重新登录一次。`user_sessions` 表里已有的历史行（JTI 型 sid）不是合法 sid（格式不同），`LookupSession` 直接 miss，无需清理。
- **回滚**：`git revert` 单个提交即可；无数据迁移（新增两列可空，删行操作等价于原来的黑名单）。
- **验收**：`go build ./...` + `go vet ./...` + `go test ./...`（14 包）+ 真后端 sqlite + `bash feedchamber_test.sh -all`，基线为 **112 通过 / 1 已知失败**。

## 7. 基线与当前进度（2026-10-13 实测）

```
DB_TYPE=sqlite DB_DSN=/tmp/of_base.db PORT=8080 \
  ADMIN_PASSWORD=admin123 go run main.go
bash feedchamber_test.sh -all
```

| 时点 | 结果 |
|---|---|
| 初次实测 | 111 通过 / 2 失败 |
| 修掉两个非代码因素后 | **113 通过 / 0 失败** |

两处失败都不是服务端 bug，均已处理：

| 失败项 | 根因 | 处置 |
|---|---|---|
| 健康检查 503 | `{"storage":"storage dir stat: stat uploads: no such file or directory"}`，db/kvstore 均 ok —— 启动环境缺 `uploads/` 目录 | 建目录后转 200 |
| 登出 401 | **测试脚本自身 bug**：`改密还原` 用 `$nt` 调用 `change_password`，而改密会吊销发起方 token；随后 `登出` 复用同一个已失效的 `$nt` 必然 401。更糟的是 `撤销token验证` 之所以"通过"，测到的是**改密**造成的吊销，而不是登出的——这个断言一直是空的 | 已修 `feedchamber_test.sh`：登出前重新 `login` 取新 token `$lt`，登出用 `$lt`、撤销验证也用 `$lt`，两个断言各自成立 |

## 9. 实施进度（截至本文件最后更新）

| §4 项 | 状态 |
|---|---|
| 1 `model/user_session.go` 加 Username/Role | ✅ 已完成，`go build`/`go vet`/14 测试包全绿时验证 |
| 2 `internal/gateway/session_store.go` + 9 个测试 | ✅ 已完成并全绿（含"缓存命中仍校验过期""吊销连带清 KV""按用户枚举逐个失效"三个陷阱用例） |
| 3 `internal/pkg/cookie.go` `master_session` | ✅ 由并行会话完成 |
| 4 `handler/auth.go` 建会话/登出/轮换 | 🟡 由并行会话进行中 |
| 5 `middleware/auth.go` → SessionAuth | 🟡 由并行会话进行中 |
| 6 `internal/gateway/session.go` 去 blacklistJWT | 🟡 进行中 |
| 7 删 `token_blacklist*.go` | ✅ 文件已删除 |
| 8 `router.go` 路由调整 | 🟡 进行中 |
| 9 `iris.go` 装配 | 🟡 进行中 |
| 11 前端 `api-client` 刷新路径 | ✅ 已完成（见下） |

**第 11 项曾是一个线上回归**：后端已删除 `/auth/refresh_token` 路由，但 `api-client/src/auth.ts` 的 `tryRefreshToken()` 仍在 POST 该路径——它挂在**每个请求的 401 自动刷新**上，意味着任何一次 token 过期都会刷新失败并被踢下线。已修：

- `tryRefreshToken()` → `POST /auth/refresh`（轮换活会话，滑动续期）
- `AuthResponse` 移除 `refresh_token` 字段（后端 `auth_iface.go` 已不返回）
- 删除 `authService.refreshToken()` 与零消费者的 `RefreshTokenParams` 类型
- `mfaService.verifyLogin` 返回类型去掉 `refresh_token`（后端只回 `access_token` = sid）
- 文件头文档改写为 session 模型

验证：`pnpm --filter @feedchamber/api-client build` 通过、20 个单测全绿、全仓 grep 无悬空引用。

**刻意未改的三处 `refresh_token`**（当时经核实均合法）：`oauth_provider.go` 给第三方 OAuth 客户端签发的 refresh token、`settings.go` 的 `jwt_refresh_token_ttl`（服务于前者）、`services/oauth.ts` 里可选字段的 callback 类型。2026-10-18 更新：前两项已不存在——provider 没有 refresh grant，refresh token 已停止签发，`jwt_refresh_token_ttl` 随之删除（`jwt_access_token_ttl` 改名为 `oauth_access_token_ttl`，另新增 `sso_id_token_ttl`）；第三项是前端可选字段，不受影响。

## 10. 轮换语义对 Bearer 客户端的影响（实测发现）

`/auth/refresh` 的实现是**签发新 sid 并立即作废旧 sid**。实测：

```
login  -> sid=4be9de06…
refresh -> new sid=2ac96628…（与旧值不同）
GET /user/profile  Authorization: Bearer <旧 sid>  -> 401
GET /user/profile  Authorization: Bearer <新 sid>  -> 200
```

浏览器无感（`Set-Cookie` 自动替换），但**任何 Bearer 客户端必须采纳响应里的新 `access_token`**，否则下一次请求即 401。原设计只写了"轮换 sid"，未记这条契约后果，是本次迁移中我遗漏的一点。

`feedchamber_test.sh` 的 `test_auth_flow` 正是因此失败：它在 `刷新token` 之后继续复用 `$token`，导致 `修改密码`/`改密还原` 连带 401。已修——接住轮换出的新 token 再往下走。修完全量 **113 通过 / 0 失败**（全新 DB + 全新进程单次运行，排除限流串扰）。

> 若希望更宽容，可给旧 sid 一个短暂宽限期（双活窗口）。代价是"即时吊销"被削弱，且并发刷新会产生多个有效凭证。当前选择严格轮换。

## 11. 验收状态（2026-10-13 实测）

| 门 | 结果 |
|---|---|
| `go build ./...` | 曾通过；当前被 `internal/pkg/tracing` 阻断（并行会话 40 秒前正在改写该文件，与迁移无关） |
| `go vet ./internal/gateway/` | ✅ 由红转绿：修掉 `resp.RawCookies()`（该 API 不存在，正确写法 `resp.Raw().Cookies()`，2 处）与两处 `store` 声明未使用 |
| `go test ./...` | **14/15 包 ok**，含 `internal/gateway`（本设计的 9 个 session store 用例全通过）；唯一 FAIL 是上述 `tracing` |
| `bash feedchamber_test.sh -all` | ✅ **113 / 0** |
| `pnpm --filter @feedchamber/web build`（vinext） | ✅ EXIT=0，产出 `dist/{client,server}` |
| `tsc --noEmit` | ✅ exit 0 |
| `biome check app/web` | ✅ 79 文件 0 error（1 个 a11y warning） |
| `pnpm test:e2e` | ✅ **36 / 36**（真后端 + React 应用，无需任何临时 shim） |
| 生产产物可启动 | ✅ `node node_modules/vinext/dist/cli.js start` 起来并按 `proxy.ts` 正确重定向 |

**本轮为跑通上述门而做的改动**：`lib/icons.ts` 补 2 个缺失导出（曾使构建失败）；根 `package.json` 声明 `playwright`、`pnpm-workspace.yaml` 加 `onlyBuiltDependencies: [playwright]`（当时浏览器验收脚本依赖一个仓库从未声明的包，该验收门在 CI 里根本执行不了；该脚本其后已删除，`playwright` 现由 `scripts/axe-audit.mjs` 使用）；两个 Dockerfile 脱离 nitro 产物路径；`feedchamber_test.sh` 适配轮换契约。

**仍未完成**：`internal/pkg/tracing` 编译（并行会话在做）、删除 `.solid-legacy`（P6 收尾）、Docker 镜像实际构建验证（daemon 未运行）、P4 交互级 e2e（现有断言只覆盖"路由渲染出内容"，不含发帖/评论等写操作）。


## 8. 顺带发现的文档/代码不一致

- `AGENTS.md` 写 `go run main.go dev`，但 `dev`/`prod` 子命令已被移除，现在直接跑根命令
- `AGENTS.md` 环境变量表写 `OR_ADMIN_PASSWORD`，代码实际读 `ADMIN_PASSWORD`
- `seed.go` 日志 `default admin created (admin / admin123)` 是硬编码文案，真实口令是随机生成的那个
