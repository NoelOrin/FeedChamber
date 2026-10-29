> ⚠️ **已废弃（2026-10-18）**：本文提取的是重构前的「Offset Litho / 档案索引」视觉基线。
> 2026-10-18 起 `app/web` 已改为 shadcn/ui 基准（中性色 + 冷紫主色、描边分层、无装饰元素），
> 现行规范见 `app/web/app/globals.css`（shadcn token）与 `app/web/components/ui/` 下的原语。
> 本文仅作历史参考，其中的视觉建议（网格、颗粒、眉标、索引编号）不再执行。

# Feed Chamber 设计系统 — 从现有 UI 提取

> 提取来源：运行中的 `app/web`（dev server :3000）截图 + `packages/tokens/w3c-tokens.json` 及其产物 + `app/web/src/assets/css/global.css`。
> 参考截图：`documents/reference-images/current-ui/`；计算样式原始数据：`documents/designs/raw/`。
> 用途：作为 React + vinext + shadcn 重构的视觉基线。**保留既有识别度，不重新发明。**

---

## 0. 设计语言定性：Offset Litho / 档案索引（Archive Index）

现有 UI 已有一套完整且罕见的自洽视觉语言，重构必须继承而不是抹掉：

| 母题 | 表现 | 出处 |
|---|---|---|
| 纸张 | 暖白 `#fcfaf8` 底，非纯白；卡片才是纯白 `#ffffff` | `--color-bg` / `--color-card` |
| 24px 网格 | `body::before` 固定层，1px 发丝线，`background-size: 24px 24px`，opacity 0.11（暗色 0.065），带 mask 渐隐 | `global.css` `body.layout-default-bg::before` |
| 油墨颗粒 | `body::after` feTurbulence SVG noise，opacity 0.028，`mix-blend-mode: multiply`（暗色 soft-light 0.05） | 同上 `::after` |
| 套准条 | 卡片顶部 2px 渐变条 `linear-gradient(90deg, primary 0 38%, transparent 74%)` | `.card--archive::before` |
| 索引编号 | 等宽/展示字体的极淡大号序号（`color-mix(text 14%, transparent)`）作楼层/排序标记 | `.index-num` |
| 报头眉标 | 大写字母 + 宽字距 + 前置短横线 | `.eyebrow` / `.eyebrow--rule::before` / `.masthead-kicker` |
| 裁切标记 | 页脚 "24PX GRID ↗"、"纸 / 油墨 / 网格" 等元信息 | 登录页左栏 |
| 巨型水印 | 竖排 `ARCHIVE` / `REGISTER` 展示字，几乎与背景同色 | 登录/注册页左栏 |

**结论**：这是一套"编辑部/印刷厂"气质，靠**发丝线、字距、层级对比**而非色彩堆砌建立秩序。重构失败的最典型方式就是把它做成通用 shadcn 默认外观。

---

## 1. 色板

### 1.1 品牌色阶（唯一可信来源：`packages/tokens/dist/tokens.css`）

浅色品牌阶（暖底用）：

| Token | Hex | 角色 |
|---|---|---|
| `--color-brand-1` | `#faf0ff` | 最浅底/选中背景 |
| `--color-brand-2` | `#f1d6ff` | 菜单选中底 `colorBgMenuItemSelected` |
| `--color-brand-3` | `#e0adff` | |
| `--color-brand-4` | `#ca83fc` | |
| `--color-brand-5` | `#ab56f0` | = `--color-primary-hover` |
| `--color-brand-6` | `#8c2de4` | **种子主色 / `--color-primary`** |
| `--color-brand-7` | `#6a1cbd` | = `--color-primary-active` |
| `--color-brand-8` | `#4c0f96` | |
| `--color-brand-9` | `#320670` | |
| `--color-brand-10` | `#1e034a` | 最深 |

深色品牌阶（暗底用，`--color-brand-dark-1..10`）：`#160c22` `#24103a` `#32164e` `#461b6e` `#5f2199` `#7928c4` `#9b4fd9` `#c17df0` `#daa8f8` `#ecd2fa`

### 1.2 语义色（light）

