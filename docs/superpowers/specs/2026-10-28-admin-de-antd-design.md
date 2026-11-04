# 后台去 antd：全量迁移到 shadcn 原语

- 日期：2026-10-28
- 状态：待批准
- 分支：`migrate/react-vinext`（沿用当前分支）
- 前置：C1 词汇统一已完成（tokens 字体栈、`controlHeight`、`admin.css` 切 shadcn 变量、54 处类名、23 个标题字号）

## 1. 背景与目标

`/admin` 是内嵌在主站里的后台，目前仍用 antd 6 + Pro Components 渲染（48 个 `.tsx` / 7502 行，41 个文件 import antd）。主站用 shadcn 原语，两套视觉语言在同一个 origin 下并存：主站是「描边 + 留白 + 无投影」，antd 是「品牌色头部 + 投影 + 32px 控件」。

C1 已经把**色彩词汇、字体栈、控件高度**拉到同一套 token，但组件**形态**仍是 antd 的：表格带斑马纹与排序图标、Modal 有自己的头部结构、Form 自带校验与错误提示样式。要做到真正的视觉统一，必须换掉组件库本身。

**目标**：删除 `antd` 与 `@ant-design/pro-components` 依赖，后台全部改用主站已有的 shadcn 原语 + 新增的 DataTable / Form 原语。

**明确不做**：

- 不改后台路由结构、URL 形态（`?id=` 查询参数保留）
- 不改 `lib/admin/auth/*` 准入与权限门控逻辑
- 不改 `@feedchamber/api-client` 调用方式与后端契约
- 不引入图表库、不做移动端适配

## 2. 决策记录

| 决策点 | 结论 | 被否选项及原因 |
|---|---|---|
| 迁移形态 | 一次性全量替换，删掉两个依赖 | 分阶段保留 ProTable（用户明确选 A：不留中间态） |
| 表格方案 | TanStack Table + 自研 DataTable 原语（shadcn 官方方案） | 手写 `<table>`（丢排序/选择/分页能力）；保留 ProTable（两套并存） |
| 表单方案 | react-hook-form + zod | 受控 state 手写（111 处 `rules` 校验需重写，无复用价值） |
| 壳层 | 复用主站 `AppShell` 结构，后台只换菜单数据源 | 保留 ProLayout 只换皮肤（317 行里 200 行是 ProLayout 的 API 胶水，留着等于没换） |
| 日期选择 | 原生 `<input type="datetime-local">` + 受控封装 | 引入 react-day-picker（仅 2 处用到，包体不值） |
| 数字输入 | `Input` + `type="number"` 封装 | 引入独立 NumberInput 原语（4 处用到，封装一层即可） |
| 消息提示 | 复用主站 `toast.ts` | 自研 message（主站已有 ToastViewport） |
| i18n | 保留 `useLocale`，菜单文案走现有字典 | 引入 antd locale（那是 antd 内部组件文案，删 antd 即无用） |

## 3. 需要新增的原语

主站现有 21 个原语，缺 8 个。按依赖顺序：

| 原语 | 文件 | 依赖 | 替换的 antd 组件 | 使用面 |
|---|---|---|---|---|
| `Table` | `components/ui/table.tsx` | 无（纯样式壳） | Table 结构 | 20 文件 |
| `DataTable` | `components/ui/data-table.tsx` | `@tanstack/react-table` | ProTable / Table 18 处 | 20 文件 |
| `Select` | `components/ui/select.tsx` | `@radix-ui/react-select` | Select 12 文件 | 12 |
| `Checkbox` | `components/ui/checkbox.tsx` | `@radix-ui/react-checkbox` | Table rowSelection | 5 |
| `Switch` | `components/ui/switch.tsx` | `@radix-ui/react-switch` | Switch 4 文件 | 4 |
| `Form` | `components/ui/form.tsx` | `react-hook-form` + `zod` | Form 13 文件 / 111 处 rules | 13 |
| `Pagination` | `components/ui/pagination.tsx` | 无 | Table pagination 49 处 | 全表格 |
| `ConfirmDialog` | `components/ui/confirm-dialog.tsx` | 已有 `dialog.tsx` | Popconfirm 17 文件 | 17 |

另需 `Separator`（`@radix-ui/react-separator`，1 处 Divider）、`Descriptions`（纯 div + grid，3 处）、`Statistic`（纯 div，1 处）、`Empty`（主站已有 `empty-state.tsx`，可直接复用）。

需安装的包：

```
@tanstack/react-table @radix-ui/react-select @radix-ui/react-checkbox
@radix-ui/react-switch @radix-ui/react-separator react-hook-form zod @hookform/resolvers
```

## 4. 壳层重建

`components/admin/layout/pro-shell.tsx`（317 行）拆成三层：

