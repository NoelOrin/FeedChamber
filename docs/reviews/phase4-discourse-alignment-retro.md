# 阶段 4 复盘：与 Discourse 对齐度

**日期：** 2026-07-19  
**分支：** `codex/discourse-phase4-platformization`

## 交付摘要

| 能力 | 状态 | 说明 |
|------|------|------|
| Groups 成员与权限聚合 | 已完成 | system groups + group_permissions；`HasPermissionForUser`；Dashboard 组管理 |
| 信任等级 TL0–TL2 | 已完成 | 字段 + 每日晋升 ticker；TL0 外链/进审门禁；admin 手动调级 API |
| 站点备份导出 | 已完成 | CLI `feedchamber backup --out`（sqlite 快照 + uploads 清单 + meta） |
| Discourse JSON 导入 MVP | 已完成 | `import-discourse --dir`；`import_maps` 幂等；文档 `../design/import-discourse.md` |
| 扩展点 / 事件目录 | 已完成 | `eventbus/events.go` + `../design/extension-points.md` |
| Chat | won't do | ADR：`docs/adr/chat-mvp.md` |

## 与 Discourse 对齐度（主观）

- **治理/权限：** 从单一 Role 走向 Role + Groups 双读，接近 Discourse Groups 的基础形态；版面覆盖权限仍可并存。
- **信任与反垃圾：** TL0 门禁覆盖“新用户外链/进审”高频场景，未做完整 TL3/TL4 与阅读时长统计。
- **运营迁移：** 备份/导入达到“可迁站 MVP”，非全量 Discourse parity（附件、权限组、标签、私信未导入）。
- **扩展模型：** 明确官方模块 + EventBus，拒绝动态插件运行时——与 Discourse 插件生态不同，更偏可维护 monorepo。
- **实时协作：** 无私信之外的 Chat 频道；实时能力仍以通知 SSE 为主。

**总体：** 阶段 4 把 Feed Chamber 从“运营留存论坛”推进到“可迁站、可分组授权、可信任分级”的平台化底座；距离完整 Discourse 仍差插件市场、完整 TL、高级审核工作流与 Chat。

## 风险与后续

1. Groups 与 Role 双轨需持续同步 system group 成员，避免漂移。
2. TL 晋升指标可配置化（settings）与拒绝率统计可后续加强。
3. 导入器扩展：附件、标签、权限组、增量同步。
4. 备份对非 sqlite 需接入引擎原生 dump。
