#!/usr/bin/env bash
#
# pre-push 自动修复：把本次要推出去的提交里能被 biome 自动修掉的问题修掉，
# 重新提交（amend）后由本脚本自己推送，然后**故意中止**外层那次 push。
#
# 为什么要「自己推 + 中止外层」而不是直接 amend 完放行：
#   git 在 pre-push 之前就已经算好了要发送的 ref 与对象，amend 之后外层那次
#   push 发送的仍是**旧提交**（实测：远端停在 amend 前的 sha，本地已是新
#   sha，两边漂移）。所以只有本脚本自己发起一次 push，才能把修复送上去；
#   而外层那次必须被中止（exit 1），否则它会把旧提交再推一遍。
#
# 环境开关：
#   FC_SKIP_AUTOFIX=1    完全跳过（急用 / 脚本本身出问题时）
#   内部用 FC_PREPUSH_AUTOFIX=1 防递归，外部不要设
#
# 刻意跳过（一律放行，交给 CI）：
#   - 工作区有未提交改动：amend 会把没写完的东西一起提交进去
#   - detached HEAD / rebase / merge / cherry-pick 进行中：此时 rewrite 会毁掉流程
#   - 找不到 biome
#
# 退出码：0 = 放行外层 push；1 = 已自行推送修复，外层应中止。
#
# 兼容性：macOS 自带 bash 3.2，**不要用 mapfile / readarray / 关联数组**。
# 写文件的地方只有 biome 自身改写的源码与本仓库的 index（git add / amend）。

set -uo pipefail

say() { printf '%s\n' "$*" >&2; }

# 内层自推时会被设置，直接放行避免无限递归。
if [ -n "${FC_PREPUSH_AUTOFIX:-}" ]; then
	exit 0
fi

if [ "${FC_SKIP_AUTOFIX:-}" = "1" ]; then
	exit 0
fi

root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
cd "$root" || exit 0

# detached HEAD 下没有「当前提交」可 amend。
branch=$(git symbolic-ref --quiet --short HEAD 2>/dev/null) || exit 0

# 分支名不合规时不能动历史。
#
# lefthook 的 pre-push **不会短路**：同组里 branch-name 校验失败后，本 command
# 仍会照常执行（实测）。如果这里不自己再判一次，分支名写错时会先被 amend 并
# 推送，然后再报分支名错误——历史已经改了。规则与 lefthook.yml 的 branch-name
# 保持一致，改一处要同步另一处。
case "$branch" in
	main|develop) ;;
	*)
		if ! printf '%s' "$branch" | grep -Eq '^(feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert|types|wip)(/[a-z0-9._-]+)+$'; then
			say "pre-push: 分支名 '$branch' 不合规，跳过 biome 自动修复。"
			exit 0
		fi
		;;
esac

# rewrite 进行中：不要动历史。
for marker in rebase-merge rebase-apply MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD; do
	if [ -e "$(git rev-parse --git-path "$marker")" ]; then
		say "pre-push: 检测到进行中的 git 操作（$marker），跳过 biome 自动修复。"
		exit 0
	fi
done

# 工作区必须干净，否则 amend 会吞掉未提交内容。
# 注意**不要**加 --untracked-files=no：那会让未跟踪文件（最典型的就是写了一半
# 的新文件）导致检查返回空、误判成「干净」。实测这条 bug 会让 amend 把半成品
# 一起提交。已跟踪与未跟踪都要拦。
if [ -n "$(git status --porcelain)" ]; then
	say "pre-push: 工作区有未提交改动（含未跟踪文件），跳过 biome 自动修复，避免把半成品一并 amend。"
	say "pre-push: 处理完后再推，或设 FC_SKIP_AUTOFIX=1 直接推。"
	exit 0
fi

# 修复范围 = 本次真正要推出去的提交（upstream..HEAD）。
upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)
if [ -n "$upstream" ]; then
	base=$(git merge-base HEAD "$upstream" 2>/dev/null || true)
	remote="${upstream%%/*}"
	remote_branch="${upstream#*/}"
else
	# 分支尚无 upstream：只看 HEAD 这一个提交，推到 origin 同名分支。
	base=$(git rev-parse -q --verify 'HEAD~1' 2>/dev/null || true)
	remote="${FC_REMOTE:-origin}"
	remote_branch="$branch"
fi

# 首次提交没有父：用空树当 base，让全部文件参与。
if [ -z "$base" ]; then
	base=$(git hash-object -t tree /dev/null)
fi

# 收集候选文件（bash 3.2：用换行分隔的字符串，不用数组）。
files=""
changed=$(git diff --name-only --diff-filter=ACMR "$base" HEAD || true)
if [ -z "$changed" ]; then
	exit 0
fi
while IFS= read -r f; do
	[ -z "$f" ] && continue
	case "$f" in
		*.js|*.jsx|*.ts|*.tsx|*.mjs|*.cjs|*.mts|*.cts|*.css|*.json) ;;
		*) continue ;;
	esac
	# 只保留工作区里仍然存在的文件（删掉的文件没什么可修的）。
	[ -f "$f" ] || continue
	files="$files$f
"
done <<EOF
$changed
EOF

if [ -z "$files" ]; then
	exit 0
fi

if ! pnpm exec biome --version >/dev/null 2>&1; then
	say "pre-push: 找不到 biome（pnpm exec biome 失败），跳过自动修复。"
	exit 0
fi

# biome 只接受位置参数；用 xargs 传递，避免把文件名拆进多个 argv 时触发 ARG_MAX。
printf '%s' "$files" | tr '\n' '\0' | xargs -0 pnpm exec biome check --write --no-errors-on-unmatched --files-ignore-unknown=true >/dev/null 2>&1 || true

# biome 改了哪些文件：只取工作区相对 index 的改动，且限于候选集。
modified=$(git diff --name-only -- $files 2>/dev/null || true)
if [ -z "$modified" ]; then
	exit 0
fi

count=0
while IFS= read -r f; do
	[ -n "$f" ] && count=$((count + 1))
done <<EOF
$modified
EOF

say "pre-push: biome 自动修复了 $count 个文件，将 amend 进当前提交："
while IFS= read -r f; do
	[ -n "$f" ] && say "  - $f"
done <<EOF
$modified
EOF

# 逐文件 add：文件名可能含空格，不能用 $(...) 展开成 argv。
add_failed=0
while IFS= read -r f; do
	[ -z "$f" ] && continue
	git add -- "$f" || add_failed=1
done <<EOF
$modified
EOF

if [ "$add_failed" -ne 0 ]; then
	say "pre-push: git add 失败，放弃自动修复，请手工处理。"
	exit 0
fi

if ! git commit --amend --no-edit --quiet; then
	say "pre-push: amend 失败，放弃自动修复（工作区可能已被 biome 改写，请检查 git status）。"
	exit 0
fi

new_sha=$(git rev-parse --short HEAD)
say "pre-push: 已 amend 为 $new_sha，正在推送（外层那次 push 会被中止）。"

if ! FC_PREPUSH_AUTOFIX=1 LEFTHOOK=0 git push --quiet "$remote" "HEAD:$remote_branch"; then
	say "pre-push: 自推送失败（$remote $remote_branch）。修复已在本地提交 $new_sha，请手动 git push。"
	# 本地历史已经被 rewrite，外层推旧提交同样不合适，所以仍中止外层。
	exit 1
fi

say "pre-push: 修复已推送到 $remote/$remote_branch。"
say "pre-push: 这次会被 git 报告为 push 失败，属预期——外层发送的是重写前的旧提交，已中止。"
exit 1
