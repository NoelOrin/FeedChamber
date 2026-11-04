# Feed Chamber Web UI 整体重设计

**日期：** 2026-07-25  
**范围：** `app/web`（SolidJS 前台）  
**约束：** 保持现有色彩方案（紫 `#8c2de4` + 中性灰语义色），不改 hex  
**工作流：** 设计系统先行 → 每批改完截图 review → 再进下一批

---

## 1. 目标与气质

### 产品
Feed Chamber — 紫调社区论坛。主阅读流是时间线 / 空间帖列表 / 帖详情。

### 气质（已确认）
- **克制玻璃态**：保留顶栏毛玻璃与全局背景图，但降低透度与背景抢戏；卡片用细边框 + 轻阴影替代「过透无边」。
- **社群时间线**：作者、头像、活跃态、空间标签在列表里更靠前；新回复用主色「活跃脉搏」点，而不是冷蓝点。

### 非目标（本轮）
- 不改路由 / 信息架构 / API
- 不改 `@feedchamber/tokens` 色板 seed hex
- 不引入新 UI 框架
- 不做 dashboard（`app/dashboard`）重设计

---

## 2. 视觉语言

### 2.1 色彩（锁死）

来源：`packages/tokens` 语义色。只调整**用法**，不改值。

| Token | Light | Dark | 用法 |
|-------|-------|------|------|
| primary | `#8c2de4` | `#9d50db` | CTA、焦点环、活跃点、选中态 |
| primary-hover / active | 现有 | 现有 | 仅 primary 按钮与主链交互 |
| bg / card / item / text / border | 现有 | 现有 | 面、字、分割 |
| brand-text | `#cc73fb` / `#a859ff` | 链接强调、品牌字 |

**禁止：** 全局 `.btn:hover { background: primary }` 这种「一切 hover 变紫」。

### 2.2 玻璃与表面

| 表面 | 规则 |
|------|------|
| 顶栏 Nav | `background` 约 70% 不透明 + `backdrop-filter: blur(12px)`；细底边；滚动后可微加强 |
| 默认 Card | 不透明度约 light `0.88` / dark `0.85`；`1px` border 用 `--color-border`；`shadow-sm`，hover `shadow-md` |
| 实心变体 | `.card-solid` / `.card-bg-opacity-100`：完全不透明，用于编辑器、表单、需要可读性的面 |
| 页面背景图 | `body.layout-default-bg::before` opacity `0.175 → ~0.11` |

### 2.3 圆角 / 阴影 / 间距

| 项 | 现状 | 目标 |
|----|------|------|
| Card radius | `0.18rem` | `0.75rem`（`--radius-lg`） |
| Button radius | `0.25rem` | `0.5rem`（`--radius-md`） |
| 列表卡片间距 | 不统一 | `gap-2` 或 `gap-3` |
| 卡片内边距 | 多为 `p-3.5` | 标准 `p-4`；紧凑 `p-3` |

阴影使用 tokens 已有 `shadow-sm/md`，不引入新阴影配方，除非 review 时证明不够。

### 2.4 字阶

| 角色 | 规则 |
|------|------|
| 帖标题 | `text-base font-semibold`（从过重的 bold 降一档） |
| 元信息 | `text-xs` + `text-secondary` / muted |
| 作者名 | `font-medium`，颜色接近正文，可点 |
| 展示字体 | 仅 SiteName、空态大标题等少数位置用现有 Barlow / BlackItalic |

### 2.5 签名元素：活跃脉搏

- 新回复指示：主色小圆点（替代纯 `bg-blue-500`）
- 可选：极轻 `pulse`（`@media (prefers-reduced-motion: reduce)` 关闭）
- 后续 PostCard / 通知列表统一同一套

### 2.6 无障碍底线

- 可见 `:focus-visible` 环（primary 系）
- 触控目标不小于约 32px（与现 Button `md` 对齐）
- 尊重 `prefers-reduced-motion`
- 亮/暗双模式同步验收

---

## 3. 实施路径

**路径 A：设计系统先行**（已确认）

```
Phase 0  Token / 全局 CSS 表面
Phase 1  基础 UI 组件（与 0 同批交付截图）
Phase 2  壳层：Nav / Sidebar / Layout / Footer
Phase 3  业务组件：PostCard / PostList / 空态业务用法 …
Phase 4  页面铺开：timeline → 详情 → user → spaces → auth → messages
```

每 Phase 结束：**启动 web → 亮/暗截图 → 用户 review → 通过再进下一 Phase**。

用户选择：**Phase 0 + Phase 1 合并为一批**再截图。

---

## 4. Phase 0 — Token / 全局 CSS

### 触达文件（预期）
- `app/web/src/assets/css/global.css` — `.card` / `.btn` / `nav` / 背景图 / 侧栏边框
- `app/web/src/styles.css` — 仅当 `@property` 或主题相关需对齐时
- `packages/tokens` — **默认不改 seed hex**；仅当需要新增非颜色 token（如 glass blur）时经 review 追加

### 具体改动

**`.card`**
- `border-radius: 0.75rem`
- `--card-bg-opacity` 默认约 `0.88`（dark 背景用对应 rgba）
- `border: 1px solid` 使用 border token / 等效色
- `box-shadow:` token `shadow-sm`；`:hover` → `shadow-md`
- 保留 `.card-bg-opacity-100` 与新增语义清晰的实心用法

**`.btn`**
- `border-radius: 0.5rem`
- **删除**全局 `:hover { background: var(--themeColorMain); color: #fff }`
- Hover 由 Button 组件 / daisyUI 类型类负责；裸 `.btn` 只做中性 hover（轻微 bg / 透明度）

