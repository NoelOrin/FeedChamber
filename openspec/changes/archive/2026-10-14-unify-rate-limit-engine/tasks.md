# Tasks

## 1. 引擎收敛

- [x] 1.1 `Limiter.Check` 落地：SetNX/Incr/比较 + `Denied`/`FailOpen` 判定 + 指标 + `logger.FromContext` 日志
- [x] 1.2 `EndpointKey` / `GlobalKey` 单点定义键形状（与旧实现逐字节相同，避免窗口清零）
- [x] 1.3 删除零调用者的 `AllowStr`，`Allow` 并入 `Check`
- [x] 1.4 `Outcome` 语义注释写明"为什么 fail-open 而不是 fail-close"

## 2. 两个调用方

- [x] 2.1 `internal/middleware.RateLimit`：改用 `EndpointKey` + `Check`，只保留链控制差异
- [x] 2.2 `pkg/gateway.RateLimiter`：不再内联窗口逻辑，改持有 `*ratelimit.Limiter`；`nil` KV 交给引擎处理
- [x] 2.3 全局策略标签值 `gateway` → `global`；`internal/metrics/gateway.go` 注释说明标签值空间

## 3. 测试与度量

- [x] 3.1 新增 `internal/pkg/ratelimit/ratelimit_test.go`（键形状、限额、fail-open 只记一次且不算拒绝、无 KV 放行）
- [x] 3.2 指标断言走 `Registry.Gather()`，不引入 `testutil`（会新增传递依赖与 go.mod 变更）
- [x] 3.3 变异验证：摘掉 `recordFailure` 里的计数，`TestCheckFailOpenAndMetrics` 立刻失败
- [x] 3.4 实测 global 60/分钟/IP：统一后 protected 第一个 429 ≈60 次（与统一前同源测量 58 的差异由窗口内前置请求数解释）
- [x] 3.5 实测 per-endpoint `login` 10/分钟：第 10 次 429，且它挡住后续登录说明端点预算独立生效
- [x] 3.6 `go build/vet/test ./...` 全绿（17 包），`orinforum_test.sh -all` 113/113

## 4. 文档

- [x] 4.1 `internal/middleware/AGENTS.md` RateLimit 行：说明与网关全局上限共用引擎、差别在策略来源
- [x] 4.2 `internal/metrics/AGENTS.md` gateway 行：`name` 标签值空间说明
- [x] 4.3 根 `AGENTS.md`：Rate Limiter 段与项目结构行
- [x] 4.4 `docs/backend-overview.md`：§9 基础设施行、§13.1 新增已修条目、§13.2 移除并存条目并重编号
