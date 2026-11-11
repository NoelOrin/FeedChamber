# 实测基线（文档数字唯一来源）

> 实测日期：**2026-11-11**（Asia/Shanghai） · 工作分支：`develop` · HEAD：`e72dce7`
> 本文件是 [AGENTS.md](../AGENTS.md) / [ARCHITECTURE.md](ARCHITECTURE.md) / [backend-overview.md](backend-overview.md) / [system-map.md](system-map.md) 及 [openspec/specs/](../openspec/specs/) 中所有可计数声明的**唯一数据来源**。
> 文档里写「N 个文件 / M 行 / K 条路由」前，先按本文件的命令重跑一遍；改了数字就改本文件，不要在各处各写一套。
> 口径统一为**生产文件**（`*.go` 且排除 `*_test.go`），除非该行显式标注「含测试」。

## 复现命令

在仓库根或 `app/server` 下执行：

```bash
# ① 目录规模（生产文件 / 生产行数 / 测试文件）
cd app/server
for d in internal/service internal/handler internal/repository internal/model \
         internal/gateway internal/router internal/middleware internal/container \
         internal/pkg internal/metrics internal/mcp internal/plugin server cmd; do
  fs=$(ls $d/*.go 2>/dev/null | grep -v '_test.go')
  printf '%-24s %3d 文件 %6d 行  %2d 测试\n' "$d" \
    "$(echo "$fs" | grep -c .)" "$(cat $fs 2>/dev/null | wc -l)" \
    "$(ls $d/*_test.go 2>/dev/null | wc -l)"
done

# ② 路由注册条目（生产文件口径）
grep -rho '\.Post(' $(ls internal/router/*.go | grep -v _test) | wc -l   # POST
grep -rho '\.Get('  $(ls internal/router/*.go | grep -v _test) | wc -l   # GET
ls internal/router/routes_*.go | wc -l                                    # routes_*.go

# ③ AutoMigrate 模型数
grep -oE 'model\.[A-Za-z]+\{\}' server/iris.go | sort -u | wc -l

# ④ 权限码（编译期真值，避免正则误判）
#    DefaultPermissions 条目数 = 62；全文件唯一权限码字面量 = 61；PermXxx 常量 = 63
grep -cE '^\s*Perm[A-Za-z]+\s*=\s*"' internal/model/permission.go

# ⑤ XxxHandler 接口数
grep -rhoE '^type [A-Za-z]+Handler interface' internal/handler/*.go | grep -v _test | wc -l

# ⑥ EventBus 事件常量数
grep -cE '^\s*Event[A-Za-z]+\s*=' internal/pkg/eventbus/events.go

# ⑦ openapi 规模（仓库根）
python3 -c "import json;d=json.load(open('openapi.json'));print(len(d['paths']), sum(1 for v in d['paths'].values() for m in v if m in ('get','post','put','patch','delete')))"

# ⑧ 模块文档数（仓库根）
git ls-files '**/AGENTS.md' | grep -v '^AGENTS.md$' | wc -l
```

> **并发改动提示**：2026-11-11 11:24–11:29 本工作区出现了非本次文档更新所致的代码改动（`middleware/middleware.go` 的 `bodyLimitWriter` 改为不缓冲流式透传、新增 `middleware/notfound.go` + 2 个测试、`server/iris.go` 启用 `WithFireMethodNotAllowed` 与 404/405 统一信封）。本表的 `internal/middleware`（10/799/9）与 `server`（7/1755/2）已按改动后的实测值更新；路由数、模型数、权限码数不受影响。引用本表前若工作区又有改动，请按上方命令重跑。

> 口径陷阱：`grep -rho '\.Post(' internal/router/*.go | wc -l` 会把 `*_test.go` 里的 19 处 `.Post(` 一起算进去，得到 **319**；生产口径是 **300**。写文档必须用后者。

## 2026-11-11 实测值

### 后端目录规模（生产文件 / 生产行数 / 测试文件）

