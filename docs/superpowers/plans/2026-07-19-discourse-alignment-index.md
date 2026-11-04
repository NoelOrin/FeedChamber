# Feed Chamber × Discourse 基本能力对齐 — 计划总览

> **For agentic workers:** 本文件是索引。各阶段必须按独立计划执行，不要跨阶段混做。
>
> - 阶段 1: `docs/superpowers/plans/2026-07-19-discourse-alignment-phase1-core-loop.md`
> - 阶段 2: `docs/superpowers/plans/2026-07-19-discourse-alignment-phase2-community-path.md`
> - 阶段 3: `docs/superpowers/plans/2026-07-19-discourse-alignment-phase3-ops-retention.md`
> - 阶段 4: `docs/superpowers/plans/2026-07-19-discourse-alignment-phase4-platformization.md`
>
> 推荐执行技能：`superpowers:subagent-driven-development`（每任务独立 agent）或 `superpowers:executing-plans`。

**Goal:** 把 Feed Chamber 从“可发帖论坛骨架”对齐到 Discourse 级**基本社区能力**（可公开运营、可治理、可留存），而不是复刻完整 Discourse 生态。

**Architecture:** 严格沿用现有分层：Handler → Service → Repository → DB；HTTP 一律 RPC 风格 `POST` + body。优先“接通已有半成品”（审核模型、like API、search API、settings seed、邮件验证码、审计模型），再新建私信/Webhook/Groups 等模块。

**Tech Stack:** Go + Iris v12、GORM/Mongo、SolidJS + TanStack Router、React + Ant Design Pro、`@feedchamber/api-client`、KV Store、EventBus、SSE。

---

## 阶段边界（硬约束）

| 阶段 | 名称 | 交付定义（DoD） | 预估 |
|------|------|-----------------|------|
| 1 | 主路径闭环 | 详情 Markdown 正确；点赞/收藏可点；审核能拦/能批；搜索可用；草稿本地可恢复；邮箱可验证；维护模式可开 | 1–2 周 |
| 2 | 社区主路径 | 私信可用；通知深链/未读正确；话题未读与 active 排序；关闭态；邀请/审批注册；发帖限流；设置 UI/审计/用户批量 | 2–4 周 |
| 3 | 运营与留存 | @提及；watch；屏蔽；2FA；敏感词；SEO URL；Digest；积分榜；徽章 MVP；Webhook | 1–2 月 |
| 4 | 平台化 | Groups/TL；导入备份；插件扩展点；Chat/高级治理（按需） | 持续 |

**依赖顺序：** 1 → 2 → 3 → 4。阶段 1 未完成不得进入阶段 2 的私信/未读等依赖通知与内容状态的工作。

---

## 全局约定

### API 风格（强制）
- 业务接口一律 `POST`，路径动词后缀：`/list` `/create` `/{id}/update` `/{id}/delete` `/{id}/status`
- body JSON：`page` / `pageSize`
- 成功：`pkg.Success` / `pkg.SuccessPaginated`
- 错误：`*pkg.AppError` + `pkg.HandleError`

### 代码落点
- 后端：`app/server/internal/{model,repository,service,handler,router,middleware,container}`
- 前端 Web：`app/web/src/{components,pages,hooks,api}`
- 管理后台：`app/dashboard/src/pages`
- 共享 API：`packages/api-client/src/{services,types,constants}`

### 验证基线
```bash
# 后端
cd app/server && go test ./internal/service/... ./internal/handler/...
# 前端类型
pnpm --filter @feedchamber/api-client build
# 可选全量 API
# bash feedchamber_test.sh -all
```

### 不做清单（YAGNI）
- 不在阶段 1–2 做完整 Discourse 插件运行时
- 不在阶段 1–2 做 Chat / AI bot / SAML / LDAP
- 不在阶段 1 做完整 TL0–TL4 自动晋升
- 不把 Category 模块复活；统一 Space

---

## 执行建议

1. 每个阶段单独开分支：`codex/discourse-phase1-core-loop` 等
2. 每完成一个 Task 提交一次（计划内已给 commit message）
3. 阶段结束后只合并“本阶段 DoD 已满足”的代码
4. 发现半成品与计划不符时，以仓库现状为准，更新该阶段计划再继续

---

## 进度勾选（人工维护）

- [x] 阶段 1 完成并合并 (branch: codex/discourse-phase1-core-loop)
- [x] 阶段 2 完成并合并 (branch: codex/discourse-phase2-community-path)
- [x] 阶段 3 完成并合并
- [x] 阶段 4 平台化完成（Groups/TL/备份导入/扩展点固化；Chat 按 ADR won't do）
  - 分支: `codex/discourse-alignment`，详见 `docs/adr/chat-mvp.md`、`docs/extension-points.md`、`docs/import-discourse.md`
