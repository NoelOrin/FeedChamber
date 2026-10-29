# CI 工作流（`.github/workflows/`）

GitHub Actions。四个 workflow，分工明确：**质量门（几分钟）与镜像构建（约 40 分钟）并行**，互不阻塞。

> 上级：[根 AGENTS.md](../../AGENTS.md) · 脚本：[scripts/AGENTS.md](../../scripts/AGENTS.md) · 分支模型：[docs/branching.md](../../docs/branching.md)

## 文件表

| 文件 | 触发 | 作用 |
|------|------|------|
| `quality.yml` | push/PR 到 `main`、`develop`；手动 | **静态门禁**：backend（gofmt 漂移 / `go vet` / `go test` / swagger 是否过期）、frontend（biome）、typecheck（共享包 build + `tsc`）、docs（AGENTS.md 索引与同步） |
| `docker-build.yml` | `v*` tag、`develop` push、手动 | 构建两个镜像推到 GHCR（供 Dokploy `image:` 拉取）。**构建前先跑 `scripts/check-metrics-exposure.sh`** |
| `mobile-build.yml` | 手动（`workflow_dispatch`，可选平台/构建类型） | Android APK/AAB + iOS IPA。**本期挂起**（vinext 静态导出与 SSR 互斥，见 `app/web/AGENTS.md`） |
| `MOBILE_CI_README.md` | — | 移动端 CI 说明（非 workflow） |

## quality.yml 的四个 job

| job | 跑什么 | 为什么在这 |
|-----|--------|-----------|
| `backend` | `gofmt -l`（只列不改）、`go vet ./...`、`go test ./...`、`swag` + `make swag-check` | gofmt 用 `-l` 保证永不替作者改文件，只让漂移暴露在 PR；swagger 产物是提交进仓库的，忘记 `make swag` 不会编译报错 |
| `frontend` | `pnpm exec biome check .` | 单条命令覆盖格式化 + import 组织 + lint；`biome.json` 已排除构建产物 |
| `typecheck` | `turbo run build --filter='./packages/*'` 然后 `app/web` 的 `tsc -p tsconfig.check.json` | **共享包必须先出 dist**，否则 web 解析不到 `@orinforum/*` 的类型 |
| `docs` | `node scripts/check-agents-docs.mjs --check-index` + PR 时 `--base` 校验模块文档同步 | 模块改了代码却没改 `AGENTS.md` 会让文档长期脱节，且没有任何编译错误提醒 |

## 历史背景（别把门禁当摆设）

仓库此前**没有任何 lint / 格式化 / 测试门禁**，于是 54 个 gofmt 漂移文件与 207 个 biome 错误一路提交进主干（2026-10-28 一次性清理）。`quality.yml` 就是那道门——不要因为「这次改动很小」就跳过它。

## 改这里要注意

- **`quality.yml` 的 `on` 分支列表**与 `docs/branching.md` 的模型一致（`main` + `develop`）；改分支模型时两边同步。
- **不要在 quality 里加 40 分钟级任务**：镜像构建是独立 workflow，故意并行。
- **新增护栏脚本要接进 workflow 并在 `scripts/AGENTS.md` 登记**，否则没人会跑。
- **`docs` job 的 `--base` 用 `origin/${{ github.base_ref }}`**：需要 `fetch-depth: 0` 或至少取到 base 分支，否则 diff 拿不到基线。
- `concurrency` 已按 `${{ github.ref }}` 分组并 `cancel-in-progress`，新增 workflow 沿用这个模式避免堆积。
- **Dokploy 触发那一步必须自己说明失败原因**：`curl -f` 只留下 `curl: (22) error: 400`，所以现在自己取 `%{http_code}`、把响应体落盘打印，再按状态码给处置建议。Dokploy 源码里这两条是硬判定：`400 + "Automatic deployments are disabled for this compose"` = 该 compose 的 **Auto Deploy 开关没开**（token 是有效的）；`404 Compose Not Found` = refresh token 不认（应用重建过要重新复制 hook URL）。别用 `continue-on-error` 把它盖掉——部署没发生还显示绿比红更糟。

## 本文件何时更新

- 新增/删除 workflow 或 job
- 触发条件（分支、tag、手动）变化
- 门禁命令变化（gofmt / biome / tsc / swag-check / docs 检查）
