# API Client 前端 API 共享包

Web 和 Dashboard 共同依赖，封装所有后端 API 调用。

## 结构

| 文件 | 说明 |
|------|------|
| client.ts | `authFetch` 封装：自动 Bearer token、401 自动刷新重试、默认超时 3s |
| types.ts | 所有 API 响应类型定义（`User`, `PostListItem`, `ContentListItem`, `AdminStats`, `SearchResults`, `EntitySearchResult<T>` 等） |
| constants.ts | 类型化常量（`ErrorCode`, `PermissionCode`, `SettingKey`, `ModerationMode`, `RoleName`, `PostStatus`, `STATUS_FILTER_ALL`, `API_BASE_URL`）。`ModerationMode`（`off` / `async` / `external` / `manual`）与后端 `model.ModerationMode*` 一一对应，**默认 `async`** |
| services/auth.ts | 认证 API |
| services/post.ts | 帖子 API (含 listByIds 批量查询、setFeatured 精华) |
| services/content.ts | 评论 API |
| services/user.ts | 用户 API |
| services/admin.ts | 管理后台 API |
| services/trust.ts | XP / 等级后台 API（等级定义、XP 规则、连续签到阶梯、指标分解、人工调整） |

| services/tag.ts | 标签 API |
| services/like.ts | 点赞 API |
| services/favorite.ts | 收藏 API |
| services/follow.ts | 关注 API |
| services/upload.ts | 上传 API |
| services/notification.ts | 通知 API |
| services/report.ts | 举报 API |
| services/checkin.ts | 签到 API |
| services/announcement.ts | 公告 API |
| services/oauth.ts | OAuth API |
| services/space.ts | 版面 API |
| services/setting.ts | 设置 API |
| services/role.ts | 角色 API |
| services/permission.ts | 权限 API |

## 约定

- 写请求 `method: 'POST'` 且 URL 带显式动词后缀（`/spaces/create`、`/spaces/{id}/delete`）；只读查询用 `get`，且必须与后端方法一致（同一路径不留 GET/POST 双读入口）
- 响应类型固定 `Response<T>` 结构
- `post<T>(url, body?)` 泛型函数自动解包 `data` 字段
- `postPaginated<T>(url, body?)` 返回 `{ data: T; total: number }`（`data` 即 items 数组）

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。

## 常量（2026-10-30）

`SettingKey` 新增 `MCP_ENABLED` / `MCP_DISABLED_TOOLS` / `MCP_MAX_RESULT_BYTES`——它们与后台 `app/admin/settings/settingsConfig.ts` 的白名单必须成对维护：漏登记就没有任何 UI 能改到该设置。
