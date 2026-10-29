## Approach

本变更是**规范迁移**，不改变任何运行行为。策略：将现有散落规范（根 `AGENTS.md`、各层子 `AGENTS.md`、`docs/ARCHITECTURE.md`）提炼为 6 个 OpenSpec capability spec，作为 spec-driven 的单一可信来源。

## Decisions

- **规范真相源迁移到 `openspec/specs/`**：后续任何规范变更 MUST 经 change 提案流（proposal → specs → design → tasks → archive），不再直接改散文 `AGENTS.md`。
- **保留 `AGENTS.md` 为项目总览**：根 `AGENTS.md` 与 `docs/ARCHITECTURE.md` 继续作为入口与导航，规范冲突时以 `openspec/specs/` 为准。
- **历史文档归口**：`docs/adr/`（ADR）、`docs/plans/`（对齐计划与轮次）、`docs/reviews/`（评审复盘）保留原位，作为决策与历史归档，不在本变更转为 active change。
- **风格一致性**：spec 使用 SHALL/MUST 规范表述，每个 Requirement 至少包含一个 WHEN/THEN 场景。

## Risks

- 规范在 `AGENTS.md` 与 `openspec/specs/` 间可能短暂双写；后续应以 specs 为准逐步收敛。
- 本次未覆盖 `docs/design/`、`docs/reviews/`、`docs/plans/` 中更细的实现说明，仅迁移稳定规范层。
