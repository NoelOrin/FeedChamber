## Why

`app/server/pkg/gateway` 自 2026-06-18 建包起就住在 `pkg/`（`2386d8a`「add pkg/gateway package with interface-based deps」），但它的消费者从头到尾只有同模块内的两个文件：`server/iris.go` 与 `internal/router/router.go`（各 4 处引用）。Go 的 `internal/` 约束的正是"谁能 import"，模块内自消费的包放在 `pkg/` 不换来任何可见性收益，只换来一件事：`app/server/internal/pkg`（通用工具）与 `app/server/pkg`（`jwt` + `gateway`）两个 `pkg` 同名不同物——这一条已作为遗留风险写在 `docs/backend-overview.md` §13.2 第 26 项，且 `internal/gateway` 反向引用 `internal/pkg/clientip.go` 时读代码的人要先猜一次前缀。

本轮是"取消网关层的独立位置"，不是重写网关：包名仍是 `gateway`，文件划分、符号、行为都不动。

## What Changes

- `app/server/pkg/gateway/` → `app/server/internal/gateway/`（31 个 `.go` 文件整体搬迁：18 个生产文件 3649 行 + 13 个测试文件 3513 行）。文件内容逐字节不变，`git mv` 后 rename 检测成立。
- 两个 importer 的 import 路径改为 `orinforum/internal/gateway`：`server/iris.go`、`internal/router/router.go`。
- 9 处跨包注释里的路径字样同步（`internal/middleware/{auth,deps,ratelimit,trace}.go`、`internal/metrics/gateway.go`、`internal/container/deps.go`、`internal/pkg/{ctxkey,ratelimit/ratelimit}.go`、`internal/router/router.go`）。
- 新增「包位置按消费者边界选择」要求（layered-architecture），把"只有模块内消费者的包 MUST NOT 住 `pkg/`"写成规则，并如实记录 `pkg/jwt` 是此刻唯一的例外及其待决状态。
- 文档路径与措辞：根 `AGENTS.md`（结构树、§Gateway 段首句"独立于 `internal` 的网关子系统"已失真）、`docs/ARCHITECTURE.md` 命名提醒、`docs/backend-overview.md`（24 处）、`docs/design/session-auth.md`（8 处）、`internal/{handler,middleware,repository,router}/AGENTS.md`。
- 顺带修正 AGENTS.md 里三处把 `internal/pkg/*` 写成 `pkg/*` 的小标题（`kvstore`/`eventbus`/`logger`），与本轮的命名收敛同源。

## Impact

- **行为**：零变化。路由、鉴权、限流、响应、指标全部不变；`go build/vet/test ./...` 全绿（17 个含测试包，与搬迁前同一基线），测试文件未改一行。
- **编译面**：`internal/gateway` 只 import `internal/*` 与 `pkg/jwt`；`internal/middleware` 不 import 网关（仅 `internal/gateway/tracing_test.go` 在测试里用到 `middleware.Trace`），故搬迁不引入新环。
- **外部可见性**：Go 的 `internal/` 限制的是"非 `orinforum` 模块不得 import"。仓库内没有任何模块外消费者，因此实际影响为 0；若将来要做可复用库，那时再移回 `pkg/` 才是有依据的。
- **不在本轮范围**（已实测确认，非遗漏）：
  - 网关自持 `*gorm.DB` 与 13 个文件直接 import `internal/model`（21 处）——数据访问下沉 repository 是另一件事，方向与本轮相反（本轮只搬位置，不改变谁持有 DB 句柄）。
  - `internal/gateway` 与 `internal/middleware` 各自声明了方法集完全相同的 `SettingReader` / `PermissionChecker` / `Ban(ner)Checker`，以及两个平行 `Deps`。合并需要确定接口归属方并新增一条包间依赖边，代价与收益要单独评估。
  - `pkg/jwt` 仍在 `pkg/`（消费者同样全在模块内），按新规则应一并迁入，待确认。
  - `docs/architecture/performance-chain.md`（5 处 `pkg/gateway`）属并行会话在写的文件，未代改。