**`nav`**
- `backdrop-filter: blur(12px)`（现 8px）
- 底边与现 dark/light 边框色对齐 tokens，避免魔法色值漂移

**背景图**
- opacity `0.175 → 0.11`（可在截图 review 微调 ±0.02）

### 验收
- 任意现有页面卡片圆角/边框一致
- 非 primary 按钮 hover 不再整钮变紫
- 背景图不压过正文对比度

---

## 5. Phase 1 — 基础组件

| 组件 | 路径 | 改动要点 |
|------|------|----------|
| Button | `components/ui/Button.tsx` | 圆角/尺寸与 token 对齐；primary 用主色；default 中性；focus-visible 环；**不改对外 API** |
| Card | 全局 `.card`（Phase 0） | 无独立 TSX 时以全局类为源；业务侧逐步去掉冲突 utility |
| EmptyState | `components/ui/EmptyState.tsx` | 更大气呼吸（py）、图标更淡、描述层级、CTA 槽样式 |
| Skeleton | `components/ui/Skeleton.tsx` | 圆角/间距对齐新卡片；`PostCardSkeleton` 等与未来 PostCard 结构预留一致 |
| Loading | `components/ui/Loading.tsx` | 间距与主色 spinner 一致 |
| Toast | `components/ui/Toast.tsx` | 圆角、边框、玻璃/实心与壳层一致 |
| Drawer | `components/ui/Drawer.tsx` | 圆角、遮罩 blur、与移动侧栏手感一致 |

### 明确不做（Phase 1）
- 重写 PostCard 信息结构
- Nav 信息架构改版
- 新增组件库抽象层

### Phase 0+1 截图清单
1. Button 矩阵：type × size × danger × loading（亮/暗）
2. Card 面：半透明默认 + 实心变体（亮/暗）
3. EmptyState
4. Skeleton 列表
5. Toast 各 type
6. Drawer 开合（可选）

---

## 6. Phase 2 — 壳层

| 组件 | 要点 |
|------|------|
| Nav | 玻璃参数统一；工具按钮中性 hover；主题切换保留 View Transitions |
| SiteName / UserInfo | 触控与 focus；下拉与卡片面一致 |
| LeftSidebar / SidebarContent | 卡片化区块间距；移动遮罩与 Drawer 一致 |
| Layout | 主列最大宽/间距微调；不改 slot 机制 |
| Footer | 字色层级、链接 hover 中性或 brand-text |

截图：整页壳层（timeline 空数据或骨架）亮/暗 + 移动侧栏。

---

## 7. Phase 3 — 业务组件

### PostCard（重点）
现状：标题 + 一行 meta（作者 / 空间 / 评论数）+ 蓝点。

目标结构（社群时间线）：
```
[avatar?]  标题 …                    [活跃点]
           作者 · 空间 · N 评论 · 时间?
```
- 活跃点：主色脉搏
- 作者可链到用户；空间可链到 space
- 保持现有路由（`/t/$slug/$id` 与 `/spaces/$slug/post`）
- API 字段不足时（无 avatar URL）可先占位或隐藏，不阻塞视觉

### 同步
- `PostListPage` 排序条、间距
- `PostCardSkeleton` 与真卡对齐
- 用户页 `PostItem` / 通知项：活跃与 meta 层级向 PostCard 靠拢（可分小步）

---

## 8. Phase 4 — 页面铺开顺序

1. `/timeline` 列表  
2. 帖详情（`/t/...` 与 timeline post）  
3. `/user` 主页与子页  
4. `/spaces` 列表与详情  
5. auth（login/register）  
6. `/messages`、`/leaderboard`、其余  

每页：改 → 截图（亮/暗，必要时 mobile）→ review。

---

## 9. 技术约束

- 栈：SolidJS + TanStack Router + Tailwind v4 + DaisyUI v5（`orinlight` / `orindark`）
- 暗色：`<html class="dark">` + `@custom-variant dark`
- 样式：utility + 全局 CSS；不引入 CSS Modules / styled-components
- 合并 class：`twMerge` / `clsx` 沿用现有
- 启动预览：`pnpm dev:web`（turbo + 依赖 watch，避免 dist 陈旧）
- 截图：Browser preview / 既有 launch 配置

---

## 10. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 全局 `.btn:hover` 删除后裸按钮变「无反馈」 | Phase 1 把中性 hover 写清；grep 裸 `class="btn"` |
| Card 更不透明后与背景图关系变闷 | opacity 可调；截图决定 |
| PostCard 加头像缺字段 | 渐进增强，字段可选 |
| tokens 包与 `styles.css` `@property` 遗留值不一致 | 以 tokens 语义为准，遗留只服务动画/兼容 |
| 一次改全局 CSS 影响 dashboard 外的全站 | 本轮只动 `app/web` 全局 CSS；tokens 色值不动则 dashboard 安全 |

---

## 11. 成功标准

- 色板 hex 零改动，目视仍是「同一品牌紫论坛」
- 卡片/按钮/壳层圆角与间距一致，无 `0.18rem` 遗老卡片
- 非 primary 控件 hover 不再误刷紫
- 列表能一眼看到「谁在说、是否有新动态」
- 每批有亮/暗截图且用户 review 通过后再合并下一 Phase
- 键盘 focus 与 reduced-motion 达标

---

## 12. 已确认决策摘要

| 决策 | 选择 |
|------|------|
| 深度 | 整体重设计 |
| 气质 | 克制玻璃态 + 社群时间线 |
| 色板 | 保持现有 |
| 路径 | A 设计系统先行 |
| 首批 | Phase 0+1 合并，再截图 |
| 审查 | 每批截图 review |
