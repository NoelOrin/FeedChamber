# agents-docs Specification

## Purpose
定义模块文档（各目录的 `AGENTS.md`）的覆盖范围、内容结构与索引方式，并钉死「模块代码变更必须同步其文档」这条增量约束及其门禁强度——让文档漂移在 CI 上暴露，而不是靠人记得。
## Requirements
### Requirement: 每个模块目录都有 AGENTS.md

每一个承载代码的模块目录 MUST 提供一份 `AGENTS.md`，说明它的职责、文件构成、必须遵守的约定、改动风险点，以及该文件自身的更新时机。

「模块目录」指满足以下任一条件的目录：包含 ≥5 个生产代码文件；或包含 1 个以上子系统；或承载跨模块契约（如 `openspec/`、`deploy/`、`scripts/`、`.github/workflows/`）。路由树（`app/web/app/`）、组件域（`app/web/components/`）、库层（`app/web/lib/`）等大块 MUST 各自成篇，不得只由上级文档一行带过。

文档 MUST 包含以下六节（标题可调整措辞，语义不得缺）：上级与相邻文档链接、职责、文件表、约定、改这里要注意、本文件何时更新。

#### Scenario: 新增一个后端层目录

- WHEN 在 `app/server/internal/` 下新增一个承载生产代码的目录
- THEN MUST 在该目录内提供 `AGENTS.md`，并在根 `AGENTS.md` 的模块文档索引中可见

#### Scenario: 前端大块不得只有顶层文档

- WHEN `app/web` 下的 `app/`、`components/`、`lib/` 任一目录的文件数超过 20
- THEN MUST 为该目录提供独立 `AGENTS.md`，不得仅由 `app/web/AGENTS.md` 覆盖

### Requirement: 模块文档索引是单一入口

根 `AGENTS.md` MUST 包含一个由 `<!-- AGENTS-INDEX:BEGIN -->` 与 `<!-- AGENTS-INDEX:END -->` 包裹的模块文档索引块，列出全部模块文档的目录、路径与标题。

该块 MUST 由 `node scripts/check-agents-docs.mjs --write-index` 生成，MUST NOT 手工编辑。块内容 MUST 与仓库中实际存在的 `AGENTS.md` 集合完全一致：多一个、少一个、或链接指向不存在的文件，都视为失效。

#### Scenario: 新增或删除模块文档后未更新索引

- WHEN 新增或删除一份 `AGENTS.md`，但未重跑 `--write-index`
- THEN `node scripts/check-agents-docs.mjs --check-index` MUST 以非零码退出，并列出「索引中缺少」与「索引中多余」的具体路径

#### Scenario: 索引链接失效

- WHEN 索引块中出现指向不存在文件的链接
- THEN `--check-index` MUST 以非零码退出

### Requirement: 模块代码变更必须同步其 AGENTS.md

在同一个变更（commit 或 PR）内，若修改了某模块的代码文件，MUST 同时修改该模块的 `AGENTS.md`。

模块归属规则 MUST 是「被改文件归属到最深的、含 `AGENTS.md` 的祖先目录」——这样新增模块目录只要建了文档就自动纳入，无需维护路径映射表。

以下内容不计入「模块代码」：文档（`.md`）、生成物（`openapi.json`、`app/server/docs/`、`packages/tokens/dist/`）、依赖与构建产物（`node_modules/`、`dist/`、`.turbo/`、`.output/`）、根级脚手架配置（`package.json`、`pnpm-workspace.yaml`、`turbo.json`、`biome.json`、`lefthook.yml`、`tsconfig*.json`）。

检查 MUST 提供两种强度：

- 本地 pre-commit MUST 只提示、不阻断（`--staged`，始终 exit 0）；
- CI MUST 在 PR 上阻断（`--base origin/<base_ref>`，exit 1）。

#### Scenario: 改了模块代码却没改文档

- WHEN 一个 PR 修改了 `app/server/internal/gateway/auth.go`，但未修改 `app/server/internal/gateway/AGENTS.md`
- THEN CI 的 `docs` job MUST 失败，并输出「模块文档路径 + 相关改动文件」

#### Scenario: 同时改了代码与文档

- WHEN 同一个 PR 既修改了 `app/server/internal/gateway/auth.go`，也修改了 `app/server/internal/gateway/AGENTS.md`
- THEN `--base` 检查 MUST 通过

#### Scenario: 本地提交不被阻断

- WHEN 开发者在本地暂存了模块代码改动但未更新文档，并执行 `git commit`
- THEN pre-commit 钩子 MUST 打印提示，但 MUST NOT 阻止提交

### Requirement: 层表不得指向不存在的文档

`docs/ARCHITECTURE.md` 与 `docs/system-map.md` 的「约定文档」列 MUST 指向真实存在的 `AGENTS.md` 文件，MUST NOT 使用「见本文 §X」这类无文档的替代写法。

#### Scenario: 某层缺少文档时先补文档

- WHEN 某层在层表中只有「见本文 §X」而没有独立文档
- THEN MUST 先补该层的 `AGENTS.md`，再把层表改为指向它

### Requirement: 文档中的模块清单以代码为准

`AGENTS.md` 与 `docs/` 中出现的数量与路径（文件数、行数、权限码数、菜单登记点、目录是否存在）MUST 与代码实际一致；发现不一致时 MUST 修正文档，MUST NOT 反向修改代码去迁就文档。

#### Scenario: 菜单登记点写错

- WHEN 文档声称后台菜单在某文件登记，而该文件不存在
- THEN MUST 修正为真实的登记点（`components/admin/layout/admin-shell.tsx` 的 `menuItems`），并在该模块文档中记录易混点