| 目录 | 生产文件 | 生产行数 | 测试文件 |
|---|---:|---:|---:|
| `internal/service` | 91 | 16525 | 54 |
| `internal/handler` | 49 | 10879 | 7 |
| `internal/repository` | 106 | 6942 | 12 |
| `internal/model` | 60 | 3613 | 2 |
| `internal/gateway` | 18 | 3918 | 13 |
| `internal/router` | 13 | 975 | 2 |
| `internal/middleware` | 10 | 799 | 9 |
| `internal/container` | 4 | 715 | 2 |
| `internal/metrics` | 16 | 828 | 4 |
| `internal/mcp` | 9 | 1688 | 4 |
| `internal/plugin` | 9 | 2578 | 8 |
| `internal/authz` | 1 | 216 | 1 |
| `server` | 7 | 1755 | 2 |
| `cmd` | 4 | 676 | 0 |
| `internal/pkg`（根，不含子包） | 10 | 527 | 2 |

### 其他关键计数

| 项 | 实测值 | 说明 |
|---|---:|---|
| 路由注册 | **300 POST + 36 GET = 336 条**（另有 1 条 `OPTIONS /mcp`） | 生产文件口径；`routes_*.go` 共 **12** 个 + `router.go` |
| 唯一 (method, 相对路径) | 250 | party 前缀无法静态还原，仅作参考 |
| `openapi.json` | **332 路径 / 333 操作**（297 POST + 36 GET） | 唯一双方法路径 `/oauth/authorize`（GET+POST） |
| `AutoMigrate` 模型 | **73** | `server/iris.go` 中 `model.Xxx{}` 去重 |
| `DefaultPermissions` 条目 | **63**（编译期真值，唯一码 63） | `internal/model/permission.go`（2026-11-11 补入 `admin:metrics`） |
| 全文件权限码字面量唯一值 | 61 | 全部已在 `DefaultPermissions`；`PermXxx` 常量 63 个 |
| `PermXxx` 常量 | 63 | 多出的 2 个不在 `DefaultPermissions` |
| `XxxHandler` 接口 | **45** | `internal/handler/*.go` |
| EventBus 事件常量 | **14**（含 `EventUserRegistered` 历史别名） | `internal/pkg/eventbus/events.go` |
| 前端 service 模块 | 49 个非 test 文件 / 311 个唯一 (path,method) | `packages/api-client/src/services/` |
| 后台页面 | **42 个 `page.tsx`** | `app/web/app/admin/**` |
| 模块 AGENTS.md | **46** | `git ls-files '**/AGENTS.md'` 排除根文档 |
| Go / 包 | `go 1.25.0` / `go test ./...` **25 个 ok 包、0 FAIL**；`go vet ./...` 退出码 0 | `go build ./...` 退出码 0 |

### 前端拓扑（2026-11-11 现状）

- **只有一个前端工程**：`app/web`（React 19 + vinext App Router）。
- 管理后台是 `app/web/app/admin/**` 的 `/admin` 段，与前台同构建、同 origin、纯 CSR（客户端挂载门挡住 SSR），复用 `components/ui/*` 的 shadcn/Radix 原语 + `@tanstack/react-table`。
- **不存在 `app/dashboard` 工程**（旧 Solid/React SPA 已删除）；`app/web/package.json` 不含 antd 依赖，源码中 "antd" 仅出现在迁移注释里。
- `@feedchamber/tokens` 的 `shadcn.css` 由 `app/web/app/globals.css` 引用；`tokens/antd` 产物为历史兼容，无消费方。

## 历史快照（保留供追溯，勿当作现状）

| 日期 | 路由 | service | handler | AutoMigrate | 权限码 | 来源 |
|---|---|---|---|---|---|---|
| 2026-10-27 | 253（223 POST + 30 GET） | 80 / 9876 | 42 / 7716 / 38 接口 | 55 | 54 | `backend-overview.md` 旧版 |
| 2026-11-09 | 351（315 POST + 36 GET） | 91 / 16094 | 48 / 10684 | 73 | — | `openspec/changes/2026-11-09-backend-domain-consolidation/proposal.md` |

> 两个历史快照互相也不一致（351/315 与本次生产口径 336/300），原因是计数口径（是否含测试文件、是否含 app 根端点）不同，且代码在持续演进。**引用现状一律以本文件 2026-11-11 表为准**。
