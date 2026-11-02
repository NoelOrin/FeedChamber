---
title: 贡献指南
description: 分支模型、提交规范、本地验证与 PR 检查清单
---

欢迎提 issue 和 PR。这个仓库对"改动必须可验证"比较认真：CI 会拦格式、lint、swagger
漂移和测试失败，文档也要跟代码同一次变更提交。

## 先跑起来

```bash
pnpm install                 # 前端 workspace
cd app/server && go mod download
```

后端需要外部 NATS（JetStream），没有就先起一个：

```bash
docker run --rm -p 4222:4222 nats:2.10-alpine -js
```

完整步骤见 [本地开发](/install/local/)。

## 分支模型

| 分支 | 用途 | 规则 |
|------|------|------|
| `main` | 发布主干 | 只能 PR 合入，Quality 必须通过 |
| `develop` | 日常集成 | 允许直接 push；短命分支从它切出、合回它 |
| `feat/*` `fix/*` `docs/*` … | 短命分支 | kebab-case，合完即删 |

命名形如 `feat/server/space-tree`、`fix/web/login-redirect`。类型前缀必须取自
commitlint 的 `type-enum`：`feat` `fix` `docs` `style` `refactor` `perf` `test`
`chore` `build` `ci` `revert` `types` `wip`。

## 提交规范

Conventional Commits，例：

```text
feat(server): 帖子列表支持 include_sub 递归子版
fix(web): 登录后 redirect 参数被丢弃
docs(api): 补充分页上限的说明
```

`commitlint` 在 `lefthook` 的 commit-msg 钩子里执行；`wip/*` 分支永不进主干。

## 代码约定

### 后端（Go）

- 分层单向：`handler → service → repository → db`，不允许跨层
- 写操作 `POST`、RPC 路径带动作后缀；响应统一走 `pkg.Success` / `pkg.Fail` / `pkg.HandleError`
- 错误码只在 `internal/pkg/errors.go` 里新增，不自造号段
- 新增服务要显式接线：`server/wiring.go` 映射进 `handler.New`，漏了会编译失败
- 格式化：`gofmt`；提交前 `go vet ./...` 与 `go test ./...`

### 前端（React / TypeScript）

- 页面在 `app/web/app/**`，组件按域放 `app/web/components/<domain>/`
- 后台新页面：建 `app/web/app/admin/<route>/page.tsx`，并在
  `components/admin/layout/admin-shell.tsx` 登记菜单、
  `lib/admin/auth/route-access.ts` 登记权限码
- 格式化与 lint：`biome`（根目录 `pnpm check`）

## 测试

```bash
# 后端单测
cd app/server && go test ./...

# 前端 lint / 类型
pnpm check

# 全量 API 回归（先起后端）
bash feedchamber_test.sh -all
```

新增接口至少覆盖：成功路径、权限不足、参数非法。修 bug 的 PR 请附一条回归用例
（单测或 API 脚本里的断言）。

## 文档

代码改动必须同步文档，这一点由 CI 检查，不是建议：

- 改模块行为：更新对应目录的 `AGENTS.md`
- 改 API：`make swag` 后提交 `openapi.json`（否则 `swag-check` 失败）
- 面向用户的变更：更新本站对应页面；文档站构建会校验站内链接与 base 前缀

文档写作约定见 [app/docs/AGENTS.md](https://github.com/NoelOrin/FeedChamber/blob/main/app/docs/AGENTS.md)。

## PR 检查清单

- [ ] 分支名符合 `<type>/<scope>/<desc>`
- [ ] 提交信息符合 Conventional Commits
- [ ] `gofmt` / `go vet` / `go test` 通过
- [ ] `pnpm check` 通过
- [ ] 改了 API 就跑过 `make swag` 并提交 `openapi.json`
- [ ] 更新了受影响的 `AGENTS.md` 与文档站页面
- [ ] PR 描述写清「为什么改」和「怎么验证」

## 安全与行为准则

- 安全漏洞不要开公开 issue，按 [SECURITY.md](https://github.com/NoelOrin/FeedChamber/blob/main/SECURITY.md) 私下报告
- 讨论请遵守 [行为准则](https://github.com/NoelOrin/FeedChamber/blob/main/CODE_OF_CONDUCT.md)
