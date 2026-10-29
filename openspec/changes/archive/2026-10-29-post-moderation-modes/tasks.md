# Tasks — 文章审核模式

提案：[proposal.md](proposal.md)

## 1. 模型与设置

- [x] `model/moderation.go`：`ModerationMode*` 常量、`DefaultModerationMode=async`、`ParseModerationMode`（读取侧回落）、`ValidModerationMode`（写入侧校验）
- [x] `model/setting.go`：`SettingModerationMode` 常量 + `DefaultSettings["moderation_mode"]="async"`；`post_approval` 保留为 `manual` 的兼容别名
- [x] `service/setting.go`：`Set` / `BatchSet` 拒绝非法模式值（`2xxx`）

## 2. 发帖路径

- [x] `service/post.go`：`Create` 按四模式分支（`off` / `async` / `external` / `manual`），旧 `post_approval=true` → `manual`
- [x] `off` 模式跳过 TL0 强制待审门；其余模式保留
- [x] 待审必留痕：`manual` / `tl0` / `external` 判定都会建审核单，并写入 `strategy`

## 3. 审核服务

- [x] `service/moderation.go`：`Mode(ctx)` 读设置并回落默认值
- [x] `CheckExternal(ctx, ...)`：发布前 HTTP 调用 + 3s 超时 + Bearer；`allow` / `flag` / `block` / `replace`；不可用返回 `errExternalModerationUnavailable`
- [x] `RecordPendingFrom(..., strategy)`：标注判定来源
- [x] `SubscribeAsync(bus)` + `handlePostCreated`：消费发帖事件，仅处理 `moderation_async=true` 且当前仍 `approved` 的帖子，命中转 `pending`
- [x] `container/runtime.go`：`Start` 时 `ModerationSvc.SubscribeAsync(EventBus)`

## 4. 前端

- [x] `packages/api-client/src/constants.ts`：`SettingKey.MODERATION_MODE` + `ModerationMode` 枚举
- [x] `app/web/app/admin/settings/settingsConfig.ts`：「发帖与评论」用四选一下拉替换 `post_approval` 布尔开关
- [x] `app/web/app/admin/moderation/page.tsx`：顶部模式卡片，可就地切换并失效设置缓存

## 5. 验证

- [x] `go build ./...` / `go vet ./internal/service/ ./internal/container/ ./internal/model/`
- [x] `internal/service`：四模式初态、external 结论与回退、异步命中转待审、旧开关兼容
- [x] `pnpm --filter @orinforum/api-client build` + `app/web` 的 `tsc -p tsconfig.check.json`

## 6. 文档

- [x] 模块 AGENTS.md：`model` / `service` / `container` / `app/web/app/admin` / `packages/api-client`
- [x] `openspec/AGENTS.md` 活跃 change 列表
