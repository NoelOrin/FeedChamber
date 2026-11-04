## ADDED Requirements

### Requirement: 壳层插槽为固定枚举且未知插槽被忽略

前端公共壳层 MUST 提供固定的具名插槽集合，覆盖侧边栏、顶栏与页脚。
插槽名 MUST 来自枚举，前端 MUST 只渲染枚举内的插槽，未知插槽名 MUST 被忽略且 MUST NOT 导致渲染错误。

插槽枚举 MUST 至少包含：

- 侧边栏：`sidebar.top`、`sidebar.nav.after`、`sidebar.spaces.before`、`sidebar.spaces.after`、`sidebar.tags.after`、`sidebar.bottom`
- 顶栏：`header.left`、`header.right.before`、`header.right.after`
- 页脚：`footer.left`、`footer.right`

#### Scenario: 未知插槽不影响渲染

- WHEN 插槽数据中包含一个不在枚举内的插槽名
- THEN 前台 MUST 正常渲染其余插槽
- AND MUST NOT 抛出错误或渲染空白区域

#### Scenario: 插槽内容按排序渲染

- WHEN 同一插槽内存在多个启用条目
- THEN 前台 MUST 按 `sort` 升序渲染

### Requirement: 插槽只接受结构化条目，不执行裸 HTML

插槽条目类型 MUST 限于 `text` / `link` / `image` / `divider` / `widget` 五种。
系统 MUST NOT 接受裸 HTML 片段作为插槽内容，MUST NOT 在插槽内执行任何调用方提供的脚本。

条目 MUST 支持可见性（公开 / 仅登录 / 指定等级及以上），语义 MUST 与 `nav_items` 一致。

#### Scenario: 裸 HTML 被拒绝

- WHEN 写入请求的条目类型为 `html`
- THEN 系统 MUST 拒绝并返回参数校验类错误

#### Scenario: 等级可见性生效

- WHEN 某条目的可见性为「指定等级及以上」且阈值为 TL2
- THEN TL1 用户与游客 MUST NOT 看到该条目
- AND TL2 及以上用户 MUST 看到该条目

### Requirement: 渲染插槽时不得同步调用插件

插槽渲染 MUST 只读取已持久化的插槽数据，MUST NOT 在渲染路径上同步调用插件进程。

插件内容 MUST 由插件通过异步钩子调用宿主 API 写入插槽数据，写入后按既有渲染流程生效。

#### Scenario: 渲染路径无插件调用

- WHEN 前台渲染任意受支持插槽
- THEN 系统 MUST NOT 向任何插件进程发起同步请求

#### Scenario: 插件异步写入后生效

- WHEN 插件在 `cron.daily` 钩子中写入一条插槽条目
- THEN 该条目 MUST 在下一次前台加载时出现
- AND 写入过程 MUST NOT 阻塞任何用户请求

### Requirement: 插槽配置为空时回退既有布局

插槽数据为空时，前台 MUST 渲染既有的硬编码壳层布局，MUST NOT 呈现空白侧边栏、空白顶栏或空白页脚。

#### Scenario: 无配置时壳层保持原样

- WHEN 插槽表为空
- THEN 前台侧边栏、顶栏、页脚 MUST 与引入插槽前一致

### Requirement: 插槽写入必须留审计

任何对插槽条目的写入（创建、更新、删除、启停）MUST 记录审计日志，
至少包含操作者（管理员或插件标识）、目标条目、动作与变更摘要。

#### Scenario: 管理员修改条目留审计

- WHEN 管理员修改一个已启用的插槽条目
- THEN 系统 MUST 写入一条审计记录，含操作者与变更摘要

#### Scenario: 插件写入留审计

- WHEN 插件通过宿主 API 写入一条插槽条目
- THEN 该审计记录的操作者 MUST 标识为对应插件
