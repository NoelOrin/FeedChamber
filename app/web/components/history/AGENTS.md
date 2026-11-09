# 浏览记录（components/history/）

浏览记录（`/user/history`）的域组件与取数契约：列表页是客户端组件，RSC 首屏在 `app/(app)/user/history/page.tsx` 预取后经 `HydrationBoundary` 水合。

> 上级：[components/AGENTS.md](../AGENTS.md) · 路由：[app/(app)/user/history/page.tsx](../../app/(app)/user/history/page.tsx) · 取数：`@feedchamber/api-client` 的 `historyService` · query key：[lib/query/keys.ts](../../lib/query/keys.ts)（`qk.history`）

## 文件表

| 文件 | 职责 |
|------|------|
| `queries.ts` | `historyListOptions(page, pageSize)`（导出 `queryOptions`）与 `totalPages(total, pageSize)`。query key 取 `qk.history.list(params)`，`queryFn` 调 `historyService.list({ page, pageSize })` |
| `history-page.tsx` | `'use client'` 列表页：列表 / 骨架 / 空态三态、单条删除（确认弹窗 `Dialog`）、清空（确认弹窗）、`Pager` 翻页（nuqs `?page=`）；删除/清空成功后 `invalidateQueries({ queryKey: qk.history.all })` |

## 约定

- **RSC 与客户端共用同一份 `queryOptions`**：`(app)/user/history/page.tsx` 调 `prefetchQuery(historyListOptions(...))`，`history-page.tsx` 调 `useQuery(historyListOptions(...))`。两侧必须用同一个工厂，否则水合后首帧立刻重发一次请求。
- **分页字段用 `page` / `pageSize`**（openspec `api-style` 的规范名），**不是**旧 `ListParams.page_size`；页大小取 `~/lib/pagination` 的 `DEFAULT_PAGE_SIZE`，页号经 `toPage(searchParams.page)` 归一。
- **列表只含「访问者现在还看得见」的帖子**：被删除、进回收站或等级不足的一律不出现（可见性谓词在 SQL 里，与列表 / 详情同一套），因此 `total` 小于历史真正读过的数量属正常，不是丢数据。展示时 `last_viewed_at ?? last_read_at` 与后端排序用的 `COALESCE` 对齐；`last_content_id` 非空时在行内标注「已记住阅读位置」。
- **`delete(postId)` 幂等**：本就没有这条记录也返回成功（`POST /history/{postId}/delete`）；**`clear()` 只清当前用户自己的记录**（`POST /history/clear`），不影响任何帖子或其他用户。两者 actor 由后端从会话派生，前端**不传 user id**。
- 三条端点均为 POST + 动词后缀；actor 身份与可见性都由后端裁定，前端只做 UX 门控。

## 改这里要注意

- 新增 / 改动端点或字段要同步 `packages/api-client/src/services/history.ts`、`packages/api-client/src/types.ts`（`PostListItem.last_content_id` / `last_viewed_at`）与 `qk.history`。
- 续读游标（`resumeFrom` / `cursorRef`）的上报与「上次读到这里」分隔线在 `components/content/` 与 `components/post/`，不在这个域；改这里不要动那条链，见 [components/AGENTS.md](../AGENTS.md) 的 2026-11-09 一节。
