# mcp-server Specification

## Purpose
定义 Feed Chamber 对外暴露的 MCP Server：协议形状、凭证与权限模型、可调用的工具目录、以及工具级开关与审计约束，使外部 AI 客户端能以最小权限、可审计的方式读写站点内容并驱动站点自定义。
## Requirements
### Requirement: MCP 端点与协议形状

系统 MUST 在应用根路径 `/mcp` 暴露一个 MCP Server，MUST NOT 挂在 `/api/v1` 下。该端点 MUST 实现 MCP 的 `initialize`、`tools/list`、`tools/call` 三个方法，请求与响应 MUST 遵循 JSON-RPC 2.0。

传输 MUST 为 Streamable HTTP。该端点 MUST NOT 套用论坛的统一响应信封（`{code,msg,data}`），因为 MCP 客户端按 JSON-RPC 解析。

#### Scenario: 客户端完成初始化握手

- WHEN 一个 MCP 客户端向 `/mcp` 发送 `initialize` 请求
- THEN 系统 MUST 返回包含协议版本与服务能力的 JSON-RPC 响应

#### Scenario: 列出可用工具

- WHEN 客户端发送 `tools/list`
- THEN 系统 MUST 返回当前对该凭证可用的工具清单，每个工具含名称、描述与输入参数模式

#### Scenario: 调用工具返回结构化结果

- WHEN 客户端发送合法的 `tools/call`
- THEN 系统 MUST 返回 JSON-RPC 结果，业务失败 MUST 以工具级错误表达而非 HTTP 层错误

### Requirement: MCP 凭证沿用 API Key

MCP 请求 MUST 以既有 API Key 凭证鉴权，支持 `X-API-Key` 头与 `Authorization: Bearer <key>` 两种携带方式。API Key 功能未启用时，`/mcp` MUST 拒绝所有请求。

凭证无效、已过期、已吊销或作用域不足时，系统 MUST 返回鉴权类错误，MUST NOT 执行任何工具副作用。

#### Scenario: 有效 Key 可调用工具

- WHEN 客户端携带有效且具备相应作用域的 API Key 调用工具
- THEN 系统 MUST 执行该工具并返回结果

#### Scenario: API Key 功能关闭时拒绝

- WHEN 站点的 API Key 功能处于关闭状态
- THEN 任何 `/mcp` 请求 MUST 被拒绝

#### Scenario: 吊销的 Key 被拒绝

- WHEN 客户端携带一个已吊销的 API Key 调用工具
- THEN 系统 MUST 返回鉴权错误且 MUST NOT 产生副作用

### Requirement: MCP 端点自行完成网关等价检查

由于 `/mcp` 位于应用根而非网关保护面内，MCP 层 MUST 自行完成与受保护 HTTP 路由等价的检查：维护模式（非管理员写入被拒）、账号封禁与禁言拦截、权限校验、限流。

MUST NOT 依赖调用方自行遵守这些约束，MUST NOT 因省略检查而使被封禁用户的凭证通过 MCP 写入数据。

#### Scenario: 维护模式阻止 MCP 写入

- WHEN 站点处于维护模式且凭证属于非管理员
- THEN 写类工具 MUST 被拒绝

#### Scenario: 被封禁账号的 Key 不能写入

- WHEN 一个已被封禁（或禁言）用户的 API Key 调用写类工具
- THEN 系统 MUST 拒绝该调用并返回相应错误

#### Scenario: 超出限流预算被拒绝

- WHEN 某凭证在窗口内超出其限流预算
- THEN 后续调用 MUST 被拒绝并返回限流类错误

### Requirement: 自定义能力使用独立权限域

自定义配置的读写 MUST 使用独立权限域，MUST 至少区分 `customize:read`、`customize:write`、`customize:publish` 三项。这些权限 MUST NOT 被并入既有的通用管理作用域或通用写作用域。

MUST NOT 因新增这些权限而扩大任何既有 API Key 的权限集；新权限 MUST 在创建凭证时显式授予。

#### Scenario: 既有管理作用域不自动获得自定义权限

- WHEN 存在一个仅持有既有通用管理作用域的 API Key
- THEN 该 Key MUST NOT 能写入自定义配置

#### Scenario: 未显式授予读取权限时不可读

- WHEN 一个 API Key 未被授予自定义读取权限
- THEN 调用自定义层读工具 MUST 被拒绝

#### Scenario: 发布权限独立于写入权限

- WHEN 一个 API Key 仅持写入权限而未持发布权限
- THEN 其写入 JS 类内容 MUST 以草稿状态保存

#### Scenario: 发布权限不得由角色继承

- WHEN 凭证所属用户的角色本身持有「发布」权限，但该 API Key 的 scope 未显式包含它
- THEN 该凭证 MUST NOT 被视为持有发布权限
- AND 其写入 JS 类内容 MUST 仍以草稿状态保存
- AND 发布权限 MUST 仅通过 API Key 的显式 scope 授予

### Requirement: 工具目录覆盖读、写与自定义

MCP 工具 MUST 至少覆盖三类能力：内容读取（搜索帖子、读取帖子、列出版面、列出标签、读取用户资料）、内容写入（发帖、评论、点赞、收藏）、以及站点自定义（列出自定义片段、写入片段、设置主题 token、写入导航项、写入首页模块、写入自定义页）。

内容写入工具 MUST 复用站点既有的审核策略决定内容是否待审，MUST NOT 为 MCP 单独引入一套审核旁路。

工具 MUST NOT 包含创建或提权 API Key 的能力，以避免凭证自我扩权。

#### Scenario: 读工具返回站内内容

- WHEN 客户端调用内容搜索工具
- THEN 系统 MUST 返回与站内搜索一致的结果集，并遵循既有的可见性门槛

#### Scenario: 写工具遵循站点审核策略

- WHEN 客户端调用发帖工具且站点审核模式要求审核
- THEN 该帖子 MUST 进入与网页发帖相同的审核流程

#### Scenario: 不存在提权工具

- WHEN 客户端请求工具清单
- THEN 返回的清单 MUST NOT 包含创建 API Key 或修改自身作用域的工具

### Requirement: 工具可逐个启停且默认关闭写工具

系统 MUST 支持逐个启用或停用 MCP 工具。内容写入与自定义写入类工具 MUST 默认处于停用状态，需站长显式开启后才可被调用。

被停用的工具 MUST NOT 出现在 `tools/list` 中，或 MUST 在被调用时返回明确的「工具未启用」错误。

#### Scenario: 默认不可写

- WHEN 站长未开启任何写类工具
- THEN 客户端调用发帖工具 MUST 被拒绝

#### Scenario: 停用工具不出现在清单

- WHEN 某个工具被停用
- THEN `tools/list` MUST NOT 返回该工具

### Requirement: MCP 调用必须留审计

每次 `tools/call` MUST 记录审计日志，包含调用者（凭证所属用户）、工具名、调用结果，以及本次调用是否产生或修改了站点配置。对自定义层的写入 MUST 额外记录生效状态（已生效或草稿）。

#### Scenario: 工具调用可追溯

- WHEN 客户端成功调用任一工具
- THEN 系统 MUST 写入一条包含调用者、工具名与结果的审计记录

#### Scenario: 自定义写入记录生效状态

- WHEN 客户端通过工具写入自定义配置
- THEN 审计记录 MUST 标明该次写入是已生效还是已存为草稿

