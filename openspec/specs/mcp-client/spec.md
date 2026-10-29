# mcp-client Specification

## Purpose
定义把外部 MCP Server 作为站点能力源的接入方式：登记、连通性验证，以及在内容审核链路中优先使用 MCP、失败时回退既有契约的行为约束。
## Requirements
### Requirement: 外部 MCP Server 可登记管理

系统 MUST 支持登记外部 MCP Server，每条登记 MUST 包含名称、端点地址、传输方式与鉴权配置，并 MUST 支持启用与停用。鉴权凭据 MUST 在接口响应中脱敏，MUST NOT 以明文返回。

停用的外部服务 MUST NOT 被调用。

#### Scenario: 登记后可用于调用

- WHEN 管理员登记一个外部 MCP Server 并启用
- THEN 系统 MUST 能对其发起 MCP 调用

#### Scenario: 凭据不以明文返回

- WHEN 管理员列出已登记的外部 MCP Server
- THEN 响应 MUST NOT 包含明文鉴权凭据

#### Scenario: 停用后不再调用

- WHEN 一个外部 MCP Server 被停用
- THEN 系统 MUST NOT 再向其发起调用

### Requirement: 可测试外部服务连通性

系统 MUST 提供连通性测试能力：对指定外部 MCP Server 执行初始化并列出其工具，把可用工具清单与失败原因返回给调用方。测试失败 MUST NOT 影响已保存的配置。

#### Scenario: 测试成功返回工具清单

- WHEN 管理员对可达的外部 MCP Server 执行连通性测试
- THEN 系统 MUST 返回该服务暴露的工具清单

#### Scenario: 测试失败给出原因

- WHEN 外部 MCP Server 不可达或响应非法
- THEN 系统 MUST 返回明确的失败原因，且 MUST NOT 修改已保存的登记信息

### Requirement: 外部审核优先走 MCP 并保留回退

当站点审核模式为外部自动化审核、且已启用一个承担审核职责的外部 MCP Server 时，系统 MUST 通过 MCP `tools/call` 获取审核结论。未配置或调用失败时，MUST 回退到既有的外部 HTTP 审核契约，以保持向后兼容。

外部审核不可用时，系统 MUST NOT 丢弃或静默放行待审内容，MUST 按既有回退策略处理并留痕。

#### Scenario: 配置了 MCP 审核服务时走 MCP

- WHEN 审核模式为外部自动化且启用了 MCP 审核服务
- THEN 系统 MUST 通过 MCP 调用取得审核结论

#### Scenario: 未配置时回退既有契约

- WHEN 审核模式为外部自动化但未配置 MCP 审核服务
- THEN 系统 MUST 使用既有的外部 HTTP 审核契约

#### Scenario: MCP 调用失败时回退

- WHEN MCP 审核服务调用超时或返回非法结论
- THEN 系统 MUST 回退到既有策略处理，MUST NOT 静默放行

### Requirement: 外部调用必须设超时与失败隔离

对外部 MCP Server 的调用 MUST 设置超时。超时或异常 MUST NOT 阻塞内容发布主链路，MUST NOT 令主链路长时间挂起。

单次外部调用失败 MUST NOT 影响后续调用，MUST NOT 使站点进入不可用状态。

#### Scenario: 超时不会挂起发布

- WHEN 外部 MCP Server 响应超时
- THEN 内容发布链路 MUST 在限定时间内继续，并按规定回退

