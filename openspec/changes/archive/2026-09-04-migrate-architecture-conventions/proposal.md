## Why

Feed Chamber 的架构与编码规范散落在根目录 `AGENTS.md`、各层子 `AGENTS.md`（`handler/service/repository`）、`docs/ARCHITECTURE.md`、`docs/adr/`、`docs/plans/` 等多处，缺乏单一可校验的规范真相源。团队与 AI 协作时难以快速确认"什么是已采纳的规范、什么是进行中的变更"。

本变更把现有稳定规范迁移进 OpenSpec，使其成为 spec-driven 的单一可信来源：规范以 `specs/<capability>/spec.md` 表达，后续变更经 change 提案流演进，而非直接改散文文档。

## What Changes

- 新增 6 个 capability spec，覆盖现有稳定规范：
  - `api-style` — RPC 风格 API 约定（强制 POST + 动词后缀 URL + body 传参 + 统一响应）
  - `layered-architecture` — 后端单向分层依赖链（Middleware → Handler → Service → Repository → DB）
  - `backend-service-design` — 关键设计（Repository 双实现、统一响应、依赖装配、权限系统、路由组织）
  - `module-conventions` — 业务模块分层文件约定与新增模块 checklist
  - `frontend-packages` — 前端共享包（api-client / sse-client / shared-utils）约定
  - `operations` — 环境变量、运行、API 测试流程
- 将 `docs/ARCHITECTURE.md` 定位为 OpenSpec specs 的导航入口；根 `AGENTS.md` 保留为项目总览并指向 OpenSpec。
- 历史性决策与轮次记录（`docs/adr/`、`docs/plans/`、`docs/reviews/`）保留原位作为 ADR 与历史归档，不在本变更转为 active change。

## Capabilities

### New Capabilities

- `api-style` — RPC 风格 HTTP API 约定
- `layered-architecture` — 后端单向分层架构
- `backend-service-design` — 后端关键服务设计约束
- `module-conventions` — 模块文件组织与新增模块流程
- `frontend-packages` — 前端共享包契约
- `operations` — 运行、配置与测试流程

### Modified Capabilities

<!-- 无既有 spec 被修改；本变更为纯迁移 -->

## Impact

- 文档：新增 `openspec/specs/` 下 6 个规范文件；`openspec/config.yaml` 已初始化（schema: spec-driven）。
- 不改动任何业务代码、API 行为或数据库结构。
- 现有 `AGENTS.md` / `docs/*` 继续保留为入口与补充参考，规范冲突时以 `openspec/specs/` 为准。
