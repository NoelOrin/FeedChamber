# 仓库脚本（`scripts/`）

**不在编译路径内**的辅助脚本：CI 护栏、阈值校准、可访问性审计、agent 文档检查。

> 上级：[根 AGENTS.md](../AGENTS.md) · CI 接入：[.github/workflows/AGENTS.md](../.github/workflows/AGENTS.md)

## 文件表

| 文件 | 语言 | 作用 | 谁在跑 |
|------|------|------|--------|
| `check-agents-docs.mjs` | Node ESM | **模块文档同步门**：模块代码改了却没改对应 `AGENTS.md` → 报错；`--check-index` 校验根索引块；`--write-index` 重写 | lefthook pre-commit（提示）/ CI（阻断） |
| `check-metrics-exposure.sh` | bash | **护栏**：`/metrics`(9091) 不得发布到宿主机。按服务解析 compose 的 `ports:` 块，抓 `9091:9091`、`127.0.0.1:9091:9091`、`target/published: 9091` 三种形状 | `docker-build.yml`（镜像构建之前） |
| `calibrate-alert-thresholds.sh` | bash | 在**已有真实流量的环境**打印当前 P95/P99/错误率，供人工判断告警阈值该收紧还是放宽。**只读不写**，不改规则文件 | 人工 |
| `axe-audit.mjs` | Node ESM | 登录后逐路由跑 axe-core，输出 JSON + 控制台摘要到 `artifacts/axe` | 人工（需先起 web） |

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
