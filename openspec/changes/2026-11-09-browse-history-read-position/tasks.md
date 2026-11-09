# Tasks

提案：[proposal.md](./proposal.md) · 规范 delta：[specs/reading-history/spec.md](./specs/reading-history/spec.md)

> 前置：`develop` 工作区当前有未提交改动（email/settings 一批）。本 change 只做**加法 + 三处定点修改**，MUST NOT 与 `2026-11-09-backend-domain-consolidation`（草案，未执行）抢目录结构——所有新文件仍按现状的扁平分层落位，该收拢提案执行时随大流搬迁。
>
> 执行记录（2026-11-09）：代码、契约、测试与模块文档全部完成，仅剩三项**需人工/运行环境**的验收：`feedchamber_test.sh -history` 实跑、`pnpm dev:web` 浏览器走查、openspec 归档。三处与原清单的偏差见文末「实施偏差」。

## 1. 测试先行（RED）

- [x] 1.1 `repository/post_filter_test.go`：钉住 `list_view=new` 新谓词（要求 `last_read_at IS NOT NULL`）✅ 已验证 RED（`does not contain "trs.last_read_at IS NOT NULL"`）
- [x] 1.2 `repository/topic_read_test.go`（新文件，真实 sqlite）：被动浏览只写 `last_viewed_at`、不写 `last_read_at`；空游标上报保留旧值；两路写入互不覆盖；删除幂等
- [x] 1.3 `service/post_read_test.go`：`has_new` 在 `last_read_at == nil` 的行上为 false、`LastContentID` 要能回读；标记已读不再 fallback 写主楼 contentID
- [x] 1.4 `service/history_test.go`：游标上报校验评论归属（不存在/他人帖子的 content → 报错且不落任何记录）
- [x] 1.5 历史列表按 `COALESCE(last_viewed_at, last_read_at) DESC` 排序，存量行（只有 `last_read_at`）不丢
- [x] 1.6 历史列表的可见性/软删除/非公开状态过滤 + `total` 与过滤后一致；另补「读状态三字段回传」用例钉住只写不读的老毛病
- [x] 1.7 删除幂等：删不存在的记录返回成功；`clear` 只影响调用者本人
- [x] 1.8 两个公开详情出口（`GetDetail` / `GetByID`）都自动记浏览；匿名与不可见帖不记
- [x] 1.9 `gateway/permission_test.go`：四条新 protected 端点在真实默认角色权限表下不被默认拒绝（并做了移除映射的变异验证，确实变 403）
- [x] 1.10 `handler/history_test.go`：HTTP 层验统一信封、空 body 容忍、`page`/`pageSize` 透传、游标回传、错误码透传

## 2. 后端落位（GREEN）

- [x] 2.1 `model/topic_read.go`：`LastReadAt` 改 `*time.Time`，新增 `LastViewedAt *time.Time`
- [x] 2.2 `repository/topic_read_sql.go`：upsert 拆成 `Upsert`（已读）与 `RecordView`（浏览）两组 `AssignmentColumns`，空游标不覆盖；新增 `DeleteByUserAndPost` / `DeleteByUser`（幂等）
- [x] 2.3 `repository/topic_read.go` 接口同步；`mock.go` 补 `TopicReadRepository` 全部新方法（并修掉 `ListByUser` 用裸前缀匹配会把 `u1` 命中 `u10` 的老 bug）
- [x] 2.4 `repository/post.go` + `post_sql.go` 增 `ListReadByUser`（INNER JOIN + 复用 `buildPostListWhere` 含 `buildVisibilityWhere`）；`post_filter.go` 改写 `new` 档谓词
- [x] 2.5 `model/post.go`：`PostListItem` 增 `LastContentID` / `LastViewedAt`（后者只对历史出口有意义）
- [x] 2.6 `service/post.go`：`EnrichWithReadState` 填游标、对 `LastReadAt == nil` 短路角标/未读数；`checkCursorForPost` 校验评论归属；`recordBrowse` 钩子被 `GetDetail` 与 `GetByID` 共用
- [x] 2.7 四个出口（`RecordView` / `ListHistory` / `DeleteHistory` / `ClearHistory`）落在 `service/post.go`（与 `MarkRead` 同族，见偏差 §1），接口加进 `post_iface.go`
- [x] 2.8 `handler/history.go`：`List` / `Delete` / `Clear` + swag；`handler/post.go` 增 `RecordView` 并把 `MarkRead` 的 body 注解补齐
- [x] 2.9 `handler/handler.go` 加 `History` 字段与 `New()` 接线（复用 `d.Post`，见偏差 §1）
- [x] 2.10 `internal/router/routes_history.go`：`POST /history/list`、`/history/clear`、`/history/{postId}/delete`；`routes_content.go` 增 `POST /posts/{id}/view`；`router.go` 按域位置插入 `historyRoutes()`；`gateway/permission.go` 同步四条映射
- [x] 2.11 `server/iris.go` **无需改动**：`TopicReadState` 已在 AutoMigrate 列表（`iris.go:80`），新增两列由 AutoMigrate 在启动时补，存量行 `last_read_at` 保留、`last_viewed_at` 为 NULL，靠 COALESCE 排序兼容（见偏差 §2）
- [x] 2.12 `go build ./... && go vet ./... && go test ./...` 全绿

