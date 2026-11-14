# Web 前台视觉层重构：改用 shadcn/ui 基准

**日期：** 2026-10-18
**范围：** 仅 `app/web`（用户端论坛）。`app/dashboard`（Ant Design）不动。
**动机：** 现有视觉是「暖米纸底 + 24px 大圆角 + 无描边软托 + 药丸按钮 + 英文大写眉标 + 帖子编号」的档案/印刷风格。用户判定该风格观感差，要求完全重构、不继承现有视觉，以 shadcn/ui 为基准，并明确禁止过度装饰。

## 1. 目标与非目标

**目标**

- 视觉层按 shadcn/ui（new-york，neutral）的语言重建：token 驱动、描边分层、克制的圆角与阴影。
- 保留全部功能、路由、取数逻辑、交互与 a11y 契约（URL 是硬契约，e2e 锁死）。
- 保留品牌识别：紫色仍是唯一主行动色（换成更冷的 modern violet）。

**非目标**

- 不改 API、路由、信息架构、Dashboard、`@orinforum/tokens` 的 seed hex。
- 不引入新组件库；继续用 Radix + Tailwind v4 + lucide。
- 不做动效升级（除既有 skeleton pulse 与必要的浮层进出场）。

## 2. 视觉语言

### 2.1 分层原则

1. 一个面用 `1px --border` 表达，不靠阴影。阴影只出现在浮层（dialog / popover / dropdown / tooltip / toast）。
2. 层级 = 描边 + 字号 + 留白。**禁止**：悬浮抬升、渐变、噪点、网格底纹、装饰性编号、英文大写眉标、`— END —` 这类排版装饰。
3. 页面级容器一律 `rounded-lg border border-border bg-card`，无 `shadow`。

### 2.2 Token（在 `app/web/app/globals.css` 定义，取代对 `@orinforum/tokens` CSS 的依赖）

| 变量 | Light | Dark | 用途 |
| --- | --- | --- | --- |
| `--background` | `oklch(1 0 0)` | `oklch(0.145 0 0)` | 页面底 |
| `--foreground` | `oklch(0.145 0 0)` | `oklch(0.985 0 0)` | 正文 |
| `--card` | `oklch(1 0 0)` | `oklch(0.205 0 0)` | 卡片面 |
| `--muted` / `--secondary` | `oklch(0.97 0 0)` | `oklch(0.269 0 0)` | 次级面 / hover |
| `--muted-foreground` | `oklch(0.556 0 0)` | `oklch(0.708 0 0)` | 元信息 |
| `--border` / `--input` | `oklch(0.922 0 0)` | `oklch(1 0 0 / 10%)` | 描边 |
| `--primary` | `oklch(0.541 0.247 293)` ≈ `#7c3aed` | `oklch(0.606 0.25 292)` ≈ `#8b5cf6` | 主行动 / 链接 / 选中 |
| `--ring` | 同 primary | 同 primary | 焦点环 |
| `--success` / `--warning` / `--info` / `--destructive` | green / amber / blue / red 阶 | 同色系提亮 | 状态 |

`--radius: 0.625rem`，并映射 `--radius-sm/md/lg/xl = radius-4/-2/+0/+4`：控件 `rounded-md`(8px)，容器 `rounded-lg`(10px)。

**禁止**再出现 `var(--color-text)` / `var(--color-card)` 这套旧 token 词汇：组件只允许用语义工具类（`bg-card`、`text-muted-foreground`、`border-border`…）。

### 2.3 排版

| 角色 | 规格 |
| --- | --- |
| 页面标题 | `text-xl`(20px) / `text-2xl`(24px) `font-semibold tracking-tight` |
| 卡片/区块标题 | `text-[15px] font-semibold` |
| 正文 | `text-sm leading-7`（正文流 `leading-[1.75]`） |
| 元信息 | `text-xs text-muted-foreground`，数字 `tabular-nums` |
| 标签 | `text-xs`，不加大写/宽字距 |

中文字体栈：`system-ui / -apple-system / PingFang SC / Microsoft YaHei`；删除 Barlow 展示字体。全局负字距只保留在标题上（`tracking-tight`），正文取消字距注入。

### 2.4 原语几何

- **Button**：`h-9 rounded-md text-[13px]`；variants `default`(紫) / `secondary`(灰) / `outline` / `ghost` / `destructive` / `link`；sizes `sm`(32) / `md`(36) / `lg`(40) / `icon`(36/32)。删除 pill、`round`、`xl`。
- **Card**：`rounded-lg border bg-card shadow-none`；保留 `CardHeader/Title/Description/Content/Footer` 的语义分区。
- **Badge**：`h-5 rounded-md px-1.5 text-[11px]`；`default`(primary)/`secondary`/`outline`/`destructive` 及状态色。
- **Input/Textarea/Select**：`h-9 rounded-md border border-input bg-transparent`，focus 用 `ring-2 ring-ring/40`。
- **Tabs**：shadcn segmented：`bg-muted p-0.5 rounded-lg`，选中 `bg-background shadow-xs`。
- **Dialog/Popover/Dropdown/Sheet**：`rounded-lg border shadow-md`，`p-4~6`，标题 `text-[15px] font-semibold`。
- **EmptyState**：虚线描边容器 + 居中图标/文案，不用彩色圆形底。
- **Skeleton**：`rounded-md bg-muted animate-pulse`。
- **Prose**：shadcn typography 风格（无左侧粗边、无虚线底线），`h2` 上边距 + `border-b`，`pre` 用 `bg-muted rounded-md`。

