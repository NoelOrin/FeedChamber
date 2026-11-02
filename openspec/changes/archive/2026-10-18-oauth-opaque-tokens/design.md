# Design

## 权威设计

本 change 的设计已批准并完整记录在 [`docs/superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md`](../../../docs/superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md)（下称「设计文档」）。

本文件**不复制全文**，只摘录与本 change 的三处 spec delta 直接相关的决策，并补一节风险。实现细节、退场清单、验收口径一律以设计文档为准；两者冲突时以设计文档为准。

## 相关决策摘要

**令牌格式与校验顺序。** access token 的明文是 `of_at_` + `base64.RawURLEncoding(32B crypto/rand)`，只在 token 响应里返回一次，服务端只存 `hex(sha256(明文))`。校验路径保持既有顺序：**会话 sid 优先**（用户登录态），未命中再按 opaque token 查 `oauth_tokens`——`hash → 按 access_token 查行 → 过滤 revoked / expires_at → 用户令牌现查 user 行取 username/role（客户端令牌投影为 `oauth_client` 伪角色 + row.scope）`，写入 ctx 的键与旧实现完全一致，下游无感知。因此 `layered-architecture` 的凭证解析归属（只在 `internal/gateway`）不变，**本轮不需要新增分层规则**——这也是本轮只改三处 requirement 的原因。用户令牌不再携带 `role` 声明，降权/封号在下一次请求即生效，`operations` delta 里「不再有 24 小时陈旧的 role」由此而来。

**存储列复用。** `oauth_tokens.access_token` 现有 `NOT NULL + UNIQUE`，历史行存的是 JWT 的 JTI。选择复用列（Go 字段 `TokenHash` + 显式 `column:access_token`）而不是新增 `token_hash` 列：新增列会让 AutoMigrate 之外多出一次三库迁移，且旧行插入空串会撞 UNIQUE 索引；复用列则零迁移，且旧 JTI 与新 hash 永不匹配 → 旧令牌自然失效，不需要清理脚本。语义变化只写在模型注释里——这是本轮唯一「列名与内容不再同名」的取舍，评审时须知道它是有意为之。

**密钥持久化。** 唯一保留的 JWT 是 OIDC `id_token`（RS256，规范强制），因此 `golang-jwt` 依赖保留。签名私钥首次使用时按 2048 位生成、以 PKCS#1 PEM 存入 settings `sso_oidc_signing_key`，之后复用；`kid` 由公钥指纹推导（`rs256-<16hex>`），密钥轮换对 relying party 表现为新 `kid`。`SettingService` 只保证 `Get`，写入走可选的 `settingsWriter` 接口断言——测试传只读 fake 时密钥退化为进程内单例，这是刻意的降级而不是漏接线。

## 为什么只改这三处 spec

- `operations`：`JWT_SECRET` 从「必填」变成「不存在」，属于环境变量清单的事实变更；令牌 TTL 从环境变量/启动强校验迁移到 settings，是本轮唯一影响部署配置的项。
- `layered-architecture`：`pkg/jwt` 是 2026-10-14 那次变更写进 spec 的**唯一例外**，包既已删除，例外条款留着就会指向一个不存在的包。
- `backend-service-design`：protected 路由的鉴权描述在本轮之前已经失真（`JWTAuth`/`BanCheck` 于 2026-10-14 删除），本轮既然是「令牌语义收口」，把它一并改到 `gw.Enforce()` 现状，避免 delta 与代码继续背离。

其余 capability（`api-style`、`observability` 等）本轮**不产生 delta**：路由形状、响应信封、指标暴露面都不变；`/oauth/token` 与 `/sso/userinfo` 仍是 RFC 形状的协议端点，`api-style` 对它们的例外条款原文已覆盖。

## 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| **旧 JWT 失效窗口**：升级瞬间所有已签发 access token 全部作废，第三方集成方在重新换取令牌前会连续收到 `401 {code:1002}` | 这是有意的破坏性变更，已在 proposal 与设计文档「破坏性变更」中明示；`1002` 是既有的「无效或过期」码，客户端无需改错误处理逻辑。**生产发布前须人工通知集成方**（见 tasks「仍需人工确认」），本 change 不提供兼容期或双读旧 JTI 的过渡开关——兼容旧令牌等于把 JWT 保留下来，与本轮目标矛盾 |
| **id_token 密钥首次生成写入 settings**：首个 OIDC 请求会在请求路径上生成 2048 位 RSA 私钥并 `Set` 一次，可能表现为首个请求延迟；多副本同时首启会各生成一份，后写覆盖先写 | 生成发生在 `signingKey()` 的互斥区内，单副本只做一次；写失败不影响进程内签名（密钥仍在内存），下次调用会重试。多副本同时首启的覆盖窗口很短（毫秒级），且被覆盖的副本在下次冷启动后会读到最终值——接受该窗口，不做分布式锁（为一个一次性写入引协调成本不划算）。可运维的方式是提前在 settings 里放好 PEM |
| **`sso_oidc_signing_key` 读不出来的情况**（PEM 损坏、手工改坏） | 按设计重新生成并覆盖，`kid` 随之变化：旧 id_token 失效，但 relying party 会通过 JWKS 拿到新公钥，OIDC 流程自愈。MUST NOT 静默留用半损坏的密钥 |
| **`kid` 由公钥推导后不再固定为 `feedchamber-rs256-1`** | 依赖方 MUST 通过 JWKS 的 `kid` 匹配，硬编码旧 kid 的集成方会验签失败。仓库内无硬件编码 kid 的消费者（grep 实测），属外部集成注意事项 |
| **撤销语义从「双写」收敛为「翻列」后，KV 里残留的 `bl:` 标记无人清理** | 标记不再被任何代码读取（`clientTokenRevoked` 已删），残留是惰性数据；KV per-key TTL 会让其在窗口后自然消失，无需迁移 |
| **旧 settings key 留在表中**（`jwt_access_token_ttl` / `jwt_refresh_token_ttl`） | 不再被读取即无害，管理员可自行删除；不写清理迁移（见 Non-goals） |