| Token | Hex | 用途 |
|---|---|---|
| `--color-primary` | `#8c2de4` | 主行动色 |
| `--color-primary-hover` | `#ab56f0` | hover |
| `--color-primary-active` | `#6a1cbd` | active |
| `--color-primary-content` | `#ffffff` | 主色上的字 |
| `--color-bg` | `#fcfaf8` | 页面纸底 |
| `--color-bg-elevated` | `#ffffff` | 浮层 |
| `--color-card` | `#ffffff` | 卡片 |
| `--color-item` | `#f5f1eb` | 次级面/内嵌块/hover 底 |
| `--color-text` | `#141416` | 正文墨色 |
| `--color-text-secondary` | `#6b625c` | 次级文字 |
| `--color-text-muted` | `#9a9590` | 弱化/元信息 |
| `--color-border` | `#e8e2dc` | 发丝边框（暖调） |
| `--color-brand-text` | `#cc73fb` | 链接/品牌文字（浅底） |

### 1.3 语义色（dark，`.dark` 覆盖）

`primary #9b4fd9` · `hover #c17df0` · `active #7928c4` · `bg #0c0c0e` · `bg-elevated #18181b` · `card #18181b` · `item #27272a` · `text #f0f0ea` · `text-secondary #a1a1aa` · `text-muted #71717a` · `border #27272a` · `brand-text #a859ff`

> 注意暗色正文是**暖白 `#f0f0ea`**，不是纯白；暗色底 `#0c0c0e` 带极轻暖调。这是"油墨纸"气质在暗色下的延续，勿改成 zinc 默认。

### 1.4 反馈色

`--color-info #3b82f6`（dark `#60a5fa`）· `--color-success #22c55e` · `--color-warning #f59e0b` · `--color-error #ef4444`
Dashboard 专用阶（AntD 对齐，web 不用）：`#52c41a` / `#fa8c16` / `#ff4d4f`

### 1.5 需要清理的色彩重复源

同一主色 `#8c2de4` 目前散落在 **4 处**：`tokens.css` 的 `--color-primary`、`daisyui.css` 的 `--color-primary`、`styles.css` 里 `@property --themeColorMain` 的 initial-value、`vite.config.ts` + `capacitor.config.ts` 的 PWA/SplashScreen `theme_color`。重构时统一收口到 tokens，其余引用 `var(--color-primary)`。

---

## 2. 排版

### 2.1 字体族

| Token | 值 | 用途 |
|---|---|---|
| `--font-sans` | `"Segoe UI", Arial, "Microsoft Yahei", sans-serif` | tokens 声明的正文族 |
| 实际 body | `"Segoe UI", system-ui, -apple-system, "Helvetica Neue", Arial, "Microsoft Yahei", sans-serif` | `global.css` 覆盖了 tokens 值 |
| `--font-display` | `"BarlowExtraBold", "Segoe UI", system-ui, sans-serif` | 展示字，本地 `@font-face` 加载 `Barlow-ExtraBold.ttf` |
| `--font-mono` | `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Cascadia Code", monospace` | 索引/眉标/代码 |
| `BlackItalic` | `MontserratAlternates-BlackItalic.ttf` | 仅个别装饰标题 |

**待改进**：正文依赖系统 `Segoe UI`（Windows 优先），macOS 上实际落到 `system-ui`。重构时建议显式引入一款正文无衬线（含中文字重），并补 `font-display: swap` 的中文字子集策略。

### 2.2 排版层级（从 global.css + 截图实测归纳）

| 层级 | 字号 | 字重 | 行高 | 字距 | 变换 | 类 |
|---|---|---|---|---|---|---|
| Hero 展示 | `clamp(2.75rem, 7vw, 5.2rem)` | 900 | 0.78 | -0.07em | uppercase | `.display-hero` |
| 档案大标题 | 视容器 | 900 | 0.84 | -0.07em | uppercase | `.display-title--archive` |
| 通用展示标题 | 视容器 | 900 | 0.82 | -0.06em | uppercase | `.display-title` |
| 文章标题 | 1.375rem / 22px | 700 | 2rem | — | — | `article .title` |
| 眉标 eyebrow | 0.66rem / ~10.5px | 700 | — | +0.16em | uppercase | `.eyebrow` |
| 报头 kicker | 0.68rem | 600 | — | +0.18em | uppercase | `.masthead-kicker` |
| 索引编号 | 0.62rem（mono）/ 0.72rem（display） | 600/900 | — | +0.14em / -0.02em | uppercase | `.archive-index` / `.index-num` |
| 正文 | 0.875rem / 14px | 400 | 1.75em | -0.015em(body) | — | `#article #content` |
| 列表项 | 0.875rem | 400 | 1.25rem | — | — | `#sectionNav li` |
| 按钮 xs | 0.75rem / 12px | 500 | 16px | — | — | tokens typography `button.xs` |
| 按钮 xl | 1.125rem / 18px | 600 | 28px | — | — | tokens typography `button.xl` |

