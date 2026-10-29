## Why

上一轮把 router 拆域时暴露出两件事：

1. `v1.Use(gw.OptionalHandler())` 对 protected 同样生效，而 `gw.Handler()` 内部又跑一遍 tracing + 全局限流 + 凭证解析，于是**每个受保护请求两次**累加同一个 `"ratelimit:"+ip` 计数、两次会话查询（含滑动续期判定）。全局限流默认关闭所以暂未显形，但一旦启用，protected 的有效阈值直接减半。
2. 流水线只有一个"全有或全无"的入口，挂载点因此变成正确性的隐藏条件：子 party 多挂一次就会顶掉兄弟 party 已快照的链（同批发现的权限门失效即由此而来）。

根因是同一个：流水线不可组合。拆成段并让每段幂等，既去掉二次计数，也让"挂哪儿"不再是安全前提。

## What Changes

- `pkg/gateway`：新增 `Preflight()`（tracing + 全局 IP 限流 + 可选凭证解析）与 `Enforce()`（必需凭证 → banCheck → 权限 → 安全风控 → 会话跟踪）；`Handler()` 保留为二者的组合，供单 party 场景与测试；删除 `OptionalHandler()`（被 `Preflight()` 取代，全仓仅一处调用）。
- 每段以 iris ctx 上的 stage 标记（`gw_stages`）实现幂等：`Preflight` 与 `Enforce` 各自最多执行一次，因此 `v1.Use(Preflight)` + `protected.Use(Handler)` 也不会二次计数。
- `Enforce` 在 Preflight 已解析出身份时跳过第二次会话查询（API Key 请求不受影响：Preflight 从不解析 API Key，故必然走完整解析分支）。
- `internal/router`：`v1.Use(gw.Preflight())` + `protected.Use(gw.Enforce())`；`middleware_chain_test.go` 的源码扫描由"禁止重复挂 OptionalHandler"升级为"禁止子 party 出现任何 `Use(gw.`"。
- 文档与规范同步：`router/AGENTS.md`、`middleware/AGENTS.md`、根 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/backend-overview.md`（§4.1 入口表、§6 挂载、§13 条目与编号）。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `layered-architecture` — 依赖链的网关段改写为"Preflight 挂 v1 / Enforce 挂 protected"；新增一条要求：网关流水线 MUST 分段可组合且每段对同一请求恰好执行一次。

## Impact

- **行为**：受保护请求的全局限流计数由 2× 变 1×（即有效阈值回到标称值——这是修正，不是放宽：标称值本就是运维配置的意图值）；每请求少一次 `user_sessions` 查询与其续期判定。凭证语义、拒绝码、响应信封、URL 均不变。
- **外部契约**：无。
- **风险**：若有代码依赖"`Handler()` 一定跑完整 preflight"这一旧行为（例如把 `Handler()` 单独挂在非 v1 的 party 上），行为不变——组合入口仍然完整；风险仅在有人自行新增子 party 挂载点，已由测试扫描拦截。
- **验证**：`rate_limit_enabled=true`、上限 60/分钟/IP 的实测探测，修复前第一个 429 出现在第 30 次 protected 请求，修复后出现在第 58 次（窗口内 3 次前置计数 + 每请求一次），与 1× 计数吻合。
