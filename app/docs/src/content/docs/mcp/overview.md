---
title: MCP 概览
description: 本站既是 MCP Server，也是 MCP Client
sidebar:
  order: 1
---

MCP（Model Context Protocol）让 AI 客户端用统一协议调用工具。Feed Chamber 两个方向都支持：

| 方向 | 含义 | 入口 |
|------|------|------|
| **Server** | 本站对外暴露工具，AI 客户端读写本站内容与站点配置 | `POST /mcp` |
| **Client** | 本站调用外部 MCP 服务（如把内容审核委托出去） | `/admin/mcp-servers` |

## Server 方向

端点：`POST /mcp`（挂在应用根，**不在** `/api/v1` 之下）

鉴权：API Key（`X-API-Key` 或 `Authorization: Bearer ak_…`）

响应是 **JSON-RPC 2.0**，不套论坛信封：

```json
{ "jsonrpc": "2.0", "id": 1, "result": { "tools": [] } }
```

业务失败用**工具级** `isError: true` 表达，JSON-RPC 层的 `error` 只留给协议问题——
客户端据此区分「工具执行失败」与「通信失败」。

:::caution[它不在网关覆盖范围内]
`/mcp` 拿不到 `gw.Preflight()` / `gw.Enforce()`，所以它自带四项检查：
凭证校验、封禁/禁言、权限码、限流。关闭 `api_key_enabled` 会同时关掉 `/mcp`。
:::

启用步骤：

1. 后台 `/admin/settings?tab=site` 把 `mcp_enabled` 设为 `true`
2. 确认 `api_key_enabled` 也是 `true`
3. 后台 `/admin/api-keys` 建一把 Key，按需授予 scope
4. 后台 `/admin/mcp` 开启要用的写工具（写工具默认关闭）

## Client 方向

在 `/admin/mcp-servers` 登记外部服务（endpoint + 鉴权头 + 用途）。
用途选「内容审核」时，审核链路会优先通过它取结论；不可用则回退既有 HTTP 契约。

安全约束：
- endpoint 只允许 http(s)，且拒绝回环 / 私网 / 链路本地地址（SSRF 防护）
- **不跟随重定向**：自定义鉴权头不会泄露给重定向目标
- 鉴权头存库为明文但**从不回显**，接口只返回键名
- 每次调用有超时，响应超过 1MB 直接报错

## 下一步

- [Server 配置与工具目录](/mcp/server/)
- [Client 登记与审核委托](/mcp/client/)
