# Feed Chamber Web UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Spec:** `docs/superpowers/specs/2026-07-25-ui-redesign-design.md`

**Goal:** 在不改色板 hex 的前提下，把 `app/web` 重做成「克制玻璃态 + 社群时间线」视觉体系：先统一全局表面与基础组件，再壳层、业务组件、页面，每批截图 review。

**Architecture:** 不引入新 UI 框架。视觉源真相仍是 `@feedchamber/tokens` 语义色 + `app/web` 全局 CSS（`.card` / `.btn` / `nav`）+ Tailwind v4 utility + DaisyUI 主题 `orinlight`/`orindark`。本轮主要改 `app/web/src/assets/css/global.css` 与组件 class/结构；色板 seed 不动。验证以亮/暗截图 + `pnpm --filter @feedchamber/web check`（biome）为主——纯视觉任务无单元测试，用「grep 断言旧值消失 + 浏览器截图」代替 TDD 红绿。

**Tech Stack:** SolidJS、TanStack Router、Tailwind CSS v4、DaisyUI v5、`@feedchamber/tokens`、Vite / `pnpm dev:web`、Claude Browser preview（截图）。

## Global Constraints

- 色板 hex **零改动**（primary light `#8c2de4` / dark `#9d50db` 等，见 tokens）
- 不改路由、API、信息架构
- 不改 `app/dashboard`
- 不引入 CSS Modules / 新组件库
- 每 Phase 结束必须亮/暗截图，**等用户 review 通过**再进下一 Phase
- Phase 0 + Phase 1 **合并一批**再截图（用户已选）
- 启动预览用根目录：`pnpm dev:web`（turbo + 依赖 watch）
- 对外 API（Button props、PostCard props 等）尽量不破坏；视觉 class 可改
- `prefers-reduced-motion` 必须尊重；`:focus-visible` 必须可见
- 代码注释优先简体中文、简洁

---

## File Map

| 区域 | 创建/修改 |
|------|----------|
| 全局表面 | `app/web/src/assets/css/global.css`（`.card` / `.btn` / `nav` / 背景 / sort-tab 圆角等） |
| 主题属性 | `app/web/src/styles.css`（仅必要时；默认少动） |
| 基础组件 | `app/web/src/components/ui/Button.tsx`、`EmptyState.tsx`、`Loading.tsx`、`Skeleton.tsx`、`Toast.tsx`、`Drawer.tsx` |
| 壳层 | `app/web/src/components/layout/nav/Nav.tsx`、`SiteName.tsx`、`UserInfo.tsx`、`Footer.tsx`；`aside/left/*`；`layouts/layout.tsx` |
| 业务 | `app/web/src/components/post/PostCard.tsx`、`PostListPage.tsx`；后续详情/用户/空间/auth 页 |
| 预览 | 若缺：`.claude/launch.json` 增加 web dev 配置 |
| 文档 | 本 plan；spec 已存在 |

**不修改（除非 review 明确要求）：** `packages/tokens/src/tokens.config.ts` 的 seed hex、DaisyUI 主题名。

---

### Task 1: Phase 0 — 全局表面（card / btn / nav / 背景）

**Files:**
- Modify: `app/web/src/assets/css/global.css`（约 L1–L110 的 body/card/btn/nav；背景在文件前部与 `body.layout-default-bg`）
- 背景图规则当前在 `app/web/src/assets/css/global.css` 顶部（若实际在别处，以仓库为准；`styles.css` 只 import global）

**Interfaces:**
- Consumes: CSS 变量 `--color-border`、`--color-bg`、`--themeColorMain`、tokens radius/shadow（Tailwind 类或 raw）
- Produces: 新 `.card` / `.card-solid` / 中性 `.btn` hover；全站卡片与裸按钮立即受影响

- [ ] **Step 1: 确认当前「旧值」存在（基线）**

Run:

```bash
rg -n "border-radius: 0\.18rem|--card-bg-opacity: 0\.7|btn:hover|opacity: 0\.175|blur\(8px\)" app/web/src/assets/css/global.css
```

Expected: 至少命中 card radius `0.18rem`、opacity `0.7`、`.btn:hover` 刷紫、背景 `0.175`、nav `blur(8px)`。

- [ ] **Step 2: 改背景图透明度**

在 `body.layout-default-bg::before`（或等价选择器）把：

```css
opacity: 0.175;
```

改为：

```css
opacity: 0.11;
```

- [ ] **Step 3: 重写 `.card` 表面**

将现有：

```css
.card {
	--card-bg-opacity: 0.7;
	background-color: rgba(255, 255, 255, var(--card-bg-opacity));
	border-radius: 0.18rem;
	box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
	transition: box-shadow 0.3s ease;
}

.card:hover {
	box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.dark .card {
	background-color: rgba(62, 62, 68, var(--card-bg-opacity));
}
```

替换为（保留 `.card-bg-opacity-100` 覆盖逻辑，并增加 `.card-solid`）：

```css
.card {
	--card-bg-opacity: 0.88;
	background-color: rgba(255, 255, 255, var(--card-bg-opacity));
	border-radius: 0.75rem; /* --radius-lg */
	border: 1px solid color-mix(in srgb, var(--color-border) 80%, transparent);
	box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.05); /* shadow-sm */
	transition:
		box-shadow 0.2s ease,
		border-color 0.2s ease;
}

.card:hover {
	box-shadow:
		0 4px 6px -1px rgb(0 0 0 / 0.1),
		0 2px 4px -2px rgb(0 0 0 / 0.1); /* shadow-md */
}

.dark .card {
	--card-bg-opacity: 0.85;
	/* 与遗留 dark card 面接近，保持可读 */
	background-color: rgba(42, 42, 48, var(--card-bg-opacity));
	border-color: color-mix(in srgb, var(--color-border) 90%, transparent);
}

.card-solid,
.card-bg-opacity-100 {
	--card-bg-opacity: 1 !important;
}
```

若 `color-mix` 在目标浏览器有顾虑，可退化为：

```css
border: 1px solid var(--color-border);
```

- [ ] **Step 4: 改 `.btn` — 去全局刷紫，改中性 hover + 圆角**

将：

```css
.btn {
	border-radius: 0.25rem;
	cursor: pointer;
}

.btn:hover {
	background-color: var(--themeColorMain);
	color: #fff;
}
```

改为：

```css
.btn {
	border-radius: 0.5rem; /* --radius-md */
	cursor: pointer;
	transition:
		background-color 0.15s ease,
		color 0.15s ease,
		opacity 0.15s ease;
}

/* 裸 .btn / default：中性 hover，禁止全站刷紫 */
.btn:hover:not(.btn-primary):not(.btn-error):not(.sort-tab):not(:disabled):not(.btn-disabled) {
	background-color: color-mix(in srgb, var(--color-item) 80%, transparent);
}

.dark .btn:hover:not(.btn-primary):not(.btn-error):not(.sort-tab):not(:disabled):not(.btn-disabled) {
	background-color: color-mix(in srgb, var(--color-item) 70%, transparent);
}

.btn-primary:hover:not(:disabled):not(.btn-disabled) {
	/* daisy 已有；确保主色链不断 */
	filter: brightness(1.05);
}

.btn:focus-visible {
	outline: 2px solid var(--color-primary);
	outline-offset: 2px;
}
```

注意：`#pagination a:hover` 等**有意**用主色的控件保留（分页选中/hover 仍可主色）。`#logo:hover` 品牌块保留主色。

- [ ] **Step 5: 加强 nav 玻璃**

将 `nav` 中：

```css
backdrop-filter: blur(8px);
```

改为：

```css
backdrop-filter: blur(12px);
-webkit-backdrop-filter: blur(12px);
```

