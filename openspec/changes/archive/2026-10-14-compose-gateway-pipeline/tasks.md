# Tasks

## 1. 网关分段

- [x] 1.1 `gateway.go` 新增 stage 标记（`ctxGatewayStages` + `ranStage`/`markStage`）与 `identityPresent(ctx)`
- [x] 1.2 抽出内部 `preflight(ctx, next)` / `enforce(ctx, next)`，公开 `Preflight()` / `Enforce()`
- [x] 1.3 `Handler()` 改为 `preflight → enforce` 组合；删除 `OptionalHandler()`
- [x] 1.4 Enforce 在身份已由 preflight 解析时跳过二次凭证查询；API Key 分支保持完整解析

## 2. 挂载点

- [x] 2.1 `router.go`：`v1.Use(gw.Preflight())`、`protected.Use(gw.Enforce())`
- [x] 2.2 相关注释与包文档中的段名同步（`router.go` 包注释、`reg` 字段注释）

## 3. 测试

- [x] 3.1 `pkg/gateway/pipeline_test.go`：5 次 protected 请求 → 限流计数恰好 5
- [x] 3.2 `Preflight` + 误挂 `Handler()` 仍恰好 5 次（幂等兜底）
- [x] 3.3 匿名 protected 仍 401/1001；死凭证仍 401/1002；`Bearer ak_*` 落到 API Key 分支（未启用 → 403/1013）
- [x] 3.4 变异验证：把 `ranStage` 改成恒 false 后，3.2 立刻失败（计数翻倍），确认断言非空过
- [x] 3.5 `middleware_chain_test.go` 扫描规则升级为禁止 `routes_*.go` 出现任何 `Use(gw.`；复刻测试断言 protected 链含 enforce 段
- [x] 3.6 `go build ./... && go vet ./... && go test -count=1 ./...` 全绿（16 个包 ok）

## 4. 端到端度量

- [x] 4.1 修复前基线：`rate_limit_enabled=true`、上限 60/分钟/IP，protected `/dashboard` 第 **30** 次请求即 429（≈2× 计数，2×30=60 吻合）
- [x] 4.2 修复后同探测：第一个 429 落在第 **58** 次（窗口内 3 次前置公开计数 + 每请求一次），与 1× 计数吻合
- [x] 4.3 `orinforum_test.sh -all` 113/113

## 5. 文档与规范

- [x] 5.1 `router/AGENTS.md`：两条缺陷合并为"已修复"叙事 + 可组合段说明；约定条目改段名
- [x] 5.2 `middleware/AGENTS.md`：链路表 protected 行改为 Enforce，并说明 trace/限流属 Preflight；MaintenanceGate 行修正为 5005
- [x] 5.3 根 `AGENTS.md`：依赖链图、路由组织段、网关入口清单
- [x] 5.4 `docs/ARCHITECTURE.md`：分层图与决策速查
- [x] 5.5 `docs/backend-overview.md`：§4.1 入口表、§6 挂载链、§13.1 新增第 24 条、§13.2 移除已修条目并重编号
