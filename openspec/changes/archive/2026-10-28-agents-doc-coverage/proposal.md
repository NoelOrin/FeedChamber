## Why

仓库已有 22 个模块级 `AGENTS.md`，但覆盖是**不完整的**，而且「模块改了要同步文档」只是口头约定——没有任何机制会发现漂移。

三处具体缺口：

1. **`internal/gateway` 是唯一没有文档的后端层**（31 个文件 / 7625 行，9 个子系统）。`docs/ARCHITECTURE.md` 与 `docs/system-map.md` 的层表里，它的「约定文档」一栏都写着「见本文 §3.2」——这是断链，不是文档。
2. **`app/web` 只有顶层一个 `AGENTS.md`**（152 行），覆盖 243 个文件。`app/`（76 文件）、`components/`（137 文件）、`lib/`（25 文件）三个大块，以及后台壳层、UI 原语、Markdown 编辑器（安全边界）、`lib/auth`（有不可逾越的导入边界）都没有各自的约定文档。`deploy/`、`scripts/`、`.github/workflows/`、`openspec/`、`docs/`、`packages/`（根）同样没有。
3. **文档漂移已经在发生，且没有告警**：根 `AGENTS.md` 让读者去 `components/admin/layout/pro-shell.tsx` 登记后台菜单（该文件不存在，实际是 `admin-shell.tsx` 的 `menuItems`）；`app/web/AGENTS.md` 与 `docs/system-map.md` 都列着 `app/ui-showcase/` 原语对照台（目录已删除）；`app/web/AGENTS.md` 的会话 cookie 名还是迁移前的 `master_access_token` / `master_refresh_token`（现为 `master_session`）；根 `AGENTS.md` 写「seed 53 个权限码」，`ARCHITECTURE.md` 写 54/56，而 `model/permission.go` 的 `DefaultPermissions` 实际是 56 条。

第 3 点是第 1、2 点的成因：**没有门禁的文档必然漂移**。补文档只解决存量，不解决增量。

## What Changes

- **补齐 16 个模块 `AGENTS.md`**：`internal/gateway`、`app/web/app`、`app/web/app/admin`、`app/web/components`、`app/web/components/ui`、`app/web/components/admin`、`app/web/components/markdown-editor`、`app/web/lib`、`app/web/lib/auth`、`packages`、`packages/api-client/src/services`、`deploy`、`scripts`、`.github/workflows`、`openspec`、`docs`。统一模板：上级/相邻链接 → 职责 → 文件表 → 约定 → 改这里要注意 → 本文件何时更新。
- **根 `AGENTS.md` 新增自动生成的「模块文档索引」块**（`<!-- AGENTS-INDEX:BEGIN/END -->` 包裹，列出全部 38 个文档的路径、目录与标题），成为单一入口。块由脚本生成，不手改。
- **修掉三处枢纽文档的断链/漂移**：`docs/ARCHITECTURE.md` 与 `docs/system-map.md` 的 gateway 行改为指向 `gateway/AGENTS.md`；`app/web/AGENTS.md` 与 `docs/system-map.md` 删掉 `ui-showcase` 并纠正 `(app)/` 的鉴权描述；根 `AGENTS.md` 的后台菜单路径改为 `admin-shell.tsx` 的 `menuItems`；权限码数量按实测统一为 56。
- **新增同步门 `scripts/check-agents-docs.mjs`**（零依赖 ESM，只用 Node 内置模块）：
  - 模块归属 = 被改的代码文件归属到**最深的、含 `AGENTS.md` 的祖先目录**。因此新增模块目录只要建了文档就自动纳入，**不需要维护映射表**。
  - `--staged`：本地提示，始终 exit 0；`--base <ref>`：CI 阻断；`--check-index`：索引块与文档集合不一致则 exit 1（并校验链接可达）；`--write-index`：重写索引块。
  - 生成物与第三方目录（`app/server/docs`、`packages/tokens/dist`、`node_modules`、构建产物）以及根级脚手架文件不计入。
- **接入两处**：`lefthook.yml` 的 pre-commit 加一条提示（`|| true`，不阻断本地提交）；`.github/workflows/quality.yml` 新增 `docs` job（`--check-index` 常驻 + PR 上 `--base` 阻断），与既有 gofmt / biome / tsc 门禁同风格。

## Non-goals

- 不把 `docs/` 的散文文档迁进 `AGENTS.md`，也不反过来。分工不变：规范进 `openspec/specs/`，说明性材料留在 `docs/`，模块约定进各层 `AGENTS.md`。
- 不为 `components/` 下每个子目录（21 个）都建文档。只有「文件多 + 有独立约定」的域才建：`ui`、`admin`、`markdown-editor`。其余域在 `components/AGENTS.md` 的子域一览里一行带过。
- 不改本次无关的过期内容（例如 `docs/backend-overview.md` 的发现项编号体系、`orinforum_test.sh` 的用例）。