**核心手法**：正文用**负字距**（-0.015em）收紧，标签用**正字距**（+0.14~0.18em）撑开，形成"密正文 / 疏标签"的对比。展示字用极端负字距（-0.06~-0.07em）+ 行高 0.78~0.84，让大标题成块。这套对比是识别度的主要来源。

> ⚠️ `w3c-tokens.json` 里有完整 typography 组（button xs…xl 等），但 `scripts/build.ts` **没有输出它**——tokens 的排版层目前是断的。重构时应顺手补齐 `--text-*` 输出，避免层级只靠散落的 CSS 类维持。

---

## 3. 圆角 / 阴影 / 描边

| Token | 值 |
|---|---|
| `--radius-sm` | 0.25rem（= `--radius-field`，输入框） |
| `--radius-md` | 0.5rem（= `--radius-selector` / `.btn`） |
| `--radius-lg` | 0.625rem（= `--radius-box`） |
| `--radius-xl` | 0.875rem |
| `--radius-2xl` | 1.25rem |
| `--radius-full` | 9999px |

卡片圆角在 `global.css` 里硬编码为 **12px**（≈ `--radius-lg` 与 `--radius-xl` 之间），未走 token —— 重构时收敛到 `--radius-lg`。

阴影：`--shadow-sm` … `--shadow-2xl` 为 Tailwind 默认值，但**现有卡片不用它们**。`.card` 用定制双层投影：

```css
box-shadow:
  0 1px 0 0 color-mix(in srgb, var(--color-text) 5%, transparent),      /* 顶部发丝 */
  0 10px 28px -20px color-mix(in srgb, var(--color-text) 16%, transparent); /* 纸张偏移 */
```

hover 抬升 `translateY(-2px)` + 阴影加深；**帖子详情页例外**：`.post-detail .card:hover` 明确禁用位移（阅读稳定性优先）——这条意图要保留。

描边哲学：**1px 实线硬边框**，靠 `color-mix` 从 `--color-text` 按比例派生透明度（3% 网格 / 5% 发丝 / 10% 报头线 / 14% 索引字 / 18% 眉标线 / 42% archive-index）。这套"同一墨色不同浓度"是层次的主要手段，重构时不要换成多套灰色。

---

## 4. 组件规范

### 4.1 卡片 `.card`
- 底 `--color-card`，边框 `1px solid --color-border`，圆角 12px，上述双层阴影
- hover：`translateY(-2px)` + 边框浓度 +7% + 阴影加深
- `.card--archive`：顶部 2px 套准渐变条
- `.card--inset`：底色 `color-mix(card 96%, item)`、**虚线边框**、无阴影、hover 不动 —— 用于内嵌次级块
- 暗色：底 `#18181b`，边框 `#27272a`

### 4.2 按钮 `.btn`
- 圆角 0.5rem，`transition: bg/color/opacity 0.18s ease, transform 0.18s var(--ease-spring), box-shadow 0.18s`
- `:active` → `scale(0.98)`（**按压反馈是刚性的，必须保留**）
- `.btn-primary`：底 `--color-primary`，阴影 `0 6px 18px -10px primary 55%`；hover 换 `--color-primary-hover` 且阴影扩为 `0 10px 24px -12px 62%`；active 换 `--color-primary-active`
- 次级按钮 hover：底 `color-mix(item 78%, transparent)`（暗色 72%）
- `.btn-error`：`--color-error`，hover 混 12% 黑
- `:focus-visible`：`outline 2px solid primary, offset 2px`（全站默认是 1.5px，按钮更重）
- 主 CTA 形态（登录页实测）：**满宽、高约 44px、圆角 8px、白字、下方带同色扩散投影**

### 4.3 输入框
- 圆角 `--radius-field` 0.25rem（实测登录页输入框圆角偏大，约 8px —— 与 token 不一致，重构时统一）
- 左侧内嵌 lucide 图标（账号/邮箱/锁），图标色 `--color-text-muted`
- 标签置于框外上方，`--color-text-secondary`，0.875rem
- placeholder 色 `--color-text-muted`
- 边框 `--color-border`，聚焦转 primary

