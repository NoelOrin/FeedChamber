# Tasks

## 1. 规范合并（apply）

- [x] 1.1 改写 `openspec/specs/api-style/spec.md`：删除 `Requirement: RPC 风格动词后缀 URL`，新增 `Requirement: HTTP 方法选择与 RPC 风格 URL`；同步 Purpose 描述。
- [x] 1.2 改写 `openspec/specs/api-style/spec.md` 的 `Requirement: 前端 service 请求约定`（写操作 POST + 动词后缀，只读 MAY GET，且与后端方法一致）。
- [x] 1.3 改写 `openspec/specs/frontend-packages/spec.md` 的 `Requirement: api-client 共享包契约` 中「默认 `method: 'POST'`」表述。

## 2. 文档对齐（消除「一律 POST」的矛盾表述）

- [x] 2.1 根 `AGENTS.md`「API 风格约定（强制）」：方法/参数/前端 service 三条改写，删除「现状偏差」告示。
- [x] 2.2 `docs/ARCHITECTURE.md`「API 风格」一条。
- [x] 2.3 `docs/backend-overview.md` §13.1-5 改为「已拍板」；§13.2-30 从「风险」移出并记录决策与依据。
- [x] 2.4 `docs/api-reference.md` 顶部端点数量说明补一句「GET 为只读查询，方法规则见 api-style」。
- [x] 2.5 `docs/design/extension-points.md` 第 5 步新增模块说明。
- [x] 2.6 `packages/api-client/AGENTS.md`「约定」第一条。

## 3. 验证

- [x] 3.1 `grep -rn "一律 \`POST\`\|全部 HTTP 接口使用 POST\|默认 \`method: 'POST'\`" --include=*.md .`（排除 `openspec/changes/archive/`）：剩余命中均为「历史说明」（本 change 正文、backend-overview 决策记录、`docs/superpowers` 历史计划），无「规则仍要求一律 POST」的矛盾表述。
- [x] 3.2 代码零改动核对：`app/server/internal`、`packages/api-client/src`、`packages/sse-client/src`、`packages/shared-utils/src` 无本次改动（既有未跟踪测试文件与并行会话改动的 `app/web/**` 不属本 change）。
- [x] 3.3 路由分类核对：`grep -rn '\.Get(' app/server/internal/router/*.go | wc -l` = 30，逐条归入设计文档 A/B 两类，无第三条类别。

## 4. 归档

- [x] 4.1 把 `openspec/changes/2026-10-18-allow-get-reads/` 移入 `openspec/changes/archive/`，`openspec/changes/` 下不留在途提案。
