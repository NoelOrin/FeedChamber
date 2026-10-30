---
title: 作为 MCP Client
description: 登记外部 MCP 服务，把审核等能力委托出去
sidebar:
  order: 3
---

本站可以作为 MCP 客户端调用外部服务。目前的生产用途是**内容审核委托**：
把待审内容发给外部 AI/审核服务，由它返回结论。

## 登记一个外部服务

后台 `/admin/mcp-servers`：

| 字段 | 说明 |
|------|------|
| 名称 | 仅用于展示 |
| Endpoint | 外部服务的 MCP 地址（`https://…/mcp`） |
| 传输 | 目前只支持 `streamable_http` |
| 鉴权头 | JSON 对象，如 `{"X-API-Key":"…"}`；只返回值键名回显，不返回凭据 |
| 用途 | `general` 或 `moderation` |
| 审核工具名 | 用途为 `moderation` 时指定，留空走默认工具名 |

保存前会校验 endpoint（拒私网/回环）与鉴权头格式（必须是字符串到字符串的对象）。

## 审核链路怎么用它

```text
发帖（moderation_mode = external）
  → 查询「用途=moderation 且已启用」的服务
    → 有：调 tools/call 取结论
    → 无 / 调用失败：回退既有 HTTP 审核契约，再失败则回退 async
```

:::note[失败不会静默放行]
外部服务调用失败会打警告并回退，而不是把内容直接判为通过。
但如果**从未配置**外部审核，则静默回退到既有契约（这是正常路径，不是错误）。
:::

## 安全约束（写在这里是因为它容易被忽略）

- **不跟随重定向**：Go 只跨域剥离 `Authorization` / `Cookie`，自定义鉴权头会原样
  带到重定向目标——所以直接返回 `http.ErrUseLastResponse`
- **SSRF 防护**：endpoint 拒绝回环 / 私网 / 链路本地（含 `169.254.169.254`）
- **凭据不回显**：列表接口只返回 header 键名，模型层对该字段打 `json:"-"`
- **响应上限 1MB**：超限报「响应过大」，而不是把截断后的内容当成非法 JSON-RPC
- **session 复用**：回传 `Mcp-Session-Id`，有状态实现在第二个请求上才不会拒绝

## 连通性测试

后台点「测试」会跑 `initialize` + `tools/list` 并返回工具清单或失败原因。
**测试失败不会修改已保存的配置。**