### 4.4 认证页布局（`AuthShell`）
- 桌面：`grid lg:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]` 左右分栏，左 52%
- 左栏：深色油墨面板（`#141416`~`#0c0c0e`），含品牌胶囊、竖排巨型水印、eyebrow 短横线、两行堆叠的中文大标题（"欢迎/回来"）、说明段、半透明信息卡（3 条 bullet）、页脚元信息
- 右栏：纸底 + 24px 网格 + 居中卡片，卡片顶部紫色套准条，右上角 `# SIGN IN` mono 徽标
- 分隔：`OR` 字样的发丝线分隔（`.rule-diagonal` 思路）
- 移动：单栏，隐藏左栏水印面板

### 4.5 导航
- `nav` fixed，`z-index: 30`，底部 1px 边框，**半透明 + backdrop-blur**：浅 `rgba(255,255,255,0.78)` + `blur(12px)`，暗 `rgba(12,12,14,0.72)`
- Logo `#logo`：高 32px、圆角 0.5rem、字重 900、hover 时文字淡出换成图标（`opacity` 交叉）
- 侧栏 `#sidebar-left`：`transition: all 0.36s var(--ease-litho)`；`<1280px` 抽屉（`left:-320px` → `.drawer-active` 滑入，`z-index:50`）；`≥1280px` 常驻、透明背景
- 分区导航 `#sectionNav`：项 0.875rem、圆角 0.375rem、padding 0.25rem 0.5rem、hover 底 `#f3f4f6`（暗 `#52525b`）；子级 `margin-left 0.5rem` + 左侧 1px 竖线

### 4.6 分页 `#pagination`
方形 2.5rem 按钮、圆角 0.5rem、白底 + `--color-border` 边框 + 轻阴影；hover/current 填 `--themeColorMain` 白字。

### 4.7 正文排版（`#article #content`）
- 14px / 1.75 行高，块间距 0.75rem
- **标题带左侧 6px 粗边 + 底部 1px 边**，且 h1/h2/h3/h4 分别用 solid / solid / dashed / dotted 边框线型区分层级（印刷感做法，很有辨识度）
- 引用块：左侧 0.25em 边 + `#ebf4ff` 底 + `#7f9cf5` 边 + `#667eea` 字（**这套蓝紫和全站暖纸+紫的调性冲突，是重构要修正的偏差点**）
- 行内 code：`color-mix(item 85%)` 底、圆角 0.25rem、0.875em
- 代码块：`color-mix(text 6%, card)` 底、1px 边框、圆角 6px
- 列表 marker 字重 700 且着主色
- 图片：`max-width:100%`、居中、圆角 6px
- 分割线：2px dashed

### 4.8 Toast / 骨架 / 空态
`components/ui/`：`Toast`(123 处引用)、`Skeleton`(28)、`EmptyState`、`Loading`、`Drawer`、`Button`（antd 风格 props 映射到 Tailwind + twMerge）、`MarkdownEditor/`（7 文件）。这些是**包装层，不是 shadcn 原语**（仓库内无 Radix / CVA / `components.json`）。

---

## 5. 间距与栅格

- 底座：**24px 网格**（既是背景视觉也是节奏基准）
- Tailwind 默认 4px 步进 + tokens 补充 `--spacing-18: 4.5rem`(72px) / `--spacing-88: 22rem` / `--spacing-128: 32rem`
- 认证页栅格：`1.05fr / 0.95fr`
- 卡片内距实测约 24~32px；列表项内距 12~16px
- z-index 阶：`dropdown 1000 / sticky 1020 / fixed 1030 / modal-backdrop 1040 / modal 1050 / popover 1060 / tooltip 1070`（但 `nav` 用裸 `z-index:30`、侧栏 `50` —— 两套体系混用，需统一）

---

## 6. 动效

| Token | 值 | 用途 |
|---|---|---|
| `--duration-fast` | 140ms | 微反馈 |
| `--duration-normal` | 280ms | 常规 |
| `--duration-slow` | 520ms | 大面积 |
| `--ease-spring` | `cubic-bezier(0.22,1,0.36,1)` | 按钮按压 |
| `--ease-smooth` | `cubic-bezier(0.4,0,0.2,1)` | 标准 |
| `--ease-out-expo` | `cubic-bezier(0.16,1,0.3,1)` | 入场 |
| `--ease-archive` | `cubic-bezier(0.25,0.1,0.25,1)` | 档案感直线运动 |
| `--ease-litho` | `cubic-bezier(0.32,0.72,0,1)` | **主曲线**：卡片 420ms、侧栏 360ms、入场 560ms |

