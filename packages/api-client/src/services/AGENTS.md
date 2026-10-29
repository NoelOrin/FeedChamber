# API Service 模块（`packages/api-client/src/services/`）

后端每个业务域对应一个 service 模块（40 个文件 / 1442 行）。**这里是前端唯一允许发起 HTTP 的地方**。

> 上级：[api-client/AGENTS.md](../../AGENTS.md) · 请求层：[client.ts](../client.ts) · 类型：[types.ts](../types.ts) · 常量：[constants.ts](../constants.ts)

## 文件清单（按后端域）

`admin` `announcement` `gate` `api-key` `auth` `backup` `badge` `banned` `checkin` `content` `dashboard` `draft` `favorite` `follow` `group` `invite` `leaderboard` `like` `mfa` `moderation` `motion` `notification` `oauth` `oauth-client` `permission` `plugin` `pm` `post` `report` `revision` `role` `security` `session` `setting` `space` `tag` `theme` `trust` `upload` `user` `user_block` `watch` `webhook`

## 模块形状

```ts
import { get, post, postPaginated } from '../client';
import type { PostListItem, ListParams } from '../types';

export const postService = {
  list(params: ListParams & { tag_name?: string }) {
    return postPaginated<PostListItem[]>('/posts/list', params);   // 列表 → 分页
  },
  getById(id: string) {
    return get<PostDetail>(`/posts/${id}`);                        // 只读单资源 → GET
  },
  create(body: CreatePostBody) {
    return post<Post>('/posts/create', body);                      // 写操作 → POST
  },
  setStatus(id: string, status: PostStatus) {
    return post<null>(`/posts/${id}/status`, { status });          // 状态流转 → POST /{id}/status
  },
};
```

## gate.ts（门控规则）

`gateService` 覆盖 `/admin/gates/*`：`meta`（域/字段/操作符/动作/窗口指标）、`listRules` / `saveRule` / `deleteRule`、`listDefaults` / `saveDefault`、`simulate`（与线上同一 `Evaluate`）。`GateRule.conditions` 与 `action_params` 是 JSON 字符串，表单层负责解析/序列化。

## trust.ts（XP / 等级）

`trustService` 覆盖后台成长体系与公开等级表：`getPublicLevels`（`GET /levels`，无需登录）、`listLevels` / `saveLevel` / `deleteLevel`、`listXPRules` / `saveXPRule`、`listStreakBonuses` / `saveStreakBonus` / `deleteStreakBonus`、`recomputeAll`、`getUserXP`（指标分解）、`adjustXP` / `listAdjustments` / `deleteAdjustment`、`setUserLevel`。类型（`TrustLevel` / `XPRule` / `XPStreakBonus` / `XPBreakdown` / `XPAdjustment`）也定义在本文件并从 `index.ts` 导出。

## 方法选择：与后端严格一致

| 场景 | 用哪个 | URL 形态 |
|------|--------|----------|
| 列表 / 分页 / 多条件筛选 | `postPaginated` / `post` | `/xxx/list`，参数走 body |
| 写操作（创建/更新/删除/状态） | `post` | `/xxx/create`、`/xxx/{id}/update`、`/xxx/{id}/delete`、`/xxx/{id}/<action>` |
| 只读单资源 | `get` | `/xxx/{id}`、`/xxx/status` |
| 文件/协议类 | `get` 或特殊处理 | `/upload/{id}`、OAuth/OIDC 端点 |

**硬约束**（来自 `openspec/specs/api-style`）：

- 写操作 **必须 `POST`**，URL 带显式动词后缀（`/spaces/create`，不是 `/spaces/`）。
- **禁止** `PUT` / `PATCH` / `DELETE` 用于业务接口。
- 同一读取路径**不得**同时提供 `GET` 与 `POST` 两个入口。
- `POST` **不得**用 query string 承载业务参数（一律 body）。
- 分页字段固定 `page` / `pageSize`。

前端 service 与后端方法**必须一致**。发现不一致时以后端 router 为准，改 service。

## 返回类型

- `post<T>` / `get<T>`：自动解包 `{ code, msg, data }` 的 `data`。
- `postPaginated<T>`：返回 `{ data: T; total: number }`（`data` 即 items 数组；`reportService.list` 这类走 `post<{ items; total }>` 的模块自己声明形状）。
- 后端错误抛 `ApiError`（业务码）/ `HttpError`（HTTP status）/ `AuthError` / `NetworkError`（`errors.ts`）。UI 用 `error-view.ts` 的 helper 映射文案。

## 新增一个 service（后端加了新模块）

1. 在 `src/types.ts` 加响应类型（若 `types.ts` 里已有同名可复用）。
2. 新建 `src/services/<domain>.ts`，按上面的形状导出 `<domain>Service`。
3. 在 `src/index.ts` 加一行 `export { xxxService } from './services/xxx'`（有类型导出的连 `export type` 一起）。
4. 若新增权限码 / 设置 key / 错误码 → 同步 `src/constants.ts` 的 `PermissionCode` / `SettingKey` / `ErrorCode`。
5. `pnpm --filter @orinforum/api-client build` 后 `cd app/web && pnpm exec tsc -p tsconfig.check.json`。

> `.agents/skills/openapi-to-service/` 有从 `openapi.json` 生成 service 的工具，批量对齐时可用；生成后仍需人工核对方法选择与 URL 后缀。

## 改这里要注意

- **不要在页面里直接 `fetch`**：所有请求经 service → `client.ts`，否则会绕过 401 单飞刷新与超时。
- **不要在 service 里做业务判断**（如「帖子是不是我的」）：那是后端 service 层的事，前端只做 UX 门控。
- **类型不要写 `any`**：`biome.json` 关了 `noExplicitAny` 但类型是跨包契约，`any` 会把错误推迟到运行时。
- **`user_block.ts` 是唯一带下划线的文件名**（历史遗留），新增文件用 kebab-case。
- `constants.ts` 的 `PermissionCode` 必须与后端 `model/permission.go` 的 `DefaultPermissions` 对齐——两边不一致会让后台权限门与后端 403 打架。

## 本文件何时更新

- 新增/删除一个 service 模块
- 请求层 API（`get` / `post` / `postPaginated`）变化
- 方法选择约定变化（理论上只在 `openspec/specs/api-style` 变更后）
