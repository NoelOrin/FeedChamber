## Why

`app/server` 在 2026-10-13 连续做完两件事（移除 MongoDB 后端、建立可观察性）之后，留下三类后果：

1. **死代码与重复实现**：会话鉴权迁移（JWT → 服务端会话）把旧中间件路径变成了零调用者，但代码与「Deprecated 注释」都还留着；client_ip、登出、`pkgSuccess` 各存在两份逐字相同的实现。读者无法判断哪条是权威路径。
2. **响应出口不统一**：handler 走 `pkg.*`，网关 93 处走 `StopWithJSON + {"code":…,"message":…}` 并自造号段（`1004` 同时表示参数错误与限流、`1092–1099` 挤占认证区、`4001/4002` 被挪用、`5002` 未定义、Discovery 把 HTTP 状态当业务码）。前端 `client.ts` 只读 `json.msg`，于是网关给出的原因被整条丢弃；维护者写的 `error-view.ts` 里 `5004` 分支因超时体用字符串码而永不命中。
3. **装配与契约不可读**：`container.New` 一个函数同时构造对象图、起 5 个后台协程、写库并吞掉错误；`Handler` 持有整个 container 当 service locator，`Shutdown()`（worker 生命周期）挂在 HTTP 层类型上；`gateway.Deps.KV` 声明为 `any`，接错线时静默降级成「限流关闭 / 无人能登录」；跨包共享的身份上下文键是散落在 13 个文件里的字符串字面量。

这不是风格问题：第 2 项是线上可见的信息丢失，第 3 项让「改一处会不会漏改另一处」无法靠编译器回答。

## What Changes

- **死代码删除**：`middleware.JWTAuth` / `OptionalJWTAuth` / `BanCheck` / `RequireRole` / `RequireOwnerOrPermission` / `SetPermissionChecker`、`Deps.Blacklist` + `TokenRevoker`、`pkg/jwt.SigningKey`、master_access_token 时代的 3 个 cookie helper、空的 `handler/cookie.go`、以及 6 个零调用者的模型/服务/仓储符号（`Space.IsRoot`/`ComputeDepth`/`FlattenSpaceTree`、`GetSpaceRolePermissionsWithInheritance`、`ProviderNames`、`MaskedKey`、`CheckInRepository.GetLatestByUser`）。生产调用数逐个实测为 0。
- **重复实现合一**：`pkg.ClientIP`（两份 22 行逐字相同）；登出只剩 `SessionManager.LogoutHandler`（保留有 3 项测试的那份）；删除 `pkgSuccess` 复制品（其注释称避免循环依赖，而同包 `apikey.go` 早已直接调 `pkg.Success`）。
- **响应信封单一出口**（**BREAKING**：网关错误体由 `{"code":…,"message":…}` 变为 `{code,msg,data}`；成功 `msg` 由 `"成功"` 改为规范字面量 `"success"`）：93 处改经 `pkg.Fail`/`pkg.Success`；号段收回共享表；新增 `pkg.FailStatus` 用于「状态本身是契约」的封禁 404；`MAINTENANCE(5005)→503`、`REQUEST_TIMEOUT(5004)→504`；OAuth2/OIDC 协议端点显式豁免。
- **panic recovery**：新增 `middleware.Recover()` 挂在 `Logger` 之后，堆栈只进日志、响应固定 5001，指标 `orinforum_http_panics_total`（无标签）。
- **装配重构**：`container` 拆 `deps.go`/`container.go`/`runtime.go`，`New` 只构造、`Start` 起后台并返回 `Runtime`；`Deps` 具名结构取代 11 个位置参数；事务管理器 9 份 → 1 份；`SetEventBus` 类型断言改为构造参数；`Handler` 不再引用 container（`handler.Deps` 显式 38 个服务，映射在 `server/wiring.go`）；`Handler.Shutdown()` 删除。
- **网关依赖与身份契约**：`gateway.Deps.KV` 由 `any` 改为 `kvstore.Store`，删掉 6 处运行时断言与 `sessionKV`/`kvClient` 两份复制；`Register` 不再接收 `*gorm.DB`、不再就地 `gateway.New`；身份上下文键收敛为 `internal/pkg/ctxkey.go` 的常量（55 处调用点）。
- **文档纠偏**：4 份 AGENTS.md 与 `docs/ARCHITECTURE.md` 中描述已删除中间件、不存在文件（`auth_gate.go`、`handler/metrics.go`、`handler/cookie.go`、11 个不存在的 repository 文件）、已废弃技术栈（SolidJS）与失真规模的部分全部按实测重写；`docs/backend-overview.md` 补 §9.1 可观察性（即上一 change 唯一未做的 10.5）；`make swag` 补齐 session-auth 遗漏的 `openapi.json` 再生成。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `api-style` — 「统一 JSON 响应结构」由「handler 用 pkg.*」扩为「所有 HTTP 出口只用 pkg.*，错误码为单一共享命名空间」，并写明协议端点豁免与 `FailStatus` 的适用边界；`/categories` 示例改为在用的 `/spaces`（Category 模块已下线）。
- `layered-architecture` — 「各层职责隔离」去掉 middleware 承担认证的表述（认证在网关）；「依赖装配集中化」改为 `New`/`Start` 两步 + composition root 装配网关 + handler 不引用 container；「关联上下文」补 recovery 在链路中的位置与身份键常量化。
- `module-conventions` — 「新增模块改动清单」补 `server/wiring.go` 接线与 `make swag` 两步。
- `operations` — 启动命令纠正为 `go run main.go -e dev`（`dev`/`prod` 子命令从未存在，实测报 unknown command）；内嵌 NATS 用途由「令牌黑名单」改为「会话 read-through 缓存」。

## Impact

- **代码**：`app/server` 内 13 个包的 100+ 文件；净删除约 700 行死码，另有 93 处响应改写点。
- **对外契约**：错误体字段名与部分业务码值变化（见上）；`packages/api-client` 的 `ErrorCode` 补 `BODY_TOO_LARGE/REQUEST_TIMEOUT/MAINTENANCE`，无需改解析逻辑（它本来就读 `msg`）。前端 e2e 与 `orinforum_test.sh` 只断言 `code`/状态码/成功路径，实测 113/113 通过。
- **不受影响**：URL、请求体字段、成功响应 `data` 结构、会话与权限语义。
- **风险**：网关错误码收敛后，任何依赖旧私有号（`1004/1020/109x`）的外部消费者会看到新码。已核对仓库内前端、Dashboard、测试脚本与 `openapi.json`，无一处引用这些私有号。
