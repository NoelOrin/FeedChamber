## Why

限流有两份实现：`internal/pkg/ratelimit.Limiter.Allow`（SetNX 起窗 + Incr 计数 + 比较）与
`pkg/gateway/ratelimit.go` 里内联的同一段算法。两者除了代码重复，还各自决定键形状
（`rl:<ip>:<policy>` vs `rl:ratelimit:<ip>`）、各自调 `RatelimitRejectionsTotal` /
`RatelimitFailopenTotal`、各写各的日志（一个走 iris app logger，一个走 logger.Global），
并且全局策略的 `name` 标签值是常量 `gateway`，与端点策略混在同一标签里——observability
梳理时记为「两套限流实现并存，看板与告警要同时看两处」。

## What Changes

- `internal/pkg/ratelimit` 成为唯一引擎：新增 `Limiter.Check(ctx, policy, key, limit, window) Outcome`
  （`Allowed`/`Denied`/`FailOpen`），把指标发出与带 trace 的 fail-open 日志收在引擎内；
  `EndpointKey`/`GlobalKey` 单点定义两种键形状（形状与旧实现逐字节相同，不重置在计数的窗口）；
  原 `Allow`/`AllowStr` 收敛（`AllowStr` 是零调用者的透传包装，删除）。
- `internal/middleware.RateLimit` 与 `pkg/gateway.RateLimiter.Middleware` 改为调用 `Check`，
  只保留"如何对待被拒请求"这一点差异（前者 `pkg.Fail` + `StopExecution`，后者返回 bool 给流水线）。
  网关侧不再持有 `kvstore.Store`，改为持有一个 `*ratelimit.Limiter`；`nil` KV 由引擎内部按放行处理。
- 全局策略的 `name` 标签值由 `gateway` 改为 `global`（无查询按该值过滤：告警规则用
  `increase(...)`/`absent(...)` 不带标签，看板用 `sum by (name)`，因此只新增一条更清晰的时间序列名）。
- 新增 `internal/pkg/ratelimit/ratelimit_test.go`：键形状钉住、超限计数、fail-open 记一次且
  不计为拒绝、无 KV 时放行；指标读取走 `Registry.Gather()`，避免为一个测试引入
  `prometheus/client_golang/prometheus/testutil` 的传递依赖。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `observability` — 限流两个指标族的埋点位置由"网关内联"改述为"`Limiter.Check` 唯一发出点"；
  新增「限流只有一个引擎」要求（含失败语义一致、端点预算不占全局配额两个场景）。

## Impact

- **行为**：不变。两类策略的键形状、限额、超限判定与响应（429 + `{code:4001}`）逐字保持；
  实测复照：global 60/分钟/IP 时 protected 第一个 429 仍在 ~60 次（统一前一次测量为 58，
  差值来自窗口内前置请求数不同），per-endpoint `login` 10/分钟仍在第 10 次命中
  （= 1 次前置 + 每请求一次）。`orinforum_test.sh -all` 113/113。
- **代码**：网关少约 30 行内联窗口逻辑；限流算法从 2 份变 1 份。
- **不受影响**：路由上的 12 个命名限额、`rate_limit_enabled` 开关语义、看板与告警查询。
