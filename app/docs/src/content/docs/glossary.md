---
title: 术语表
description: 读这套文档会遇到的专有名词，一句话一个
---

| 术语 | 含义 |
|------|------|
| **版面（Space）** | 帖子的归属容器，树形结构，可设可见性、图标、排序与启禁状态 |
| **可见性（Visibility）** | 读权限档位：`public`（游客可读）/ `login`（登录可读）/ `level`（达到等级可读），沿祖先链取最严 |
| **门控（Gate）** | 按条件自动决定 `allow` / `deny` / `require_review` / `rate_limit` 的规则引擎 |
| **门控域（Domain）** | 规则作用的能力域，如发帖、评论、外链；无规则命中时走该域的默认动作 |
| **TL0** | 等级 0，新注册用户与游客的起点等级（历史上叫 Trust Level 0） |
| **XP** | 由行为聚合出来的经验值：`Σ(指标 × 规则) + 连续签到 + 人工调整`，删帖会回收 |
| **保级** | 等级按滚动窗口判定，长期不活跃会降级；可升可降，逐级下探 |
| **人工调整（XP adjustment）** | 管理员加/减 XP 并附原因的记录，参与聚合，不会被重算冲掉 |
| **审核模式** | `off` / `async`（默认）/ `external` / `manual` 四档，决定新内容的初始状态 |
| **分级生效** | 机器凭证（API Key / MCP）写入 JS/HTML 等高风险内容时强制存草稿，需人工启用 |
| **轮换（Rotate）** | `/auth/refresh` 签发新 sid 并立即作废旧 sid，单次有效 |
| **服务端会话** | 凭证是 `user_sessions` 表里的一行；"是否有效"等价于"行是否存在" |
| **read-through 缓存** | 先查 KV，未命中回源数据库并回填 |
| **信封（Envelope）** | 统一响应形状 `{ code, msg, data }` |
| **协议端点** | 不套信封的端点（OAuth / OIDC / MCP），必须保持 RFC 形状 |
| **scope** | API Key 上的权限范围，是**凭证上限**；实际权限还要与角色权限取交集 |
| **Deps** | 显式依赖清单；handler 消费 `handler.Deps`，装配在 `server/wiring.go` |
| **wiring** | 把 container 里构造好的服务填进 handler 依赖的那一层 |
| **事件总线（EventBus）** | NATS core pub/sub；业务 service 发布事件，通知服务订阅消费 |
| **JetStream KV** | NATS 的持久化 KV，承担会话缓存、限流窗口、设置缓存、OAuth state |
| **SSE** | Server-Sent Events，通知与通用实时推送的传输方式；空闲时每 15s 发心跳注释 |
| **MCP** | Model Context Protocol；本站既可作 Server 对外暴露工具，也可作 Client 调外部服务 |
| **动议（Motion）** | 社区投票类议题，支持创建、投票、结论 |
| **主题订阅（Watch）** | 关注某个帖子，有新回复/点赞时收到通知 |
| **回收站（Trash）** | 软删除内容的暂存区，可恢复；不是物理删除 |
| **修订（Revision）** | 帖子与评论的历史版本，可查看与恢复 |
| **审计日志（Audit Log）** | 管理操作与 MCP 调用的留痕，记录操作者、动作、目标与时间 |
| **备份（Backup）** | 后台或 CLI 生成的 DB + 上传清单；NATS 数据可重建，不算业务备份 |
