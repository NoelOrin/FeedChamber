---
title: 可见性与门控
description: 版面可见性、等级门槛、门控规则引擎
sidebar:
  order: 3
---

## 可见性（Visibility）

版面有 `visibility ∈ public | login | level`，配合 `min_level`；帖子可用 `min_level` 单帖覆盖。
有效策略**沿祖先链取最严格一档**（level > login > public，level 取链上最大值）。

六个内容出口共用同一谓词：列表 / 详情 / 搜索 / sitemap / RSS / 批量查询。

| 场景 | 行为 |
|------|------|
| 聚合出口（列表、搜索、sitemap、RSS） | SQL 层过滤，不可见的直接不出现 |
| 详情 | 返回 403，并提示「需要等级 N，当前等级 M」 |
| 作者本人 | 例外，能看到自己的内容 |
| 有 `report:manage` 的审核者 | 不受限（否则无法治理受限内容） |
| 游客 | 一律按 TL0 |

评论列表与评论创建也走同一门槛：帖子不可见时同样 404/403，**不能盲写看不见的主题的评论**。

## 门控规则引擎（Gate）

比可见性更细的一层：按谓词 + 优先级决定一次动作该被允许、拒绝、转人工还是限流。

```text
gate_rules(priority, domain, conditions JSON, action)
gate_domain_defaults(domain, action)
```

- **谓词集固定**：等级 / XP / 版面 / 标签 / 作者等级 / 是否作者 / 游客 / 邮箱验证 / 封禁 / 暗区 / 外链 / 窗口指标
- **动作只有四个**：`allow` / `deny` / `require_review` / `rate_limit`
- **求值**：按 `priority` 降序，**首个命中**即生效；无命中走该域的默认动作
- **生效范围**：发帖在 `post_create` 域、评论在 `comment_create` 域；
  正文含外链时额外进 `external_link` 域（只在含链接时求值，避免误伤纯文本）

配置有内存快照（30s）并在写入时通过 Pub/Sub 广播失效，所以改规则立即生效，不必重启。
后台 `/admin/gates` 支持 CRUD、域默认与「以某个用户试算」——试算与线上共用同一 `Evaluate`。

### 典型配置

| 规则 | 域 | 条件 | 动作 | 优先级 |
|------|----|------|------|--------|
| TL0 不许发外链 | `external_link` | 等级 = 0 | `deny` | 100 |
| 等级 ≥1 放行外链 | `external_link` | 等级 ≥ 1 | `allow` | 50 |

## 与审核的关系

`require_review` 会把帖子落成 `pending` 并留痕（`strategy=gate`），
进人工审核队列。这与「审核模式」是两套独立机制：

- **审核模式**（`moderation_mode`）决定**所有**帖子的初始走向
- **门控规则**决定**特定条件下**的额外干预

两者同时命中时，取更严格的结果。