入场动画 `litho-enter`：`opacity 0 → 1` + `translateY(10px) → 0` + `scale(0.99) → 1`，560ms `--ease-litho`；阶梯延迟 70/140/210/280ms（`.litho-enter--2..5`）。

主题切换用 **View Transitions**：`::view-transition-old/new(root)` 关闭默认 cross-fade，靠 `z-index` 分层做自定义 clip-path 擦除（`styles.css` 已埋钩子）。

其他：`::selection` 主色 22%（暗 28%）；滚动条 6px、`--color-border` 做 thumb、`scrollbar-width: thin`。

---

## 7. 暗色模式

- 机制：`.dark` 类挂 `<html>`，Tailwind 侧 `@custom-variant dark (&:is(.dark *))`
- 首屏无闪烁：`DarkModeScript` 在 hydration 前写 `data-theme-mode`（`[data-theme-icon]` 三态图标切换）
- daisyUI 主题名：`orinlight`（默认）/ `orindark`，由 `tokens/dist/daisyui.css` 生成
- 暗色不是简单反色：网格 opacity 降到 0.065、noise 改 `soft-light` 提到 0.05、正文暖白 `#f0f0ea`

---

## 8. → shadcn 变量桥接表

重构采用「shadcn 原语 + tokens 自定义主题」，因此需要把现有语义色映射到 shadcn 的 `--background/--foreground/...` 约定。**建议 tokens 新增一份 `shadcn.css` 产物**（与 `daisyui.css` 平级，由 `w3c-tokens.json` 生成），内容：

| shadcn 变量 | 应取的值 | 来自 |
|---|---|---|
| `--background` | `#fcfaf8` / 暗 `#0c0c0e` | `--color-bg` |
| `--foreground` | `#141416` / 暗 `#f0f0ea` | `--color-text` |
| `--card` / `--card-foreground` | `#ffffff` `#141416` / 暗 `#18181b` `#f0f0ea` | `--color-card` |
| `--popover` / `--popover-foreground` | `#ffffff` / 暗 `#18181b` | `--color-bg-elevated` |
| `--primary` / `--primary-foreground` | `#8c2de4` `#ffffff` / 暗 `#9b4fd9` `#ffffff` | `--color-primary` |
| `--secondary` / `--secondary-foreground` | `#f5f1eb` `#141416` / 暗 `#27272a` `#f0f0ea` | `--color-item` |
| `--muted` / `--muted-foreground` | `#f5f1eb` `#9a9590` / 暗 `#27272a` `#71717a` | `--color-item` / `--color-text-muted` |
| `--accent` / `--accent-foreground` | `#f1d6ff` `#6a1cbd` / 暗 `#1e1030` `#daa8f8` | `--color-brand-2` / `brand-dark-9` |
| `--destructive` / `--destructive-foreground` | `#ef4444` `#ffffff` | `--color-error` |
| `--border` / `--input` / `--ring` | `#e8e2dc` / 暗 `#27272a`；ring = primary | `--color-border` / `--color-primary` |
| `--radius` | `0.5rem`（`.5rem`，卡片另用 `--radius-lg` 0.625rem） | `--radius-md` |

**shadcn 默认值必须覆盖掉的项**：默认 `--background: 0 0% 100%`（纯白）、默认 `--radius: 0.5rem` 单一圆角、默认 shadow。这三项若不改，成品就会失去"纸"的质感而变成通用模板。

**shadcn 不覆盖、需自建原语的**（现有识别度所在）：
1. `.card--archive` 套准条 + 双层偏移阴影
2. `.eyebrow` / `.masthead-kicker` / `.archive-index` / `.index-num` 索引字组
3. 24px 网格 + noise 画布层（`layout-default-bg`）
4. `litho-enter` 阶梯入场
5. 文章正文的 6px 左边框 + 分线型（solid/dashed/dotted）标题层级
6. 认证页双栏油墨面板

建议做成 `components/ui/archive-*.tsx`（`ArchiveCard` / `ArchiveEyebrow` / `ArchiveIndex` / `LithoGrid` / `Prose`），而不是散在全局 CSS —— 这也是把现有 ID 选择器（`#logo` `#sidebar-left` `#sectionNav` `#pagination` `#article`）迁移成组件的契机。

