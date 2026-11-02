# 贡献指南

欢迎提 issue 和 PR。这个仓库对「改动必须可验证」比较认真：CI 会拦格式、lint、swagger
漂移与测试失败，文档也要跟代码同一次变更提交。

## 开发环境

```bash
# 依赖
pnpm install
cd app/server && go mod download

# 外部依赖：NATS（必须启用 JetStream）
docker run --rm -p 4222:4222 nats:2.10-alpine -js

# 后端
cd app/server && NATS_URL=nats://127.0.0.1:4222 go run main.go -e dev

# 前端（另一个终端，仓库根）
pnpm dev:web
```

需要 Go 1.25+、Node.js 22+、pnpm 10+。完整说明见
[本地开发](https://noelorin.github.io/FeedChamber/install/local/)。

## 分支模型

| 分支 | 用途 | 规则 |
|------|------|------|
| `main` | 发布主干 | 只能 PR 合入，Quality 必须通过 |
| `develop` | 日常集成 | 允许直接 push；短命分支从它切出并合回 |
| `feat/*` `fix/*` `docs/*` … | 短命分支 | kebab-case，合完即删 |

命名形如 `feat/server/space-tree`、`fix/web/login-redirect`。类型前缀取自 commitlint 的
`type-enum`：`feat` `fix` `docs` `style` `refactor` `perf` `test` `chore` `build` `ci`
`revert` `types` `wip`。

## 提交规范

[Conventional Commits](https://www.conventionalcommits.org/)：

```text
feat(server): 帖子列表支持 include_sub 递归子版
fix(web): 登录后 redirect 参数被丢弃
docs(api): 补充分页上限的说明
```

`commitlint` 由 `lefthook` 的 commit-msg 钩子执行。

## 代码约定

### 后端（Go）

- 单向分层：`handler → service → repository → db`，不跨层调用
- 写操作 `POST` + 动作后缀；响应统一 `pkg.Success` / `pkg.Fail` / `pkg.HandleError`
- 错误码只在 `internal/pkg/errors.go` 新增
- 新服务显式接线到 `server/wiring.go`，漏了会编译失败
- 提交前：`gofmt`、`go vet ./...`、`go test ./...`

### 前端（React / TypeScript）

- 页面 `app/web/app/**`，按域组件 `app/web/components/<domain>/`
- 后台新页面：`app/web/app/admin/<route>/page.tsx` + 菜单登记
  （`components/admin/layout/admin-shell.tsx`）+ 权限登记（`lib/admin/auth/route-access.ts`）
- `pnpm check`（biome + tsc）

## 测试

```bash
cd app/server && go test ./...
pnpm check
bash feedchamber_test.sh -all   # 需要后端在 8080 运行
```

新接口至少覆盖：成功路径、权限不足、参数非法。修 bug 的 PR 请附回归用例。

## 文档

文档是变更的一部分，CI 会检查：

- 改模块行为：更新对应目录的 `AGENTS.md`
- 改 API：`make swag` 后提交 `openapi.json`
- 面向用户的变更：更新 `app/docs` 对应页面

## PR 清单

- [ ] 分支名符合 `<type>/<scope>/<desc>`
- [ ] 提交信息符合 Conventional Commits
- [ ] `gofmt` / `go vet` / `go test` 通过
- [ ] `pnpm check` 通过
- [ ] API 变更已 `make swag` 并提交 `openapi.json`
- [ ] 更新了受影响的 `AGENTS.md` 与文档站页面
- [ ] PR 描述写清「为什么改」与「怎么验证」