可选：把魔法边框色改为更接近 token 的写法（不改观感也可先不动边框色，避免 scope 膨胀）。

- [ ] **Step 6: sort-tab 圆角与卡片体系对齐（小改）**

将 `.sort-tab-group` / `.sort-tab` 的 `border-radius: 0.125rem` 改为：

```css
/* group */
border-radius: 0.5rem;
/* tab */
border-radius: 0.375rem;
```

- [ ] **Step 7: 断言旧值消失**

Run:

```bash
rg -n "border-radius: 0\.18rem|--card-bg-opacity: 0\.7|opacity: 0\.175|blur\(8px\)" app/web/src/assets/css/global.css
rg -n "\.btn:hover \{|background-color: var\(--themeColorMain\)" app/web/src/assets/css/global.css
```

Expected:
- 第一条：无 `0.18rem` / `0.7` card opacity / `0.175` / `blur(8px)`（nav 应为 12px）
- 第二条：全局 `.btn:hover { background-color: var(--themeColorMain) }` **不再存在**；`#pagination a:hover` / `#logo:hover` / `.sort-tab.active` 等仍可含主色

- [ ] **Step 8: Commit**

```bash
git add app/web/src/assets/css/global.css
git commit -m "$(cat <<'EOF'
style(web): restrain glass surfaces and neutralize bare btn hover

Raise card opacity/radius, soft border + shadow, nav blur 12px,
lower bg image strength; stop global purple hover on every .btn.
EOF
)"
```

---

### Task 2: Phase 1 — Button 对齐 focus / 默认类型

**Files:**
- Modify: `app/web/src/components/ui/Button.tsx`

**Interfaces:**
- Consumes: DaisyUI `btn` / `btn-primary` / `btn-ghost` 等；Task 1 的全局 `.btn` 圆角与 focus
- Produces: 同一 `ButtonProps` API；default 类型更明确中性

- [ ] **Step 1: 给 default 类型显式中性类（避免依赖被删的全局紫 hover）**

在 `buildClass` 的 `switch (props.type)` 中，`default` 分支目前是空注释。改为推入中性类，例如：

```ts
		switch (props.type) {
			case 'primary':
				cls.push('btn-primary');
				break;
			case 'dashed':
				cls.push('btn-outline', 'border-dashed');
				break;
			case 'text':
				cls.push('btn-ghost');
				break;
			case 'link':
				cls.push('btn-ghost', 'text-primary', 'underline');
				break;
			default:
				// 中性实心：不走 primary
				cls.push('btn-ghost', 'bg-base-200');
				break;
		}
```

若 `btn-ghost bg-base-200` 与现网 default 观感差太大，改用：

```ts
			default:
				cls.push('border', 'border-base-300', 'bg-base-100');
				break;
```

以截图为准，**不要**给 default 加 `btn-primary`。

- [ ] **Step 2: 确保 root class 含 focus 友好**

`twMerge('btn', buildClass(local), ...)` 已含 `btn`。可追加 utility：

```ts
class={twMerge('btn focus-visible:outline-none', buildClass(local), (local as any).class)}
```

（outline 已由全局 `.btn:focus-visible` 提供时，组件侧 `outline-none` 避免双环；二选一，截图检查只保留一圈 primary 环。）

- [ ] **Step 3: lint**

Run:

```bash
pnpm --filter @feedchamber/web exec biome check src/components/ui/Button.tsx
```

Expected: 通过或仅既有无关告警；新改无 error。

- [ ] **Step 4: Commit**

```bash
git add app/web/src/components/ui/Button.tsx
git commit -m "style(web): make Button default neutral and focus-safe"
```

---

### Task 3: Phase 1 — EmptyState + Loading

**Files:**
- Modify: `app/web/src/components/ui/EmptyState.tsx`
- Modify: `app/web/src/components/ui/Loading.tsx`

