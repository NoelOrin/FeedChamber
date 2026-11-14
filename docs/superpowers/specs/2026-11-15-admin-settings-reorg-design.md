# 后台设置页重组设计

日期：2026-11-15
状态：待审查
范围：纯前端展示层重组（`app/web/app/admin/settings/`），**不改任何设置 key / 默认值 / 类型 / 后端**

## 1. 背景与问题

当前后台 `/admin/settings` 有 5 个顶部 Tab，40+ 个设置项散落其中，管理员反馈「不清晰、不快捷」。具体问题：

1. **归类错位**：邮件服务塞在「用户与认证」、MCP 塞在「站点」、上传与 SSE 塞在「内容」，管理员需要「猜」去哪个 Tab 找。
2. **OAuth 组超长**：5 个平台（GitHub/Google/QQ/微信/微博）共 21 个字段堆在一个可折叠组里，一屏拉不完。
3. **无法搜索**：找某一个开关只能在 5 个 Tab 间来回翻。
4. **整页保存**：改任意一项都要滚到页首点「保存设置」，且一次提交全部字段，误存风险大。

## 2. 目标

- 按功能域重排 Tab 与分组，消除错位归类。
- OAuth 按平台拆折叠子块，每个平台可独立展开/保存。
- 顶部提供搜索框，跨 Tab 一键定位任意设置项。
- **每个分组一个「保存」按钮**，只提交本组字段。
- **切换时有未保存改动需二次确认**。

## 3. 顶层 Tab（6 个）

从顶部横向 Segmented 切换，顺序即展示顺序：

| # | Tab key | Tab 名 | 包含分组 |
|---|---------|--------|----------|
| 1 | `site` | 站点与 SEO | 站点信息、SEO 设置、MCP Server |
| 2 | `user` | 用户与准入 | 用户与注册、安全与高级认证（API Key / MFA） |
| 3 | `content` | 内容与审核 | 发帖与评论、上传设置 |
| 4 | `notify` | 通知与邮件 | 邮件服务、SSE / 通知 |
| 5 | `oauth` | 第三方登录 | OAuth 第三方登录（按平台折叠）、OAuth2 / OIDC |
| 6 | `storage` | 存储 | 存储设置 (S3) |

### 迁移映射（相对现状）

| 分组 | 现 Tab | 新 Tab |
|------|--------|--------|
| 站点信息 / SEO 设置 / MCP Server | site | site（不变） |
| 用户与注册 / 安全与高级认证 | user | user（不变） |
| OAuth2 / OIDC | user | **oauth**（迁出） |
| 邮件服务 | user | **notify**（迁出） |
| 发帖与评论 / 上传设置 | content | content（不变，但上传单独成组） |
| SSE / 通知 | content | **notify**（迁出） |
| OAuth 第三方登录 | oauth | oauth（不变，内部按平台拆） |
| 存储设置 (S3) | storage | storage（不变） |

## 4. 搜索框

- 位置：Tab 栏同一行右侧（或 Tab 下方独立一行），输入即筛选。
- 匹配范围：所有分组的 `key` 与 `description`（不区分大小写）。
- 命中时：隐藏正常 Tab 分组视图，改为「搜索结果」平铺展示，每项标注其所属 Tab 名；控件直接可编辑，且归属到其原分组的保存逻辑。
- 清空搜索：恢复当前 Tab 分组视图。
- 搜索结果内的编辑同样计入对应分组的「未保存改动」；保存按钮出现在结果项所属分组处或搜索结果顶部按组聚合（实现取简单一致者：搜索结果按分组聚合，每组带自己的保存按钮）。

## 5. 折叠与表单粒度

- 每个分组是一个 `Collapse` 块（沿用现有 `components/ui/collapse`）。
- **OAuth 第三方登录**组内部再按平台拆 5 个折叠子块：GitHub / Google / QQ / 微信 / 微博。每个子块 = {@平台}_enabled、client_id、client_secret、callback。
- **每个分组（OAuth 每个平台子块视为一个分组）底部一个「保存」按钮**：只提交本分组包含的 key。

## 6. 保存逻辑

- 分组内维护 `dirty` 状态（本组任一字段被改 → dirty=true）。
- 保存按钮：只 `batchUpdate` 本组 key 的 payload；成功后清本组 dirty、toast「已保存」、invalidate settings 缓存。
- 全局不再有整页保存按钮。
- 未改动时保存按钮 disabled（可选：dirty 才可点，避免空提交）。

## 7. 切换二次确认

- 触发：切换顶部 Tab、或展开/收起导致当前编辑区离开时，若**当前视图中任一分组存在 dirty**，弹出确认框。
- 确认框文案：「有未保存的更改，确定离开吗？」选项：取消 / 仍要离开（离开则丢弃 dirty，不回滚已保存值）。
- 无 dirty：直接切换，不弹。
- 复用 `components/ui/confirm-dialog.tsx` 的 `useConfirm`。
- 浏览器原生 beforeunload 不在本次范围（可选增强）。

## 8. 改动文件

| 文件 | 改动 |
|------|------|
| `app/web/app/admin/settings/settingsConfig.ts` | `ServiceTabKey` 增 `notify`；`serviceTabs` 改 6 项；各 `SettingsGroup.tab` 按映射调整；OAuth 组拆平台（用 group.name 或新增 platform 标识）；新增可选 `subgroups` 结构支撑 OAuth 平台折叠 |
| `app/web/app/admin/settings/page.tsx` | 顶部搜索框；按分组渲染独立保存按钮；dirty 跟踪；Tab 切换二次确认；搜索结果视图 |

不改：后端、`settingService` 接口、任何 key 名与默认值。

## 9. 风险与兼容

- key/默认值/类型零改动 → 已保存数据完全兼容。
- `?tab=` 深链：现有 `site/user/content/oauth/storage` 中 `user` 少了 OAuth2 TTL、`content` 少了 SSE/上传（迁到 notify）——属展示层变化，旧 `?tab=` 仍有效。新增 `?tab=notify`。
- 无 SSR 影响（后台纯 CSR）。
- 分组独立保存减少整页误提交风险。

## 10. 验收

- 6 个 Tab 分组正确，邮件在「通知与邮件」、OAuth2 TTL 在「第三方登录」、上传与 SSE 归位。
- OAuth 5 平台各自折叠、各自可存。
- 每个分组有独立保存按钮，只提交本组。
- 搜索能跨 Tab 命中并可直接编辑。
- 有未保存改动切 Tab 弹二次确认；无改动不弹。
- 匿名 timeline 等既有功能不受影响（本改动不涉后端）。
