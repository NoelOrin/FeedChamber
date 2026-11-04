# 壳层插槽（`components/slot/`）

公共壳层（侧边栏 / 顶栏 / 页脚）的具名插槽渲染层。站长在后台「站点自定义 → 插槽」配置条目，
插件通过宿主 API 写入条目，本目录把它们渲染成 DOM。

> 上级：[components/AGENTS.md](../AGENTS.md) · 规范：[openspec/changes/2026-11-04-plugin-hooks](../../../../openspec/changes/2026-11-04-plugin-hooks/specs/shell-slots/spec.md)

## 为什么分成「纯逻辑」与「渲染」两层

`slot-logic.ts` 承载全部判定（合法插槽、合法 kind、props 解析、过滤与排序），
`slot-host.tsx` 只做 kind → JSX 的直接映射。

这样分是因为 **`app/web` 没有测试栈**（无 vitest / jsdom / testing-library）：
为一个 60 行的展示组件引入三件套是过度工程，而真正会出 bug 的恰恰是
「排序对不对、未知插槽会不会炸、未知 kind 会不会拖掉整块」这些判定逻辑。
抽成纯函数后它们可被独立断言；渲染层是 5 个 if 分支的直接映射，靠人工验收。

## 11 个插槽

| 插槽 | 位置 |
|------|------|
| `sidebar.top` | 侧栏最上方，`BackToPrev` 之前 |
| `sidebar.nav.after` | 自定义导航之后、版面面板之前 |
| `sidebar.spaces.before` | 版面面板标题之后、树之前 |
| `sidebar.spaces.after` | 版面树之后、`MotionPanel` 之前 |
| `sidebar.tags.after` | 标签云之后 |
| `sidebar.bottom` | 侧栏最下方 |
| `header.left` | `SiteName` 之后 |
| `header.right.before` | 顶栏右侧操作组开头 |
| `header.right.after` | 顶栏右侧操作组末尾（`UserInfo` 之后） |
| `footer.left` | 页脚版权文字之后 |
| `footer.right` | 页脚右侧状态文字之后 |

**未知插槽名一律返回 null**（不渲染、不报错）。这是「配置写错不该炸页面」的直接体现，
也是 `slot-logic.ts` 的 `isKnownSlot` 存在的原因。

## 五种条目

| kind | props 形状 | 说明 |
|------|-----------|------|
| `text` | `{ text }` | 纯文本 |
| `link` | `{ href, label? }` | 链接；`label` 缺省时退回 `text`/`href` |
| `image` | `{ src, alt? }` | 用原生 `<img>`——src 是站长配置的任意 URL，不受 next/image 域名白名单约束 |
| `divider` | `{}` | 分隔线 |
| `widget` | `{ renderer, ... }` | 由 `renderers/` 注册表决定外观 |

**刻意没有 `html`**：插槽是同源位置，允许任意 HTML 等于允许全站 XSS。
后端 `model.ValidSlotKind` 与服务端 `validateSlotInput` 双重拒绝，前端 `isKnownKind` 再跳过一次。

## widget 渲染器注册表

键是 `props.renderer`（不是 kind）。当前三个：

| renderer | 需要的 props | 降级行为 |
|----------|-------------|---------|
| `list` | `{ items: string[] }` | 空数组返回 null |
| `stats` | `{ value, label? }` | 无 value 返回 null |
| `markdown` | `{ markdown }` 或 `{ text }` | 空文本返回 null |

**未注册的 renderer 退回 `fallback`**（纯文本展示 `props.text`），而不是整块消失。
未知 renderer 是配置错误，但一条配置错不该让整个插槽区域空白。

`markdown` 渲染器刻意**不复用** `components/content/markdown-renderer.tsx`：
那条链路支持图片、代码块、外链卡片，对插槽这种小面板过重，且插槽内容来自
插件/站长，渲染面积越小越好。它只做 `whitespace-pre-wrap` 的纯文本换行。

## 取数

`publicSlotsOptions` 指向 `POST /slots/public`，`staleTime` 5 分钟。
插槽不是高频变化的数据（插件写入是 `cron.daily` 级别的），没必要频繁拉；
与 `publicCustomizationOptions` 同一量级。

**渲染路径不调用插件**：后端 `ListForViewer` 只读库。插件内容由插件在异步钩子里
调用 `ctx.slots.set()` 写入，写入后按既有渲染流程生效。这条是 spec 的硬约束——
渲染路径上任何同步跨进程调用都会把插件故障变成页面故障。

## 改这里要注意

- 新增插槽名：**四处同一次改动**——后端 `model.AllSlots`、`slot-logic.ts` 的 `SLOTS`、
  壳层组件的 `<PublicSlot name="…" />`、本文件的上表。漏一处会让该插槽永远不渲染且不报错。
- 新增 kind：后端 `model.SlotKinds` + `ValidSlotKind`、`slot-logic.ts` 的 `KINDS`、
  `slot-host.tsx` 的渲染分支、本文件。**先确认它真的需要新增**：能用 widget + renderer 表达的
  就不要加 kind，kind 是跨端契约（后端存什么、前端怎么渲染要一起改）。
- 新增 renderer：只加 `renderers/` 一个文件 + `RENDERERS` 一行。**不需要动后端**——
  renderer 名存在 `props` 里，后端只当作不透明 JSON。
- 不要给 `SlotHost` 加「同步取数」的分支。它的 `entries` 必须是已就绪的数据；
  取数在调用方（`PublicSlot` 的 `useQuery`）。
- 不要移除 `sort` 相同时的 `id` 兜底排序：`sort` 相同的条目若无稳定次序，
  每次渲染顺序抖动会让 `useQuery` 的缓存命中看起来像「数据变了」。

## 本文件何时更新

- 插槽枚举增减（上表 + `slot-logic.ts` 的 `SLOTS` + 后端 `model.AllSlots`）
- 条目 kind 增减（上表 + `KINDS` + `slot-host.tsx` 分支）
- widget 渲染器增减（上表 + `RENDERERS`）
- 取数端点或 `staleTime` 变化
- 「渲染路径不调用插件」这条约束被改变（那是 spec 变更，需走 openspec 提案）