**Interfaces:**
- Consumes: 现有 props（`icon` / `title` / `description` / `children`；Loading `text`）
- Produces: 同一 props；更克制的层级 class

- [ ] **Step 1: 更新 EmptyState**

替换整文件为：

```tsx
import type { JSX } from 'solid-js';

export function EmptyState(props: {
	icon?: string;
	title?: string;
	description?: string;
	children?: JSX.Element;
}) {
	const iconClass = () => props.icon ?? 'i-tabler-inbox';
	return (
		<div class="w-full flex flex-col items-center justify-center py-20 px-4 text-center gap-3">
			<div class={`${iconClass()} text-4xl text-base-content/25`} aria-hidden="true" />
			<span class="text-base font-semibold text-base-content/80">{props.title ?? '暂无数据'}</span>
			{props.description && (
				<span class="text-sm text-base-content/50 max-w-sm">{props.description}</span>
			)}
			{props.children && <div class="mt-2 flex items-center gap-2">{props.children}</div>}
		</div>
	);
}
```

（若项目无 `text-base-content`，改用 `text-remark` / `text-secondary` 等价 token class，保持层级：图标最淡 → 标题 → 描述。）

- [ ] **Step 2: 更新 Loading**

```tsx
export function Loading(props: { text?: string }) {
	return (
		<div class="w-full flex flex-col items-center justify-center py-20 text-base-content/50 gap-3">
			<div class="i-tabler-loader-2 animate-spin text-2xl text-primary" aria-hidden="true" />
			<span class="text-sm">{props.text ?? '加载中...'}</span>
		</div>
	);
}
```

- [ ] **Step 3: lint + Commit**

```bash
pnpm --filter @feedchamber/web exec biome check src/components/ui/EmptyState.tsx src/components/ui/Loading.tsx
git add app/web/src/components/ui/EmptyState.tsx app/web/src/components/ui/Loading.tsx
git commit -m "style(web): calm EmptyState and Loading hierarchy"
```

---

### Task 4: Phase 1 — Skeleton 对齐新卡片节奏

**Files:**
- Modify: `app/web/src/components/ui/Skeleton.tsx`（至少 `PostCardSkeleton`；其它 variant 圆角统一 `rounded-lg` / `rounded-md`）

**Interfaces:**
- Consumes: 全局 `.card` 新 radius
- Produces: `PostCardSkeleton` 布局预留头像槽（与 Task 10 PostCard 一致）

- [ ] **Step 1: 重写 PostCardSkeleton**

将 `PostCardSkeleton` 改为：

```tsx
export function PostCardSkeleton() {
	return (
		<div class="p-4 card" aria-hidden="true">
			<div class="flex gap-3 items-start">
				<span class={`${skel} h-9 w-9 shrink-0 rounded-full`} />
				<div class="min-w-0 flex-1 space-y-2">
					<div class={`font-semibold text-base ${skel} w-4/5 h-5 rounded-md`} />
					<div class="flex items-center gap-3">
						<span class={`${skel} h-3 w-14 rounded`} />
						<span class={`${skel} h-3 w-16 rounded`} />
						<span class={`${skel} h-3 w-12 rounded`} />
					</div>
				</div>
			</div>
		</div>
	);
}
```

- [ ] **Step 2: 其它 skeleton 内边距 `p-3.5` → `p-4`，明显 `rounded` 过小的改为 `rounded-md`**（grep `p-3.5` in file）

Run:

```bash
rg -n "p-3\.5" app/web/src/components/ui/Skeleton.tsx
```

有则改 `p-4`。

- [ ] **Step 3: Commit**

```bash
git add app/web/src/components/ui/Skeleton.tsx
git commit -m "style(web): align skeletons to community post card rhythm"
```

---

### Task 5: Phase 1 — Toast + Drawer 表面

**Files:**
- Modify: `app/web/src/components/ui/Toast.tsx`
- Modify: `app/web/src/components/ui/Drawer.tsx`

