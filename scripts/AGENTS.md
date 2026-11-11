# 仓库脚本（`scripts/`）

**不在编译路径内**的辅助脚本：CI 护栏、阈值校准、可访问性审计、agent 文档检查。

> 上级：[根 AGENTS.md](../AGENTS.md) · CI 接入：[.github/workflows/AGENTS.md](../.github/workflows/AGENTS.md)

## 文件表

| 文件 | 语言 | 作用 | 谁在跑 |
|------|------|------|--------|
| `check-agents-docs.mjs` | Node ESM | **模块文档同步门**：模块代码改了却没改对应 `AGENTS.md` → 报错；`--check-index` 校验根索引块；`--write-index` 重写 | lefthook pre-commit（提示）/ CI（阻断） |
| `pre-push-biome-autofix.sh` | bash | **pre-push 自动修复**：把本次要推的提交里 biome 能修的改掉 → amend → 自行推送 → 中止外层 push。见下节 | lefthook pre-push |
| `check-metrics-exposure.sh` | bash | **护栏**：`/metrics`(9091) 不得发布到宿主机。按服务解析 compose 的 `ports:` 块，抓 `9091:9091`、`127.0.0.1:9091:9091`、`target/published: 9091` 三种形状 | `docker-build.yml`（镜像构建之前） |
| `calibrate-alert-thresholds.sh` | bash | 在**已有真实流量的环境**打印当前 P95/P99/错误率，供人工判断告警阈值该收紧还是放宽。**只读不写**，不改规则文件 | 人工 |
| `axe-audit.mjs` | Node ESM | 登录后逐路由跑 axe-core，输出 JSON + 控制台摘要到 `artifacts/axe` | 人工（需先起 web） |

## pre-push 自动修复（`pre-push-biome-autofix.sh`，2026-11-11）

`git push` 时 biome 自动修掉本次提交里可修的问题，amend 回去，**由脚本自己推送**，然后让外层那次 push 失败。三个反直觉的点，改动前必须知道：

1. **amend 之后外层 push 发的还是旧提交**。git 在 pre-push 之前就固定了要发送的 ref 与对象，于是「只在 hook 里 amend 然后放行」会让远端停在重写前的 sha、本地是新 sha，两边漂移（实测）。所以脚本必须自己 `git push`，并把外层那次中止（`exit 1`）。
2. **`exit 1` 是设计的一部分**，不是失败。终端会看到 `error: failed to push some refs`，但修复已经在远端了。用 `FC_SKIP_AUTOFIX=1 git push` 可完全绕过。
3. **lefthook 的 pre-push 不短路**：同组里前面的 command 失败了，后面的照样跑。所以脚本内**自己再判一次分支名**——否则分支名写错时会先被 amend+推送，再报分支名错误，历史已经改了。分支名规则在 `lefthook.yml` 与脚本里各有一份，改要同步。

跳过条件（一律放行，交给 CI）：工作区有未提交改动（**含未跟踪文件**——用 `--untracked-files=no` 会把写了一半的新文件误判成干净）、detached HEAD、rebase/merge/cherry-pick 进行中、找不到 biome。修复粒度是 `--amend`，因此多提交一起推时修复落在 **tip 提交**，不会回填到引入问题的那一个。

兼容性：macOS 自带 bash 3.2，脚本里**不能用 `mapfile` / `readarray` / 关联数组**。

## 约定

- **零依赖优先**：`check-agents-docs.mjs` 只用 Node 内置模块（`node:fs` / `node:path` / `node:child_process`），不需要 `pnpm install` 就能跑。新增脚本尽量保持这条——CI 里少一层失败面。
- **幂等 + 可重跑**：脚本可以反复跑，产物写在 `artifacts/`（已 gitignore）。
- **失败要可读**：护栏脚本失败时打印「为什么这是问题」与「怎么修」，不要只 `exit 1`。
- **只读或明确声明写**：`calibrate-alert-thresholds.sh` 明确只读；会写文件的脚本要在文件头注释里写清楚写哪儿。
- `.mjs` 走 biome 检查（`scripts/**/*.mjs` 有一条 override 关掉 `noUndeclaredEnvVars`）。

## 改这里要注意

- `axe-audit.mjs` 里硬编码了 `POST_ID` / `USER_ID` 与 `CAPTURE_BASE` 默认 `http://localhost:3001`——换数据库后这些 id 会失效，跑之前先确认。
- `check-metrics-exposure.sh` 的 awk 状态机是**按服务**解析的，改动时确认不会把 postgres 的 `ports` 误判给 backend（历史上这条被专门验证过）。
- 新增 CI 护栏脚本时，把它接进 workflow 并在这里登记；否则没人会跑它。
- `check-agents-docs.mjs` 的 `EXEMPT_FILES` 是**按路径精确匹配**的：重命名根级脚手架文件（如 `commitlint.config.js` → `.mjs`）必须同步这张表，否则该文件一改动就会被当成模块代码，逼你改根 `AGENTS.md`。

## 本文件何时更新

- 新增/删除脚本
- 某个脚本被接入或移出 CI
- 脚本的调用约定（参数、退出码语义）变化

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。