---

## 9. 现状技术债清单（重构时应一并处理）

| 问题 | 证据 | 处置 |
|---|---|---|
| daisyUI 与自定义 `.card`/`.btn` 同名冲突 | `tailwind.css` 只 `exclude: a, button`，`global.css` 又重写 `.card`/`.btn` | 移除 daisyUI 后由 shadcn + archive 原语接管 |
| 主色 4 处重复定义 | 见 §1.5 | 收口到 tokens |
| 全局 ID 选择器耦合 DOM | `#logo` `#sidebar-left` `#sectionNav` `#pagination` `#article #content` | 组件化 |
| tokens 的 typography 组未输出 | `scripts/build.ts` 缺该分支 | 补 `--text-*` 输出 |
| 圆角实测与 token 不符 | 卡片 12px 硬编码、输入框 ~8px vs `--radius-field` 4px | 收敛 |
| z-index 两套体系 | token 1000+ vs nav 裸 30 / 侧栏 50 | 统一走 token |
| 正文蓝紫引用块跑调 | `#ebf4ff`/`#7f9cf5`/`#667eea` | 换成暖纸 + 主色浓度阶 |
| 正文字体依赖系统 Segoe UI | `global.css` body | 显式引入含中文的正文族 |
| daisyUI 类用量（替换工作量） | `card` 240 · `toast` 123 · `btn` 111 · `input` 50 · `avatar` 34 · `skeleton` 28 · `badge` 12 · `modal` 11 · `join` 8 · `alert` 6 · `table` 4 · `drawer` 3 · `tabs` 2 · `menu` 2 | 前 6 项覆盖 ~80% |

---

## 10. 参考截图清单

`documents/reference-images/current-ui/`（36 张桌面 1440px + 4 张移动，含明暗）

| 文件 | 内容 |
|---|---|
| `_login-light-desktop.png` | ⚠️ 实际落在 /timeline（首屏缓存重定向），登出态见 `pre-stub-guest/` |
| `_login-dark-desktop.png` | 登录页，双栏油墨/纸面板 |
| `_register-{light,dark}-desktop.png` | 注册页，5 字段长表单 |
| `_timeline-{light,dark}-desktop.png` | 时间线列表（12 帖 + 筛选 + 公告条） |
| `_t_p0001_…-{light,dark}-desktop.png` | 帖子详情 + 嵌套回复 + 回复编辑器 + 相关主题 |
| `_timeline_create` / `_timeline_tag` | 发帖页 / 标签页 |
| `_spaces` / `_spaces_tech` / `_spaces_tech_post` | 版面总览 / 单版首页 / 单版发帖 |
| `_user` `_user_notifications` `_user_follow` `_user_favorites` | 用户中心各页（`/user/posts` 重定向到 `/user`） |
| `_home` `_leaderboard` `_messages` `_search` | 首页聚合 / 排行榜 / 私信 / 搜索 |
| `pre-stub-guest/` | 无后端时的登出态登录/注册页 × 桌面/移动 |

---

## 11. 页面解剖（补录自真实渲染页）

### 11.1 全站报头（masthead）
- 顶栏左：`☰` 抽屉钮 → 品牌方块（深色底 + 白色图标）+ `灵思社区 Feed Chamber` / 下方 `ARCHIVE — 档案索引` 小字 + `• LIVE` 状态点
- 顶栏右：搜索 / 通知铃 / 私信 / 头像（渐变紫圆）
- 报头下方一条 **kicker 带**：左 `EDITION — 2026 —— OFFSET LITHO · 24…`，右 `VOL. 01 — ARCHIVE INDEX`（全大写 mono + 宽字距）
- 顶栏之下可挂**橙色公告条**（`--color-warning` 底、白字、右侧 `×`），标题加粗 + 副行说明

### 11.2 左侧栏
- 卡片区 `版面索引` + 计数，下方条目：`技术 / 产品 / 闲聊 / 生活`，各带小色块图标，当前项反白
- `<1280px` 收成抽屉（见 §4.5）