**Interfaces:**
- Consumes: Button、DaisyUI `alert` / `toast`
- Produces: 同一导出；圆角与边框更克制

- [ ] **Step 1: ToastItem 加圆角与边框**

将 ToastItem 根节点 class 从：

```tsx
class={`alert ${alertClass[props.toast.type]} shadow-lg flex items-center toast-item-enter`}
```

改为：

```tsx
class={`alert ${alertClass[props.toast.type]} shadow-md rounded-lg border border-base-300/40 flex items-center gap-2 toast-item-enter`}
```

- [ ] **Step 2: Drawer 遮罩与面板**

遮罩 button 当前：

```tsx
class="absolute inset-0 card backdrop-blur-sm ..."
```

`card` 在全屏遮罩上不合适。改为：

```tsx
class="absolute inset-0 bg-black/30 backdrop-blur-sm transition-opacity duration-300 border-0 rounded-none"
```

面板：

```tsx
class={`absolute top-0 ... h-full ... bg-base-100 shadow-xl flex flex-col ... rounded-none`}
```

保持；头部 `h-14` / `border-b` 已可。可选面板加 `border-l border-base-300`。

- [ ] **Step 3: Commit**

```bash
git add app/web/src/components/ui/Toast.tsx app/web/src/components/ui/Drawer.tsx
git commit -m "style(web): tighten Toast and Drawer surfaces"
```

---

### Task 6: Phase 0+1 截图 Review Gate（强制停）

**Files:**
- 可能 Create: `.claude/launch.json`（若不存在）

- [ ] **Step 1: 确保可启动 web**

若无 `.claude/launch.json`，创建：

```json
{
  "version": "0.0.1",
  "configurations": [
    {
      "name": "web",
      "runtimeExecutable": "pnpm",
      "runtimeArgs": ["dev:web"],
      "port": 3000
    }
  ]
}
```

端口以 `app/web` 实际 dev port 为准（查 `app/web/vite.config.*` 或启动日志；常见 3000/5173）。

- [ ] **Step 2: 启动并打开时间线**

Run: `pnpm dev:web`（或 preview_start `web`）

打开 `/timeline`（需登录态则用本地已有 session / 测试账号）。

- [ ] **Step 3: 截图清单（每张亮 + 暗）**

| # | 场景 | 怎么触发 |
|---|------|----------|
| 1 | 时间线列表卡片面 | `/timeline` |
| 2 | 排序条 + 发帖 tab | 同上顶部 |
| 3 | Nav 玻璃 + 工具按钮 hover | 顶栏 |
| 4 | 空态（若有空列表或临时清空） | EmptyState 出现处 |
| 5 | Skeleton | 慢网或刷新瞬间 |
| 6 | Toast | 任意成功/失败操作或临时 `toast.success('demo')` |
| 7 | 移动宽 Nav/侧栏 | viewport ~375 |

暗色：点主题切换到 dark 再各拍一遍关键帧。

- [ ] **Step 4: 把截图交给用户 review**

在对话中贴图，明确问：Phase 0+1 是否通过？  
**在用户明确说通过之前，禁止开始 Task 7。**

- [ ] **Step 5: 若用户要求微调 opacity/radius**

只改 `global.css` 对应数字，再截关键 diff 图，仍停在本 gate。

---

### Task 7: Phase 2 — Nav 壳层

**Files:**
- Modify: `app/web/src/components/layout/nav/Nav.tsx`
- Modify: `app/web/src/components/layout/nav/UserInfo.tsx`（若有夸张 hover scale，克制之）
- 可选: `SiteName.tsx`（一般不动品牌 hover 紫，属签名）

**Interfaces:**
- Consumes: Task 1 nav 玻璃；Button/中性 btn
- Produces: 同一 `NavProps`

- [ ] **Step 1: Nav 工具按钮统一中性**

汉堡/搜索/主题等 `class="... btn"` 保持；确认无额外 `hover:bg-primary`。图标尺寸可统一 16–18。给可聚焦控件加 `aria-label`（已有则保留）。