## 3. 契约同步

- [x] 3.1 `make swag` 刷新 `app/server/docs/*` 与根 `openapi.json`；已用二次生成幂等验证同步。`make swag-check` 因工作区本来就有未提交改动而按设计报「stale」，非本 change 引入（见偏差 §3）
- [x] 3.2 `feedchamber_test.sh` 增 `-history` 模块（上报/列表/游标归属校验/幂等删除/清空/匿名被拒）并接入默认与 `-all` 模块表；`bash -n` 语法通过。**实跑需外部 NATS + 已启动的服务器，本环境无 NATS 故未执行**（HTTP 形状由 `handler/history_test.go` 覆盖）
- [x] 3.3 `packages/api-client/src/types.ts` 增 `last_content_id` / `last_viewed_at`；`services/post.ts` 增 `recordView`；新增 `services/history.ts` 并在 `index.ts` 导出

## 4. 前端体验

- [x] 4.1 `components/content/comment-list.tsx`：IntersectionObserver 观察阅读区里最靠下的楼层，防抖 1.2s 上报 `recordView`；只在真实用户输入（wheel/touch/keydown）后才报，避免首帧回调把服务端游标悄悄回退；追加分页时重新登记观察目标
- [x] 4.2 `components/post/post-detail.tsx`：把 `last_content_id` 传给 `CommentList`，进入时 `scrollIntoView` 定位 + `content-item.tsx` 画「上次读到这里」分隔线；「标记已读」带上当前观测到的游标；上报失败静默降级
- [x] 4.3 `app/web/lib/query/keys.ts` 增 `qk.history.*`；新建 `components/history/queries.ts`（RSC 预取与客户端共用同一工厂）；`toPage` 下沉到 `lib/pagination.ts` 并把 `components/user/queries.ts` 改为 re-export，避免复制
- [x] 4.4 `app/web/app/(app)/user/history/page.tsx` + `components/history/history-page.tsx`；顶栏下拉登记「浏览记录」；单条删除与清空二次确认；分页走 nuqs
- [ ] 4.5 `pnpm dev:web` 走查：定位准确、角标语义未变、`/timeline?list_view=new` 仍含只打开过的帖子 — **待人工浏览器走查**（本环境无法交互验证滚动/IntersectionObserver）
- [x] 4.6 `pnpm --filter @feedchamber/api-client build` + `tsc -p tsconfig.check.json` 通过；`biome check` 全绿

## 5. 文档与归档

- [x] 5.1 同步模块 AGENTS.md：`model/`、`repository/`、`service/`、`handler/`、`router/`、`gateway/`、`mock/`、`app/web/app/`、`app/web/components/`、`app/web/lib/`、`packages/api-client/`、`packages/api-client/src/services/`；新建 `app/web/components/history/AGENTS.md`。`node scripts/check-agents-docs.mjs --base HEAD` 与 `--check-index`（46 个文档）均通过
- [x] 5.2 根 `AGENTS.md` 模块说明加「Reading history / 续读位置」一节、路由表加 `/user/history`、API 测试段加 `-history`；`node scripts/check-agents-docs.mjs --write-index` 已重生成索引。另同步 `docs/api-reference.md`
- [x] 5.3 `openspec validate --all` 通过（19/19）
- [ ] 5.4 **待人工验收后**执行 `openspec archive 2026-11-09-browse-history-read-position -y` → 再 `openspec validate --all`（归档会把 `reading-history` 合进 `specs/` 并移入 `changes/archive/`；本会话按提案流约定未自行归档）

## 实施偏差（相对原清单）

1. **不新建 `service/history.go` 与独立 Handler 服务字段**。这四个方法全是对 `topic_read_states` 的读写，和 `MarkRead` / `EnrichWithReadState` 同族；拆一个新 service 只会让 `container` 多一个对象、却仍要注入同一个 `PostRepository`（`ListReadByUser` 在那儿）。落在 `postService` 更符合「域内聚」，`historyHandler` 直接收 `service.PostService`。同理不需要改 `container/container.go` / `server/wiring.go`。
2. **不写手写迁移 SQL**。`topic_read_states` 已在 AutoMigrate 中，新增两列由 GORM 自动补；存量行 `last_viewed_at` 为 NULL，历史排序用 `COALESCE(last_viewed_at, last_read_at)` 兼容，无需数据回填。
3. **分页字段用 `page` / `pageSize`**。`openspec/specs/api-style` 是 SSOT，明确规定分页字段名；仓库里部分旧接口用 `page_size` 属历史漂移，新端点不继续扩散。
4. **`make swag-check` 无法作为「通过」判据**：它对 `git diff` 判定，工作区里本就有未提交的 docs 改动，任何 handler 变更都会红。改用「连续两次 `make swag` 后 md5 不变」证明生成产物与代码同步。
5. **新增 `history` 模块目录后需 `git add -N`**：`scripts/check-agents-docs.mjs` 用 `git ls-files` 发现模块文档，未 track 的新 `AGENTS.md` 不会被索引识别；已用 intent-to-add 让 `--write-index` 生成 46 行正确索引。
