# Tasks

> 提案：[proposal.md](./proposal.md)；规范 delta：[specs/agents-docs/spec.md](./specs/agents-docs/spec.md)。

## 1. 补齐缺失的模块文档（16 篇）

- [x] 1.1 `app/server/internal/gateway/AGENTS.md` — 两段流水线、`Deps`、凭证模型（会话 sid / opaque OAuth token / API Key scope）、自持端点清单、`initRouteMap` 必改规则
- [x] 1.2 `app/web/app/AGENTS.md` — 路由组、页面清单、登录墙两层分工、RSC 页面固定形状
- [x] 1.3 `app/web/app/admin/AGENTS.md` — 纯 CSR 挂载门、三道准入、新增后台模块四步
- [x] 1.4 `app/web/components/AGENTS.md` — 分层规则（谁能 import 谁）、21 个子域一览、取数约定、安全边界
- [x] 1.5 `app/web/components/ui/AGENTS.md` — 原语五条硬规则、文件表、外观验证方式
- [x] 1.6 `app/web/components/admin/AGENTS.md` — 三层壳层区分、菜单登记点、准入分工
- [x] 1.7 `app/web/components/markdown-editor/AGENTS.md` — 三层 XSS 防线、双向 Markdown 契约、草稿键兼容
- [x] 1.8 `app/web/lib/AGENTS.md` — 模块表、三条最易踩规则（guards 导入边界 / QueryClient 每请求 / qk 唯一来源）
- [x] 1.9 `app/web/lib/auth/AGENTS.md` — 状态机、服务端门、不可逾越的 barrel 边界、凭据形态
- [x] 1.10 `packages/AGENTS.md` — 四个共享包、构建顺序、通用约定
- [x] 1.11 `packages/api-client/src/services/AGENTS.md` — 模块形状、方法选择硬约束、新增 service 五步
- [x] 1.12 `deploy/AGENTS.md` — 三条硬约束（NATS 必填 / 9091 只 expose / 6 条爬虫 location）、两档 compose 同步
- [x] 1.13 `scripts/AGENTS.md` — 脚本表与「零依赖优先」约定
- [x] 1.14 `.github/workflows/AGENTS.md` — 四个 workflow、quality 四 job、门禁历史背景
- [x] 1.15 `openspec/AGENTS.md` — 目录、spec 写法、change 提案流
- [x] 1.16 `docs/AGENTS.md` — 入口与结构、写作约定、权威性边界

## 2. 索引与枢纽文档

- [x] 2.1 根 `AGENTS.md` 增加 `<!-- AGENTS-INDEX:BEGIN/END -->` 索引块（38 篇，由脚本生成）
- [x] 2.2 `docs/ARCHITECTURE.md` 层表 gateway 行改指 `gateway/AGENTS.md`
- [x] 2.3 `docs/system-map.md` 层表 gateway 行改指 `gateway/AGENTS.md`；阅读路径指向根索引
- [x] 2.4 修正根 `AGENTS.md`：后台菜单登记点 `admin-shell.tsx` 的 `menuItems`；权限码 53 → 56（按 `DefaultPermissions` 实测）
- [x] 2.5 修正 `app/web/AGENTS.md`：删除已不存在的 `ui-showcase/`；会话 cookie 名改 `master_session`
- [x] 2.6 修正 `docs/system-map.md`：删除 `/ui-showcase` 行；`(app)/` 鉴权描述改为「layout 不兜底」

## 3. 同步门

- [x] 3.1 `scripts/check-agents-docs.mjs`：模块归属（最深的含文档祖先）、四种模式（`--staged` / `--base` / `--check-index` / `--write-index`）
- [x] 3.2 生成物、依赖、根级脚手架排除清单
- [x] 3.3 `--check-index` 额外校验索引链接指向的文件真实存在
- [x] 3.4 `lefthook.yml` pre-commit 加 `agents-docs-hint`（`|| true`，只提示）
- [x] 3.5 `.github/workflows/quality.yml` 新增 `docs` job（`--check-index` 常驻；PR 上 `--base` 阻断；`fetch-depth: 0`）

## 4. 验证

- [x] 4.1 `openspec validate 2026-10-28-agents-doc-coverage --strict` 通过
- [x] 4.2 `openspec validate --all` 通过（归档后重跑）
- [x] 4.3 脚本五条路径自测（临时 git 仓库）：阻断 / 放行 / staged 提示 / 无索引 / 写后校验
- [x] 4.4 本仓库实跑：`--staged` 通过、`--check-index` 通过
- [x] 4.5 `pnpm exec biome check scripts/check-agents-docs.mjs` 通过