- [ ] **Step 2: UserInfo 克制头像 hover（社群感保留、少戏剧）**

若存在 `group-hover/avatar:scale-250` 这类极端缩放，降为：

```tsx
// 示例：适度放大，避免飞出顶栏
class="... transition-transform duration-200 group-hover/avatar:scale-110"
```

具体 class 以文件为准；目标是可读、不挡通知铃。

- [ ] **Step 3: 去掉调试残留**

`UserInfo.tsx` 若有 `console.log(avatar())`，删除。

- [ ] **Step 4: Commit**

```bash
git add app/web/src/components/layout/nav/
git commit -m "style(web): calm Nav chrome and avatar hover"
```

---

### Task 8: Phase 2 — Sidebar / Layout / Footer

**Files:**
- Modify: `app/web/src/components/aside/left/Sidebar.tsx`
- Modify: `app/web/src/components/aside/left/Menu.tsx`（及 Spaces/Tags 若同模式）
- Modify: `app/web/src/layouts/layout.tsx`
- Modify: `app/web/src/components/layout/Footer.tsx`
- Modify: `app/web/src/assets/css/global.css`（`#sidebar-left` / `.sidebar-left-pop` 边框色可改 token）

**Interfaces:**
- Consumes: `.card` 新表面
- Produces: 同一 layout slot 行为

- [ ] **Step 1: 侧栏菜单项圆角**

`Menu.tsx` 内 `rounded-[0.15rem]` → `rounded-md`；`hover:bg-gray-100 dark:hover:bg-zinc-600` 可改为 `hover:bg-base-200`（若 daisy 基色可用），避免硬编码灰阶漂移。

- [ ] **Step 2: 侧栏卡片间距**

`card p-2` 的区块之间用 `space-y-2` 包裹（在 `SidebarContent.tsx` 根上加 `flex flex-col gap-2`），避免挤成一片。

- [ ] **Step 3: Layout 主列间距**

`layout.tsx` 中：

```tsx
<div class="flex w-full h-full px-2 space-x-2!">
```

可改为更稳的：

```tsx
<div class="flex w-full h-full px-3 gap-3">
```

（去掉 `space-x-2!` 若 gap 已表达间距。）`mt-13` 保留以避开 fixed nav。

- [ ] **Step 4: Footer 玻璃与字色**

`global.css` footer 已半透明；Footer 组件保持 `text-remark`。链接若启用，hover 用 `hover:text-primary` 或 brand-text，不要整块刷紫。

- [ ] **Step 5: 移动侧栏遮罩与 Drawer 一致**

`Sidebar.tsx` 遮罩：

```tsx
class="fixed inset-0 z-40 backdrop-blur-sm"
```

补背景：

```tsx
class="fixed inset-0 z-40 bg-black/30 backdrop-blur-sm"
```

- [ ] **Step 6: Commit**

```bash
git add app/web/src/components/aside/left/ app/web/src/layouts/layout.tsx app/web/src/components/layout/Footer.tsx app/web/src/assets/css/global.css
git commit -m "style(web): align sidebar layout footer with glass system"
```

---

### Task 9: Phase 2 截图 Review Gate（强制停）

- [ ] **Step 1: 截图** — 亮/暗：整页壳（timeline）、移动侧栏开合、Footer、User 下拉
- [ ] **Step 2: 交用户 review；通过前禁止 Task 10**

---

### Task 10: Phase 3 — PostCard 社群时间线结构

**Files:**
- Modify: `app/web/src/components/post/PostCard.tsx`

**Interfaces:**
- Consumes: `PostListItem`（含 `author_avatar`、`author_name`、`author_id`、`space_name`、`comment_count`、`has_new`、`created_at` 等，见 `packages/api-client/src/types.ts` L112–135）
- Produces: 同一 `PostCardProps`；内部结构升级

