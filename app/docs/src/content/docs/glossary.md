---
title: 术语表
description: 读这套文档会遇到的专有名词
sidebar:
  order: 4
---

| 术语 | 含义 |
|------|------|
| **版面（Space）** | 帖子的归属容器，树形结构。取代了早期的 Category |
| **门控（Gate）** | 按条件自动决定「允许 / 拒绝 / 转人工 / 限流」的规则引擎 |
| **可见性（Visibility）** | 版面/内容的读权限档位：public / login / level |
| **TL0** | 等级 0，游客与未成长的用户。历史上叫 Trust Level 0 |
| **XP** | 行为聚合出来的经验值，不是累加计数器 |
| **保级** | 等级按滚动窗口判定，长期不活跃会降级 |
| **暗区** | 不对普通用户公开的版面（`visibility=level` 且门槛较高） |
| **分级生效** | 机器凭证写入高风险内容时强制存草稿，需人工启用 |
| **轮换（Rotate）** | `/auth/refresh` 签发新 sid 并立即作废旧 sid |
| **read-through 缓存** | 先查 KV，未命中回源数据库并回填 |
| **信封（Envelope）** | 统一响应形状 `{code, msg, data}` |
| **协议端点** | 不套信封的端点（OAuth/OIDC/MCP），必须保持 RFC 形状 |
| **scope** | API Key 上的权限范围，是**凭证上限** |
| **Deps** | 显式依赖清单。handler 消费 `handler.Deps`，装配在 `server/wiring.go` |
| **wiring** | 把 container 里构造好的服务填进 handler 依赖的那一层 |
| **gofmt 漂移** | 代码未按 gofmt 格式化，`lint` 会拦 |