## 3. 骨架与页面

- **顶栏**：`h-14 border-b`，`bg-background/95 backdrop-blur`（全站唯一模糊），左侧菜单 + 站点名，右侧搜索/通知/主题/账号。账号改用 `DropdownMenu`。
- **左栏**：240px sticky；分区标题 `text-xs font-medium text-muted-foreground`；条目 `h-8 rounded-md px-2 text-sm hover:bg-muted`，选中 `bg-muted text-foreground`。去掉「软托」包裹。
- **主列**：`max-w-[46rem]`；页头 = 标题 + 一行描述 + 右侧主行动。
- **列表**：一张 `Card` + `divide-y` 行；行内标题 `text-[15px] font-medium`，meta 一行 `text-xs`；右侧统计。取消「每帖一张浮起卡片」。
- **详情**：文章 Card → 正文 Prose → 评论 Card + `divide-y`，嵌套回复用左侧 `border-l` 缩进。
- **认证页**：左右分栏改为单列居中卡片（`max-w-sm`），去掉巨幅展示字与水印。

## 4. 验收

1. 每批改完在 `:3001` 用 playwright 截图（1440 桌面 / 390 移动，亮 + 暗）人工比对。
2. `pnpm --filter @orinforum/web build`（vinext build + `tsc --noEmit`）通过。
3. 全局 grep 无 `spec-` / `litho-` / `archive-` / `display-` / `eyebrow` 残留，无 `var(--color-…)` 旧词汇。
4. 交互契约不变：URL、搜索快捷键、主题切换、无限滚动、SSE 通知。

## 5. 分批

| 批次 | 内容 |
| --- | --- |
| A | globals.css + 20 个原语 + 骨架 + 时间线 |
| B | 帖子详情 / 评论 / 编辑器 chrome |
| C | 用户中心 / 版面 / 私信 / 搜索 / 排行 / 通知 / 签到 / 认证页 |
| D | 清扫残留、ui-showcase、build 与全量截图 |

## 6. 实施结果（2026-10-18 收口）

**落地的 token**（`app/web/app/globals.css`，Web 不再 import `@orinforum/tokens`）

- Light：`--background #fff`、`--foreground oklch(0.145 0 0)`、`--muted oklch(0.97 0 0)`、`--muted-foreground oklch(0.556 0 0)`、`--border oklch(0.922 0 0)`、`--primary oklch(0.541 0.281 293)`（≈ `#7c3aed`）。
- Dark：`--background oklch(0.145 0 0)`、`--card oklch(0.205 0 0)`、`--border oklch(1 0 0 / 12%)`、`--primary oklch(0.606 0.25 292.717)`。
- `--radius: 0.625rem`，`rounded-sm/md/lg/xl = 6/8/10/14px`。
- 旧词汇 `var(--color-text)` / `var(--color-card)` / `var(--color-item)` 全量清除（0 处残留），只保留语义工具类。

**清除的装饰层**

- `components/ui/archive.tsx` 删除（Eyebrow / IndexNum / DisplayTitle / Rule / lithoEnter）。
- `globals.css` 里 `spec-*` / `litho-*` / `archive-*` / `display-*` / `eyebrow` / `index-num` 全量移除；Barlow 展示字体与 `app/fonts/` 一并删除。
- 主题切换的圆形扩散动画移除（`lib/theme.ts` 直接换色）。

**结构变化**

- 主列从 `max-w-[46rem]` 改为占满容器（`max-w-6xl` 内 240px 侧栏 + 弹性主列）。
- 时间线/搜索结果从「每帖一张浮起卡片」改为「一张 Card + divide-y 行」。
- 帖子详情把标题、动作、正文合并进同一张 Card；评论同一张 Card + divide-y，嵌套回复用左侧 1px 竖线缩进。
- 认证页从左右分栏（含竖排水印、线网）改为单列居中卡片；`auth-panel.tsx` 删除。
- 账号菜单从 CSS hover 卡（hack）改为 Radix `DropdownMenu`。
- 左栏从「软托卡片」改为裸列表分区（`text-xs` 分组标题 + `rounded-md` 行）。
- 活动热力图浓度改用主色透明度阶梯；签到月历改用主色填充。

**验收**

- `pnpm --filter @orinforum/web build`（vinext build + `tsc --noEmit`）通过；`biome lint ./app ./components ./lib` 无 error。
- 路由巡检（18 条路由，含 404）：无 pageerror、无非预期 console error，URL 契约未变。
- 截图：`artifacts/ui-refactor/`（桌面 1440 亮/暗 + 移动 390），采集脚本 `scripts/shot.mjs`。