- [ ] **Step 1: 抽出共享内部内容，避免两套 Link 重复**

目标结构（两套路由 Link 共用 children）：

```tsx
import type { PostListItem } from '@feedchamber/api-client';
import { Link } from '@tanstack/solid-router';
import { Show } from 'solid-js';

export interface PostCardProps {
	post: PostListItem;
	/** space slug — 有则走 /spaces/$slug/post，否则 SEO /t 或 timeline */
	slug?: string;
}

function PostCardBody(props: { post: PostListItem }) {
	const initial = () => (props.post.author_name || '?').slice(0, 1).toUpperCase();
	return (
		<div class="flex gap-3 items-start">
			<div class="shrink-0">
				<Show
					when={props.post.author_avatar}
					fallback={
						<div
							class="h-9 w-9 rounded-full bg-primary/15 text-primary text-sm font-semibold flex items-center justify-center"
							aria-hidden="true"
						>
							{initial()}
						</div>
					}
				>
					<img
						src={props.post.author_avatar}
						alt=""
						class="h-9 w-9 rounded-full object-cover bg-base-200"
					/>
				</Show>
			</div>
			<div class="min-w-0 flex-1">
				<div class="font-semibold text-base truncate flex items-center gap-2">
					<span class="truncate">{props.post.title}</span>
					<Show when={props.post.has_new}>
						<span
							class="activity-pulse w-2 h-2 rounded-full bg-primary shrink-0"
							title="有新回复"
						/>
					</Show>
				</div>
				<div class="mt-1.5 text-xs text-remark flex flex-wrap items-center gap-x-3 gap-y-1">
					<span class="font-medium text-base-content/70">{props.post.author_name}</span>
					<span>{props.post.space_name}</span>
					<span>{props.post.comment_count} 评论</span>
				</div>
			</div>
		</div>
	);
}

export default function PostCard(props: PostCardProps) {
	const seoSlug = () => props.post.slug || 'post';
	const cardClass = 'card p-4 cursor-pointer block transition-shadow';

	return (
		<Show
			when={props.slug}
			fallback={
				<Link
					to="/t/$slug/$id"
					params={{ slug: seoSlug(), id: props.post.id }}
					class={cardClass}
				>
					<PostCardBody post={props.post} />
				</Link>
			}
		>
			<Link
				to="/spaces/$slug/post"
				params={{ slug: props.slug! }}
				search={{ id: props.post.id }}
				class={cardClass}
			>
				<PostCardBody post={props.post} />
			</Link>
		</Show>
	);
}
```

- [ ] **Step 2: 全局 CSS 增加活跃脉搏（尊重 reduced-motion）**

在 `global.css` 末尾追加：

```css
.activity-pulse {
	box-shadow: 0 0 0 0 color-mix(in srgb, var(--color-primary) 50%, transparent);
	animation: activity-pulse 2s ease-out infinite;
}

@keyframes activity-pulse {
	0% {
		box-shadow: 0 0 0 0 color-mix(in srgb, var(--color-primary) 45%, transparent);
	}
	70% {
		box-shadow: 0 0 0 6px transparent;
	}
	100% {
		box-shadow: 0 0 0 0 transparent;
	}
}

@media (prefers-reduced-motion: reduce) {
	.activity-pulse {
		animation: none;
	}
}
```

- [ ] **Step 3: 目视** — 有 `has_new` 的帖显示紫点；无头像显示首字母；标题 semibold

- [ ] **Step 4: Commit**

```bash
git add app/web/src/components/post/PostCard.tsx app/web/src/assets/css/global.css
git commit -m "feat(web): community timeline PostCard with activity pulse"
```

---

### Task 11: Phase 3 — PostListPage 间距与骨架已对齐

**Files:**
- Modify: `app/web/src/components/post/PostListPage.tsx`（若需）

- [ ] **Step 1: 列表 gap**

确认：

```tsx
<div class="grid gap-2 grid-cols-1">
```

