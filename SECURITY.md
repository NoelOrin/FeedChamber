# 安全策略

## 报告漏洞

**不要开公开 issue。** 请通过 GitHub 的私有漏洞报告渠道提交：

<https://github.com/NoelOrin/FeedChamber/security/advisories/new>

报告里请包含：

- 受影响的版本或提交（`git rev-parse --short HEAD`）
- 复现步骤与最小请求（curl 优先）
- 影响面：未授权读取、越权写入、提权、信息泄露、拒绝服务
- 如果方便，给出修复建议

## 响应时间

| 阶段 | 目标 |
|------|------|
| 确认收到 | 72 小时内 |
| 初判与影响分级 | 7 天内 |
| 修复或缓解方案 | 视严重程度，通常 30 天内 |

修复发布后会在 release notes 里致谢（除非你要求匿名）。

## 支持范围

只有 `main` 分支的最新提交在支持范围内。自建部署遇到问题请附上：

- 部署方式（Compose / 自建）、版本或提交
- 相关日志（保留 `trace_id`，删除密钥与个人信息）
- 是否启用了 API Key / MCP / OAuth / 外部审核

## 已知的部署前提

以下不是漏洞，而是设计前提，报告前请先确认：

| 前提 | 说明 |
|------|------|
| NATS 与 Postgres 只在内网 | 两者都不带面向公网的鉴权层，必须靠网络隔离 |
| `MASTER_COOKIE_SECURE=true` | 生产必须经 HTTPS，否则会话 cookie 可被截获 |
| 管理员密码已改 | seed 密码只用于首次登录 |
| 反代限制请求体 | 不设置 `client_max_body_size` 时上传会被 nginx 默认值拒绝 |
| 机器凭证受 scope 限制 | API Key 权限是「scope ∩ 角色权限」，不是超集 |

## 安全相关配置

- 会话：登出 / 改密 / 改角色 / 封号都会吊销该用户全部会话
- MFA：`mfa_totp_enabled`，支持 TOTP 与备份码
- 暴力破解：`security_brute_force_max_attempts` / `security_brute_force_window`
- 限流：`rate_limit_enabled`（默认关闭）与各端点预算
- 高风险内容：机器凭证写 JS/HTML 会被强制存草稿
- 审计：管理操作与 MCP 调用都留痕

完整清单见[生产部署清单](https://noelorin.github.io/FeedChamber/install/production/)与
[认证与安全](https://noelorin.github.io/FeedChamber/configuration/authentication/)。