1. **`admin-shell.tsx`** — 复用主站 `AppShell` 的布局骨架（Nav + Sidebar + main），但侧栏内容换成后台菜单树；顶部保留主题/语言切换（主站 `Nav` 已有同款控件）。
2. **`admin-menu.tsx`** — 菜单树从现有 `menuItems` 的 useMemo 逻辑平移，去掉 ProLayout 的 `route` 对象格式，改为主站 `SidebarContent` 同款 `NavItem[]`。
3. **`admin-root.tsx`** — 保持现状（客户端挂载门），仅删掉 `ConfigProvider` 包裹。

`ConfigProvider`（2 处）随之消失；`theme` 由 `getAntdTheme` 改为 CSS 变量驱动（C1 已铺好，`admin.css` 的 `.admin-surface` 覆盖层大部分可删，因为不再有 antd DOM 需要压）。

## 5. 页面迁移顺序

按「依赖少 → 依赖多」排，每批结束都能独立构建通过：

| 批次 | 范围 | 文件数 | 主要工作 |
|---|---|---|---|
| P1 | 壳层 + 原语 | 3 + 10 | 新建原语，重建 ProShell，删 ConfigProvider |
| P2 | 简单页（只读 + 按钮） | ~12 | audit-logs / security / sessions / plugins / api-keys / oauth-clients / backups / webhooks / invites / search / settings 模块 |
| P3 | 中等页（Modal + Form） | ~8 | announcements / banned / spaces / groups / tags / roles / users / trash |
| P4 | 复杂页（ProTable → DataTable） | 6 | posts / posts-detail / comments / comments-detail / reports / moderation |
| P5 | 收尾 | — | 删 `antd` / `pro-components` 依赖，删 `admin.css` 中 antd DOM 覆盖，跑全量验证 |

P4 是重心：6 个 ProTable/ProDescriptions 页面中，`ProTable` 的 `request` / `actionRef` / `columns` / `rowSelection` / `search` 五件套要映射到 DataTable 的 `useQuery` + `useReactTable`。数据获取从「ProTable 内部管」改为「页面用 `useQuery` 管、DataTable 只做展示」，这与主站取数模式一致。

## 6. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| 7500 行同时改动，中途不可用 | 高 | 按 P1–P5 分批提交，每批 `pnpm --filter @feedchamber/web build` 必须通过 |
| ProTable 能力丢失（列配置、密度切换） | 中 | 这些能力后台实际未启用（无 `columnsState` / `size` 配置），不迁移 |
| 表单校验 111 处规则遗漏 | 高 | 逐页对照现有 `rules` 写 zod schema，迁移后逐页手测必填/长度/一致性强校验 |
| 表格排序/筛选行为变化 | 中 | DataTable 先只支持「受控排序 + 服务端分页」，保持与后端参数契约一致；不做前端排序 |
| 主题/语言切换失效 | 中 | 壳层重建后立即验证 light/dark/中文/English 四态 |
| 主站原语不够用被迫临时改主站 | 中 | 新原语一律放 `components/ui/`，主站可复用；改动主站现有原语需单独说明理由 |

## 7. 验证标准

每批必跑：

```bash
pnpm --filter @feedchamber/tokens verify      # 122 条 token parity 断言
pnpm --filter @feedchamber/web build          # vinext build + tsc --noEmit
```

P5 完成后额外：

1. `grep -r "from 'antd'\|@ant-design" app/web` → 必须为空
2. `app/web/package.json` 中 `antd` / `@ant-design/*` 已删除
3. 起服务，逐页走查 `/admin/**` 全部 32 条路由，light/dark 各一遍
4. 对照主站 `/timeline`：字体、控件高度、圆角、描边、主色用法肉眼一致
5. 表单类页面（用户编辑、版面创建、公告发布）做一次完整增删改
6. 权限门控回归：普通用户访问 `/admin/users` 仍返回 403 / 原路返回

## 8. 工作量估算

| 批次 | 估算 |
|---|---|
| P1 壳层 + 10 个原语 | 2–3 天 |
| P2 简单页 12 个 | 1 天 |
| P3 中等页 8 个 | 1.5 天 |
| P4 复杂页 6 个（含 DataTable 打磨） | 2–3 天 |
| P5 收尾与全量验证 | 1 天 |
| **合计** | **约 8–9 个工作日** |

## 9. 与既有规范的关系

- 主站原语改动遵循 `docs/superpowers/specs/2026-10-18-web-shadcn-refactor-design.md`
- 后台准入与权限逻辑不动，仍以 `openspec/specs/` 与 `app/web/AGENTS.md` 为准
- 后台路由与菜单登记方式不变（`app/admin/<route>/page.tsx` + `pro-shell.tsx` 菜单 + `route-access.ts` 权限码）
