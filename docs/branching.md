# 分支管理规范（Branching Strategy）

> 单一可信来源：本文件。变更须经 PR 合入 `main`（或 `develop`，见保护规则）。

## 1. 模型概览

```
main        ── 发布主干（受保护，必须 PR 合入，只收经过验证的发布）
develop     ── 永久长命开发集成线（受保护；**允许直接 push**，日常集成用）
feat/* fix/* … ── 短命分支：从 develop 切出，合回 develop，做完即删
```

- 短命分支只跟 `develop` 打交道：从 `develop` 切出、合回 `develop`。
- `develop` 周期性地通过**发布 PR**升到 `main`，并在 `main` 上打 `v*` tag（`docker-build.yml` 的 `tags: ["v*"]` 触发镜像构建）。
- 紧急 hotfix：从 `main` 切 `fix/*` → 合回 `main` → 同步合回 `develop`（见 §6）。

## 2. 永久分支与保护规则

| 分支 | 角色 | 直接 push | 合入方式 | CI |
|------|------|-----------|----------|----|
| `main` | 发布主干 | ❌ 禁止 | 仅 PR + Quality 通过 | Quality + Docker 构建 + 部署（仅 `v*` tag 或显式触发） |
| `develop` | 开发集成线 | ✅ 允许 | 短命分支直接 PR，或维护者直接 push | Quality + Docker 构建 + 部署（`develop` push 触发） |

> 保护规则在 GitHub 仓库 Settings → Branches 配置，命令见 §7。`main` 必须勾选
> "Require a pull request before merging" + "Require status checks to pass"（Quality）。
> `develop` **不勾选** PR 强制，仅保留 Quality 状态检查作为建议门禁。

## 3. 分支命名

`<type>/<scope?>/<short-desc>`，kebab-case，`<type>` 取自 commitlint 的 `type-enum`：

`feat` `fix` `docs` `style` `refactor` `perf` `test` `chore` `build` `ci` `revert` `types` `wip`

- `<scope>`（可选）：模块名，建议取值 `server` `web` `admin` `shared` `deploy` `mobile`。
- `<short-desc>`：kebab-case 短描述（动词优先，如 `login-redirect`）。

**放行（不做名校验）的分支**：`main`、`develop`。

**示例**：
- `feat/web/login-redirect`
- `fix/server/space-tree`
- `refactor/shared/date-format`
- `chore/ci/sync-branches`
- `docs/branching-spec`
- `wip/experiment-branch`（个人草稿，**永不进主干**）

**反例（禁止）**：
- ❌ `feature/login`（前缀必须是 commitlint 的 `feat`）
- ❌ `Fix_Server_Bug`（大小写与下划线，须 kebab-case）
- ❌ `my-branch`（无类型前缀）
- ❌ `develop-fix-x`（必须用斜杠分隔的 `<type>/...` 形态）

命名校验由 `lefthook` 的 `pre-push` 钩子执行（见 §5）。

## 4. 工作流

### 日常开发（指向 develop）
```bash
git switch develop && git pull
git switch -c feat/web/login-redirect
# ... 开发，commit 遵循 Conventional Commits（commitlint 已配置）...
git push -u origin feat/web/login-redirect
# 在 GitHub 开 PR 到 develop；或直接 push 回 develop（允许）
git switch develop && git merge --no-ff feat/web/login-redirect
git branch -d feat/web/login-redirect
```

### 发布（develop → main）
```bash
git switch develop && git pull
# 开 PR：base=main，compare=develop；标题形如 "chore(release): v1.4.0"
# Quality 通过后合入 main
git switch main && git pull
git tag v1.4.0 && git push origin v1.4.0   # 触发 Docker 镜像构建
```

### 紧急 hotfix（main → main，再回 develop）
```bash
git switch main && git pull
git switch -c fix/server/auth-bypass
# ... 修复 ...
git push -u origin fix/server/auth-bypass
# 开 PR 到 main（必须），Quality 通过合入
git tag v1.4.1 && git push origin v1.4.1
# 同步回 develop
git switch develop && git merge --no-ff main && git push
```

## 5. 分支名校验钩子（lefthook）

钩子不随仓库下发，**克隆后要手动装一次**（`pnpm install` 不会代劳，根 `package.json` 也没有 `prepare`）：

