# ADR: Chat MVP — won't do（阶段 4）

日期: 2026-08-22  
状态: decided — **won't do** in Phase 4  
关联: `docs/superpowers/plans/2026-07-19-discourse-alignment-phase4-platformization.md` Task 6

## 决策
阶段 4 **不实现**站内实时群聊（Chat 频道）独立模块。

## 原因
- Discourse Chat 是可选重型能力，非“基本能力对齐”必选；当前已有私信（PM）与通知/实时通道（SSE `/realtime/stream` + NATS EventBus）满足核心实时需求。
- 引入 WebSocket/SSE 频道、成员、已读、持久化等会显著扩大阶段 4 范围，与“平台化以 Groups/TL/备份导入/扩展点为优先”的目标冲突。
- 按计划 Task 6 为可选：若不做，需在 index 标注 `won't do` 并留下 ADR。

## 后果
- 阶段 4 的 DoD 中 Chat 为 0 分项，不影响验收。
- 若未来需要，再以独立计划引入 `ChatChannel` / `ChatMessage`，复用现有 SSE 或评估 WebSocket，需通过新 ADR 审批后编码。

## 替代满足
- 私信（`messages/conversations`）覆盖 1:1 与小群私密会话。
- 通用实时流与 watch/提及通知覆盖轻量实时感知。
