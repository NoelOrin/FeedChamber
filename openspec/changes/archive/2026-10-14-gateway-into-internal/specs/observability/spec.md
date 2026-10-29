## MODIFIED Requirements

### Requirement: 关联标识与请求追踪

网关 MUST 为每个进入的 HTTP 请求确定 trace 上下文：优先接受上游 `traceparent` 头（W3C Trace Context Level-1，仅 `00` 版本），其 trace id 合法时沿用并新建 span id；缺失或不合法时 MUST 生成 128-bit 随机 trace id 与 64-bit span id。trace 上下文 MUST 写入 `request.Context()`，响应 MUST 回写 `traceparent` 与 `X-Request-ID`（后者取值等于 trace id，头名保持不变以兼容既有客户端）。

`internal/gateway/tracing.go` 的 `Tracing` 早先是空 TODO 实现却每请求调用，现 MUST 只做一件事：路由匹配后把端点模板写进 `trace.Name`（即 span 命名）。trace 本体 MUST 由 app 级 `internal/middleware.Trace` 建立——它必须先于 `Logger`，否则请求日志无 trace 可取。`Tracing` 在流水线里的首位调用位置 MUST 保持（由 `Gateway.preflight` 与组合入口 `Handler` 调用；跨包位置引用一律指符号，不指行号）。

#### Scenario: 无入站 traceparent

- WHEN 请求不携带 `traceparent` 头
- THEN 网关 MUST 生成新 trace id 与 span id，并在响应头回写 `traceparent: 00-<trace-id>-<span-id>-01`

#### Scenario: 沿用上游 trace

- WHEN 请求携带合法的 `traceparent`（版本 `00`、32 位十六进制 trace id、16 位十六进制 span id、非全零）
- THEN trace id MUST 沿用入站值、span id MUST 新生成，且入站 span id 记为该 span 的 parent

#### Scenario: 拒绝畸形 traceparent

- WHEN `traceparent` 存在但格式不合法（版本非 `00`、长度错误、含非十六进制字符、trace id 全零）
- THEN 系统 MUST 忽略该头并生成全新 trace，且 MUST NOT panic 或因此拒绝请求

#### Scenario: 追踪开关

- WHEN `TRACING_ENABLED=false`
- THEN 系统 MUST 仍产生用于日志关联的 trace id 并写入 context，仅停止解析入站 `traceparent` 与回写响应头
