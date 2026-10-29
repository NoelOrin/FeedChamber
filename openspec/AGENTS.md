# OpenSpec（`openspec/`）

规范驱动开发的单一可信来源。**已采纳的稳定规范以 `specs/` 为准**；规范冲突时 specs 优先，后续变更须经 change 提案流。

> 上级：[根 AGENTS.md](../AGENTS.md) · 相关：[docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)、[docs/AGENTS.md](../docs/AGENTS.md)

## 目录

| 路径 | 说明 |
|------|------|
| `specs/<capability>/spec.md` | **已采纳规范**。每个 capability 一个目录，一个 `spec.md`，按 `## Purpose` + `## Requirements` 组织 |
| `changes/` | 活跃 change（当前为空；`2026-10-28-trust-level-xp-system` 与 `2026-10-29-post-moderation-modes` 均已归档）。每个 change 一个目录：`proposal.md` + `tasks.md` + `specs/<capability>/spec.md`（delta） |
| `changes/archive/YYYY-MM-DD-<name>/` | 已归档 change，附带 `.openspec.yaml`（`schema` + `created`） |
| `config.yaml` | OpenSpec 项目配置（`schema: spec-driven`） |

## 现有 capabilities

`access-gating` · `agents-docs` · `api-style` · `backend-service-design` · `community-motion` · `frontend-packages` · `layered-architecture` · `module-conventions` · `observability` · `operations` · `post-moderation` · `seo` · `trust-levels`

## 规范写法（spec.md）

```markdown
# <capability> Specification

## Purpose
一段话说清这个 capability 约束什么。

## Requirements
### Requirement: <一句话标题>

规范正文：用 **MUST** / **MUST NOT** / **SHOULD** / **MAY** 表达强度，给出具体约束。

#### Scenario: <场景名>

- WHEN <条件>
- THEN <期望>
- AND <附加期望>
```

- **Requirement 标题用陈述句**，不写「需求一」这种编号。
- **每个 Requirement 至少一个 Scenario**（`openspec validate --strict` 会检查结构）。
- 规范里引用真实模块与真实路径，不要写已下线模块（`Category` 已由 `Space` 承接）。

## 变更流程（change 提案流）

```bash
openspec list                      # 看活跃 change
# 手工建 changes/<YYYY-MM-DD>-<name>/{proposal.md,tasks.md,specs/<cap>/spec.md}
openspec validate --all            # 校验（--strict 更严）
openspec archive <name> -y         # 归档：把 delta 合进 specs/，移到 changes/archive/
openspec validate --all            # 归档后再校验一次
```

- **delta spec 用 `## ADDED Requirements` / `## MODIFIED Requirements` / `## REMOVED Requirements`**，不是完整规范。
- `proposal.md` 写 `## Why` + `## What Changes`（+ 可选 `## Non-goals`）。
- `tasks.md` 是**可勾选的实施清单**（`- [ ]` / `- [x]`），提案链接写在文件头。
- 归档后 `specs/` 里会出现新的 capability 目录——**同时更新根 `AGENTS.md` 的模块文档索引**（`node scripts/check-agents-docs.mjs --write-index`）。

## 改这里要注意

- **不要直接改 `specs/` 来改规范**：那等于绕过提案流。规范变更走 change。
- **`openspec validate --all` 必须通过**（当前 13 个 spec 全绿）。新增或归档后都跑一次。
- 归档用 `openspec archive <name> -y`，**不要手工搬目录**——它会同时处理 spec 合并与校验。
- 纯工具/纯文档类 change 可以 `openspec archive --skip-specs`。

## 本文件何时更新

- 新增一个 capability 目录
- 变更流程（validate / archive 命令或目录约定）变化
- `config.yaml` 的 schema 变化
