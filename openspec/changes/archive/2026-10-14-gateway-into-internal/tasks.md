# Tasks

## 1. 搬迁

- [x] 1.1 `git mv app/server/pkg/gateway app/server/internal/gateway`（31 文件，`git diff --cached -M --stat` = 0 insertions / 0 deletions，逐字节重命名）
- [x] 1.2 两个 importer 改 import 路径并按字典序复位：`server/iris.go`（`internal/gateway` 插在 `internal/db` 之后；删除旧 `pkg/gateway` 行，避免同名重复导入编译错）、`internal/router/router.go`（插在 `internal/handler` 之前）
- [x] 1.3 确认包名与目录名一致（`gateway`），无需任何调用点符号改写
- [x] 1.4 确认无环：`internal/middleware` 不 import 网关；`internal/gateway/tracing_test.go` 单向 import `internal/middleware`（测试内，搬迁前后一致）

## 2. 注释与文档路径

- [x] 2.1 9 处跨包注释里的 `pkg/gateway` → `internal/gateway`（middleware 4 个文件、metrics/gateway.go、container/deps.go、internal/pkg/ctxkey.go、internal/pkg/ratelimit/ratelimit.go、router/router.go）
- [x] 2.2 根 `AGENTS.md`：结构树（`internal/` 下新增 gateway 条目、`pkg/` 仅剩 jwt）+ §Gateway 段首句改写（原"独立于 `internal` 的网关子系统"已失真）+ 另 2 处路径引用
- [x] 2.3 根 `AGENTS.md` 三处 `internal/pkg/*` 小标题去掉误导性 `pkg/` 前缀写法（kvstore / eventbus / logger）
- [x] 2.4 `docs/ARCHITECTURE.md`：层级表网关行路径 + 「命名提醒」改写（说明迁移动机与 `pkg/jwt` 例外）
- [x] 2.5 `docs/backend-overview.md`（24 处）、`docs/design/session-auth.md`（8 处）、`internal/{handler,middleware,repository,router}/AGENTS.md`（8 处）路径同步
- [x] 2.6 历史文档保持原样：`agent_briefs/**`、`openspec/changes/archive/**`、`docs/reviews/**`、`docs/plans/**`（是某时点记录，改写等于伪造证据）

## 3. 验证

- [x] 3.1 `go build ./...` = 0
- [x] 3.2 `go vet ./...` = 0
- [x] 3.3 `go test ./...` = 0，17 个含测试包全 `ok`（与搬迁前同一基线，测试文件未改动）
- [x] 3.4 `gofmt -l` 触及文件全清；`internal/gateway/{auth_test.go,ratelimit_test.go}` 两个文件不合格是 HEAD 既有状态（纯对齐空格），为保住"搬迁=逐字节重命名"这一可核对性质本轮不顺手修
- [x] 3.5 `go.mod` / `go.sum` 零改动

## 4. openspec

- [x] 4.1 本 change 的 proposal/tasks
- [x] 4.2 `layered-architecture` 增量：MODIFIED「单向分层依赖链」「各层职责隔离」，ADDED「包位置按消费者边界选择」（含 `pkg/jwt` 例外的如实记录）
- [x] 4.3 `observability` 增量：MODIFIED「关联标识与请求追踪」——路径改 `internal/gateway/tracing.go`，并把 `gateway.go:45/94` 两处行号引用改为符号（行号在上一轮拆 router 后已全部失效）
- [x] 4.4 `api-style` 增量：MODIFIED「统一 JSON 响应结构」（覆盖范围表述里的网关路径）
- [x] 4.5 `backend-service-design` 增量：MODIFIED「权限系统与动态鉴权」（path→permission 映射所在包）
- [x] 4.6 四条增量应用进 `openspec/specs/` 并归档
