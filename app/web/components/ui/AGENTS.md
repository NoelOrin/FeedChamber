# UI 原语（`components/ui/`）

shadcn/ui 基准的原语层：**无业务、无取数、只接收 props**。前台与后台共用同一套，后台不再有 antd / Pro Components。

> 上级：[components/AGENTS.md](../AGENTS.md) · 视觉基准：`app/web/app/globals.css`（shadcn token，中性色 + 冷紫主色）

实测规模：33 个文件。token 的唯一载体是 `app/web/app/globals.css`——**Web 不依赖 `@feedchamber/tokens`**（该包只服务 Dashboard），daisyUI 已移除。

## 硬规则

1. **不 import 任何业务域**（`components/<domain>/`），不 import `lib/auth`、`lib/queries`、`lib/query`。原语只认 props。
2. **不取数**。需要数据的复合组件（如 `data-table.tsx`）由调用方传 `data` / `columns` / `loading`。
3. **图标从 `~/lib/icons` 引入**。`biome.json` 的 `noRestrictedImports` 把直接 import `lucide-react` 或 `lucide-react/*` 判为 **error**。
4. **颜色只用 token 类**（`bg-background`、`text-foreground`、`border-border`、`bg-primary` …），不要写死 hex 或 `bg-[#...]`。改配色改 `globals.css`，不要改组件。
5. **`prose.tsx` 是正文排版的唯一出口**（Markdown 渲染走它）。shadcn 不覆盖这一块，不要就地再写一套 typography。
6. **浮层层级只用 `globals.css` 的 `--z-*` token**（`z-[var(--z-popover)]` 等），不要写裸 `z-50` / `z-[100]`。层级梯度是 `dropdown 1000 < sticky 1020 < fixed 1030 < modal-backdrop 1040 < modal 1050 < popover 1060 < tooltip 1070`：任何「弹窗里的下拉/浮层」都必须 ≥ `--z-popover`，否则会被 modal（1050）压住——`select.tsx` 曾用 `z-50`，在 dialog/sheet 里点不出来。
7. **浮层必须 portal 出去**，不要在业务容器里用 `relative + absolute` 自己画面板。自绘面板既吃不到统一层级，也会被 dialog 的 `overflow-y-auto` 裁掉——`tree-select.tsx` 已因此改走 `popover.tsx`。


## 文件表

| 文件 | 说明 |
|------|------|
| `button.tsx` | `cva` 变体：`variant`（default/primary/secondary/outline/ghost/destructive/dashed/link）× `size`（xs/sm/md/lg/xl/icon） |
| `card.tsx` | 卡片容器族 |
| `badge.tsx` / `avatar.tsx` / `separator.tsx` / `skeleton.tsx` | 基础展示 |
| `input.tsx` / `textarea.tsx` / `label.tsx` / `checkbox.tsx` / `switch.tsx` / `select.tsx` | 表单控件（Radix 基准） |
| `form.tsx` | 表单编排（`react-hook-form` + 错误展示） |
| `dialog.tsx` / `sheet.tsx` / `popover.tsx` / `dropdown-menu.tsx` / `tooltip.tsx` | 浮层族 |
| `confirm-dialog.tsx` | `useConfirm()` —— 删除/危险操作的统一确认入口。**不要用 `window.confirm`** |
| `tabs.tsx` / `segmented.tsx` / `collapse.tsx` | 分段与折叠 |
| `table.tsx` | 裸表格结构 |
| `data-table.tsx` | 复合表格：TanStack Table 封装（排序 / 列筛选 / 行选择 / 分页 / 空态 / loading），后台列表页统一用它 |
| `pagination.tsx` | 页码组件（列表页用；无限滚动用 `components/pagination/`） |
| `descriptions.tsx` | 详情页键值对布局（后台详情统一用它） |
| `page-header.tsx` | 页面标题区（标题 + 描述 + 右侧操作） |
| `empty-state.tsx` / `loading.tsx` | 空态与加载态 |
| `tree-select.tsx` | 树形选择（版面树等） |
| `toast.tsx` | Toast 视口；命令式 API 在 `lib/toast.ts` |
| `alert.tsx` / `prose.tsx` | 提示条 / 正文排版 |

## 外观验证

**没有原语对照台。** 历史文档提到的 `app/ui-showcase/` 原语对照台**已不存在**（`app/web/AGENTS.md` 仍在引用，属文档漂移）。改原语的验证方式：

```bash
pnpm dev:web          # 手动走查前台 + /admin 两条面
node scripts/axe-audit.mjs   # 可访问性审计（需先起 web，登录态见脚本内常量）
```

改配色或间距前先读 `app/globals.css` 的 token 定义——那里是单一可信来源。

## 改这里要注意

- 改 `button.tsx` 的变体/尺寸会**同时影响前台与整个后台**（41 个页面）。改完至少走查：前台时间线、帖子详情、`/admin` 首页、任一后台列表页。
- `data-table.tsx` 的 props 是后台 20+ 列表页的共享契约。加必填 prop 等于一次批量改动，优先用可选 prop + 默认值。
- `prose.tsx` 的 class 表被 Markdown 渲染、公告、评论共用。改层级规则前先确认没有页面在覆写它。
- 不要在这里引入 `@feedchamber/tokens`：Web 的 token 来自 `globals.css`，两套 token 并存会立刻出现色差。
- **弹窗里的浮层要真的点一次再交付**：`dialog.tsx` 的 `DialogContent` 自带 `overflow-y-auto`，`sheet.tsx` 同理。浮层即使层级正确，也可能被这两层滚动容器裁掉——验证方式是在 `/admin/spaces` 新建弹窗里点开「可见性策略」下拉与「父版面」树选择，确认面板完整可见且能选中。

## 本文件何时更新

- 新增/删除原语文件
- `data-table.tsx` / `confirm-dialog.tsx` / `prose.tsx` 的对外契约变化
- 浮层层级契约（`--z-*` 梯度、`select.tsx` / `tree-select.tsx` 的实现方式）变化
- 视觉基准从 `globals.css` 迁移到别处（例如恢复 tokens 包消费）