可升为 `gap-3` 若卡片更高后需透气。排序条 `mb-2` → `mb-3` 可选。

- [ ] **Step 2: Commit（有改再提）**

```bash
git add app/web/src/components/post/PostListPage.tsx
git commit -m "style(web): breathe post list spacing"
```

---

### Task 12: Phase 3 截图 Review Gate（强制停）

- [ ] 亮/暗：PostCard 有/无头像、有/无 `has_new`、列表 + skeleton  
- [ ] 用户通过前禁止页面 Phase

---

### Task 13: Phase 4 — 页面铺开（按页截图）

每页一个小循环：**改关键 class/局部模块 → 截图 → review → commit**。不要一次改光。

**顺序与文件提示：**

| 顺序 | 路由 | 主要文件 |
|------|------|----------|
| 1 | `/timeline` | 已大部分被 PostList 覆盖；查 `pages/(app)/timeline/*` 残余 |
| 2 | 帖详情 | `components/post/PostDetailView.tsx`、`pages/(app)/timeline/-modules/*`、`t/$slug.$id.tsx` |
| 3 | `/user/*` | `pages/(app)/user/-modules/*` |
| 4 | `/spaces/*` | spaces 页 + `.space-card` CSS（已有主色 hover，保持克制） |
| 5 | auth | `components/auth/AuthShell.tsx`、Login/Register |
| 6 | messages / leaderboard | 对应 pages |

**每页最低改动清单：**
- 外层容器用 `.card` / `p-4` / `gap-3`，去掉 `rounded-[0.15rem]` 遗老
- 元信息 `text-xs text-remark`；标题 semibold
- 主 CTA 才 `btn-primary`
- 截图亮/暗各至少 1 张

**Commit 粒度：** 每页或每相关模块一次，message 如 `style(web): refine post detail surfaces`。

---

### Task 14: 收尾检查

- [ ] **Step 1: 全库扫遗老**

```bash
rg -n "0\.18rem|rounded-\[0\.15rem\]|bg-blue-500.*新回复|--card-bg-opacity: 0\.7" app/web/src
rg -n "\.btn:hover \{[^}]*themeColorMain" app/web/src/assets/css/global.css
```

Expected: 业务路径无旧卡片尖角；新回复指示非冷蓝；无全局 btn 刷紫。

- [ ] **Step 2: biome**

```bash
pnpm --filter @feedchamber/web check
```

- [ ] **Step 3: 对照 spec 成功标准勾选**（`docs/superpowers/specs/2026-07-25-ui-redesign-design.md` §11）

- [ ] **Step 4: 最终 commit（若有扫尾）** 或合并说明交给用户

---

## Self-Review (plan author)

| Spec 项 | 对应 Task |
|---------|-----------|
| 色板锁死 | Global Constraints + 不改 tokens seed |
| 克制玻璃 card/nav/bg | Task 1 |
| 禁止全局 btn 刷紫 | Task 1 Step 4 |
| Button/Empty/Skeleton/Toast/Drawer | Task 2–5 |
| Phase 0+1 合并截图 | Task 6 |
| Nav/Sidebar/Layout/Footer | Task 7–8 + gate 9 |
| PostCard 社群 + 活跃脉搏 | Task 10–12 |
| 页面铺开顺序 | Task 13 |
| a11y focus / reduced-motion | Task 1 focus-visible；Task 10 pulse media query |
| 每批截图 review | Task 6/9/12/13 |

无 TBD 占位；PostListItem 字段名与 api-client 一致；不假设不存在的 avatar 字段——类型已有 `author_avatar`。

---

## Execution Handoff

Plan 已保存到 `docs/superpowers/plans/2026-07-25-ui-redesign.md`。

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每 Task 新子代理，Task 间审查，迭代快  
2. **Inline Execution** — 本会话按 executing-plans 连续做，在 Review Gate 停下等你

选哪个？