```bash
pnpm exec lefthook install   # 写入 .git/hooks/{pre-commit,commit-msg,pre-push}
```

判断是否装过：`git config --get core.hooksPath` 为空、`.git/hooks` 里没有 `pre-commit`，就是没装。
装好后本地有三道门：`pre-commit` 对暂存文件跑 `biome check`（与 CI 的 Quality / Biome job 同一条命令，漏跑就会把格式错误直接推上去）、`commit-msg` 跑 commitlint、`pre-push` 校验分支名。

`lefthook.yml` 的 `pre-push` 会校验当前分支名是否符合 §3 命名，或在放行清单内。
不合规则拒绝 push，并提示正确格式。校验逻辑纯 POSIX，跨平台安全。

```yaml
pre-push:
  commands:
    "branch-name":
      run: |
        branch=$(git rev-parse --abbrev-ref HEAD)
        case "$branch" in
          main|develop) exit 0 ;;
        esac
        if printf '%s' "$branch" | grep -Eq '^(feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert|types|wip)(/[a-z0-9._-]+)+$'; then
          exit 0
        fi
        echo "✖ 分支名 '$branch' 不符合规范 (<type>/<scope?>/<short-desc>)"
        echo "  允许类型: feat fix docs style refactor perf test chore build ci revert types wip"
        echo "  放行分支: main develop"
        exit 1
```

## 6. CI 触发矩阵

| 事件 | `quality.yml` | `docker-build.yml` |
|------|---------------|--------------------|
| push `develop` | ✅ | ✅ 构建 + **触发部署** |
| push `main` | ✅ | ✅ 构建（不部署，除非是 `v*` tag） |
| push `v*` tag | — | ✅ 构建 + 部署 + `latest` |
| PR → develop / main | ✅ | ✅ 仅构建（不推送、不部署） |

`docker-build.yml` 的 `deploy` job 触发条件：`github.ref == refs/heads/develop` 或 `v*` tag。

## 7. GitHub 保护分支设置（命令）

以下命令需仓库 `admin` 权限，由维护者执行（脚本不自动运行）：

```bash
# main：必须 PR + Quality 状态检查通过
gh api -X PUT repos/NoelOrin/Feed Chamber/branches/main/protection \
  -f required_status_checks.strict=true \
  -f required_status_checks.contexts[]=Quality \
  -f enforce_admins=true \
  -f required_pull_request_reviews='{"required_approving_review_count":0}' \
  -f restrictions=null

# develop：不做 PR 强制，仅保留状态检查建议（用空 protection 或仅 dismiss stale）
gh api -X DELETE repos/NoelOrin/Feed Chamber/branches/develop/protection 2>/dev/null || true
# develop 直接 push 即放行；如需禁止 force-push 可单独设：
gh api -X PUT repos/NoelOrin/Feed Chamber/branches/develop/protection \
  -f required_pull_request_reviews=null \
  -f enforce_admins=false \
  -f restrictions=null \
  -f required_status_checks=null \
  -F allow_force_pushes=false 2>/dev/null || true
```

## 8. 当前迁移收尾（migrate/react-vinext → develop/main）

当前仓库默认分支仍是 `migrate/react-vinext`，需收尾到本规范。以下命令**需维护者确认后手动执行**
（涉及改远程默认分支与 push，脚本不会自动运行）：

```bash
# 1. 本地将当前开发线重命名为 develop 并推上去
git branch -m migrate/react-vinext develop
git push -u origin develop

# 2. 基于 develop 的当前 tip 创建 main 并推送
git branch main develop
git push -u origin main

# 3. 在 GitHub 把默认分支改为 main，并保护 main（见 §7）
#    Settings → Branches → 默认分支选 main；再跑 §7 的 gh 命令

# 4. 删除旧的过渡分支（确认 develop/main 都就位后）
git push origin --delete migrate/react-vinext
git branch -d migrate/react-vinext 2>/dev/null || true

# 5. 同步其他远端遗留分支（如 worktree-kvstore-nats、rebuild/dashboard-template）
#    按需合回 develop 或删除：git push origin --delete <branch>
```

> 注意：`origin/HEAD` 当前指向 `migrate/react-vinext`，改完默认分支后 GitHub 会自动更新；
> 本地可用 `git remote set-head origin -a` 重新指向。
