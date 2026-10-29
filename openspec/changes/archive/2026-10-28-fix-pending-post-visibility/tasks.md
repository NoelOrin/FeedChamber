# Tasks — 修复「发帖成功但全站不可见」

提案：[proposal.md](proposal.md) · 规范 delta：[specs/post-moderation/spec.md](specs/post-moderation/spec.md)

## 1. 后端：信任门默认关闭

- [x] 1.1 `internal/model/setting.go` 的 `DefaultSettings` 增加 `"trust_tl0_require_approval": "false"`
- [x] 1.2 `internal/service/trust.go` 的 `ShouldForceApproval` 硬默认值由 `"true"` 改为 `"false"`
- [x] 1.3 补测试：默认配置下 TL0 用户创建的帖子 `status=approved`

## 2. 后端：列表状态过滤哨兵

- [x] 2.1 `internal/service/post.go` 的 `List` 识别 `__all__`：有 `report:manage` 时不过滤状态，否则回退 `approved`/`closed`
- [x] 2.2 `internal/service/report.go` 的 `List` 同样识别 `__all__`
- [x] 2.3 补测试：特权调用者 `__all__` 能拿到 `pending`，匿名调用者拿不到

## 3. 后端：待审帖可见性与审核留痕

- [x] 3.1 `internal/handler/post.go` 的 `GetByID` / `GetDetail` 注入 `service.WithViewerID`
- [x] 3.2 `internal/service/post.go` 增加作者例外（`AuthorID == viewerID` 时可读非公开帖）
- [x] 3.3 `internal/service/post.go` 去掉 `RecordPending` 的 `approval == "true"` 前置条件（待审必留痕）
- [x] 3.4 补测试：作者可读自己的 `pending` 帖，他人 404

## 4. 后端：状态变更端点

- [x] 4.1 `internal/service/post.go` 增加 `SetStatus(ctx, id, status)`（校验状态取值，非法值返回 `INVALID_PARAMS`）
- [x] 4.2 `internal/handler/post.go` 增加 `SetStatus` handler（swag 注解）
- [x] 4.3 `internal/router/routes_content.go` 注册 `POST /posts/{id}/status`
- [x] 4.4 `internal/gateway/permission.go` 增加 `addPattern("/api/v1/posts/{id}/status", "report:manage")`
- [x] 4.5 `make swag` 重新生成 `docs/` 与 `openapi.json`

## 5. 前端：后台「通过 / 拒绝」入口与反馈

- [x] 5.1 `packages/api-client/src/services/post.ts` 增加 `setStatus(id, status)`
- [x] 5.2 `app/web/app/admin/posts/page.tsx` 增加行内「通过 / 拒绝」操作（写后失效 `qk.admin.posts.all`）
- [x] 5.3 `app/web/components/post/composer.tsx`：`status=pending` 时提示「已提交审核」并跳详情
- [x] 5.4 `app/web/components/post/post-detail.tsx`：`status=pending` 时展示「审核中」标识

## 6. 文档

- [x] 6.1 更新受影响的模块 `AGENTS.md`（service / handler / router / gateway / model / web components / admin / api-client services）
- [x] 6.2 `node scripts/check-agents-docs.mjs --check-index` 通过

## 7. 验证

- [x] 7.1 `cd app/server && go build ./... && go vet ./... && go test ./...`
- [x] 7.2 `cd app/web && pnpm typecheck && pnpm check`
- [x] 7.3 独立复现：默认配置下 TL0 发帖 → 公开列表可见；`__all__` 不向匿名者泄露非公开帖
- [x] 7.4 存量帖救援路径确认：后台「帖子管理 → 待审」能看到并可「通过」