### 11.3 时间线列表页
- **页头卡**：`# TIMELINE — 时间线 | 12 篇档案` 眉标 + 右侧 `+ 发布新主题` 深色胶囊；主标题 `索引 · 档案现场`（display，含中点分隔）；副说明一行；底部 `• LIVE INDEX / 2026-10-13` 与右对齐 `滚动加载 ↗`
- **筛选两行**：`视图` 标签 + 胶囊组（最新/新帖/未读/已读/我参与），`排序` 标签 + 胶囊组（最新/最热/评论最多/活跃）+ 右侧 `⇄ SORTED`；选中态为**深色实心底白字**，未选为描边
- **帖子卡解剖**（自上而下）：
  1. 左上 `index-num`（三位字母/数字组合：`777` `EEE` `NNN` `WWW` `333` `AAA` `HHH` `RRR` `ZZZ` `666` `DDD` `MMM`），右上 `📌 置顶` 深色胶囊（置顶帖才有）
  2. 标题（粗、`text-[15px]` 量级）
  3. 元信息行：圆形头像（首字 + 品牌色底）· 作者名 · 版面徽标（浅底胶囊 `技术`/`产品`/`闲聊`/`生活`）· `👁 1543` · `💬 1543` · `🕐 10月12日`
  4. 右下 `OPEN ↗` mono 小字
  5. 左侧一条**竖向发丝轨**从头像向下延伸，串起卡片内容
- 列表尾部 `没有更多了` + `— END OF INDEX —` 居中 mono 分隔
- 页脚带：`OF 灵思社区 Feed Chamber / ARCHIVE INDEX` + 一行说明 `档案索引 · 对话留痕 · 以纸页承载讨论，以索引编织社区。— Offset litho · 24px grid · hairline rule.` + 三枚胶囊 `© 2026 灵思社区 ORINFORUM` `CRAFTED — 纸 · 油墨 · 网格` + 右侧版权声明

### 11.4 帖子详情页
- 面包眉标：`# TOPIC — 主题 / 技术`，右上 `📌 置顶`
- 大标题（display，约 28px）
- **元信息条**：头像 + 作者 · `⊙ 2026-10-12 10:36` · `@ Go后端 VIEWS` · `💬 1543 REPLIES` · `32 点赞` 浅底徽标 · 右对齐 `ID — 777777`（mono）
- 动作行：`✓ 标记已读 (2)`（主色实心胶囊）+ `🕐 历史版本`（描边胶囊）；右侧 `关注话题` `收藏` 文字链
- **正文卡**：卡头 `≋ BODY — 正文` ↔ 右 `苏澈 · 原帖`；正文含行内 code 浅底胶囊与深色代码块
- 楼层尾：`点赞 · 3` `🕐 版本` 描边小胶囊 + 右侧 `# 7777`（mono 楼层号）
- **回复区**：卡头 `💬 REPLIES — 回复 [1543]` ↔ 右 `按时间排序 · 最早在前`
  - 嵌套评论用**左侧竖轨 + 虚线连接**表达层级
  - 每层：`index-num`（`3333`/`AAAA`/`HHHH`/`RRRR`）+ 头像 + 作者 + 时间胶囊 + 右侧 `# 01CMT0`
  - 作者本人多一个 `✎ 编辑` 胶囊
- `— END —` 分隔后是回复编辑器
- **回复编辑器卡**：眉标 `WRITE A REPLY — 回复`；`回复内容` 输入 + 右上 `👁 预览`；工具栏 `B I U S <> 🔗 ⧉ | H₁ H₂ H₃ | 有序 无序 引用 任务`；placeholder `写下你的回复…支持 Markdown · 拖拽粘贴图片`；底部 `🖫 草稿自动保存 · 支持 MARKDOWN` ↔ `按 ⌘+ENTER 快速发布` + 主色 `发布回复`（带发送图标）
- **相关主题**：纯文字列表 + 右侧浏览量，无卡片框（弱层级）

### 11.5 对重构的直接影响
1. `index-num` 不是装饰性序号，而是**每层内容的身份标记**（帖子/评论/楼层各一套），必须做成原语并保证与后端 id 稳定映射
2. 列表页的"选中态 = 深色实心底白字"与主色按钮并存，说明**主色只用于行动，中性深色用于状态**——这条规则要写进组件 variant
3. 详情页层级完全靠**卡片 + 卡头眉标 + 发丝轨**表达，几乎没有分割线堆叠；改成 shadcn `Card` 时若加上默认阴影会破坏这层秩序
4. 回复编辑器是全站最复杂的交互组件，Tiptap 工具栏的图标顺序与分组需 1:1 保留
5. 公告条用 `warning` 橙而非主色紫，是全站唯一的非紫强调块，保留其稀缺性

