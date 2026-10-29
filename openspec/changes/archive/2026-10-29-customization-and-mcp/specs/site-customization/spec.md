## Purpose

定义站点自定义层（外观与信息架构）的数据模型、生效语义与安全约束，使站长无需改代码即可调整主题、导航、首页模块与静态页，并为 AI 通过 MCP 驱动这些配置提供统一的行为契约。

## ADDED Requirements

### Requirement: 自定义内容以「片段」为单位管理

自定义的 CSS / JS / HTML MUST 以独立片段（snippet）为单位存储，而非单一全局字符串。每个片段 MUST 具备：唯一标识、类型（`css` / `js` / `html`）、注入位置（`head` / `body_end`）、启用状态、排序权重、最后修改者与修改时间。

片段 MUST 可独立启用与停用，停用某个片段 MUST NOT 影响其他片段。

#### Scenario: 多段 JS 并存且可单独停用

- WHEN 站点存在两个已启用的 `js` 片段
- THEN 前台 MUST 按排序权重依次注入两者
- AND 停用其中一个后，前台 MUST 只注入另一个，另一段内容 MUST 保持不变

#### Scenario: 片段按注入位置输出

- WHEN 一个 CSS 片段的注入位置为 `head`，一个 JS 片段的注入位置为 `body_end`
- THEN CSS 片段 MUST 出现在文档 `<head>` 内，JS 片段 MUST 出现在文档体末尾

### Requirement: 主题 token 覆盖无需编写 CSS

系统 MUST 支持按 token 名覆盖主题视觉变量（如主色、圆角、字体），语义等价于覆盖 `packages/tokens` 产出的 CSS 变量。覆盖 MUST 以「token 名 → 值」的结构化形式存储，MUST NOT 要求站长编写选择器或 CSS 规则。

未覆盖的 token MUST 保持默认值；覆盖 MUST 同时作用于明亮与暗色两种模式，或明确指定仅作用于其中一种。

#### Scenario: 覆盖主色即时生效

- WHEN 站长将主色 token 设为某个色值
- THEN 前台的主色相关视觉 MUST 更新为该色值
- AND 未覆盖的其他 token MUST 保持默认值

#### Scenario: 清除覆盖回退默认

- WHEN 站长删除某条 token 覆盖
- THEN 该 token MUST 回退到默认值

### Requirement: 导航与首页模块由配置驱动

站点的导航项与首页模块 MUST 由配置数据驱动渲染，而非硬编码在组件中。导航项 MUST 支持标签、目标地址、图标、排序与可见性（公开 / 仅登录 / 指定等级及以上）。首页模块 MUST 支持类型、排序与可见性。

当配置为空时，系统 MUST 回退到既有硬编码的默认导航与首页布局，MUST NOT 呈现空白导航或空白首页。

#### Scenario: 自定义导航项出现在前台

- WHEN 站长新增一个可见性为公开的导航项
- THEN 前台导航 MUST 显示该项，并按排序权重排列

#### Scenario: 可见性受限的导航项对游客隐藏

- WHEN 某导航项的可见性为「仅登录」
- THEN 未登录访问者 MUST NOT 看到该项
- AND 已登录用户 MUST 看到该项

#### Scenario: 无配置时回退默认

- WHEN 导航配置为空
- THEN 前台 MUST 渲染既有的默认导航项，MUST NOT 渲染空导航

### Requirement: 自定义页面可独立发布

系统 MUST 支持创建以固定前缀（`/p/{slug}`）访问的自定义静态页，正文为 Markdown。页面 MUST 具备草稿与已发布两种状态，未发布页面 MUST NOT 对站点访客可访问。slug MUST 唯一；与既有路由冲突的 slug MUST 被拒绝。

#### Scenario: 已发布页面可公开访问

- WHEN 一个自定义页被发布
- THEN 访问 `/p/{slug}` MUST 返回该页内容

#### Scenario: 草稿页面不可访问

- WHEN 一个自定义页处于草稿状态
- THEN 访客访问 `/p/{slug}` MUST 得到未找到的结果

#### Scenario: slug 冲突被拒绝

- WHEN 创建一个 slug 与既有页面重复的自定义页
- THEN 系统 MUST 拒绝该请求并返回参数校验类错误

### Requirement: 自定义写入必须留审计

任何对自定义配置的写入（创建、更新、删除、启停）MUST 记录审计日志，包含操作者、动作、目标与变更摘要。审计记录 MUST 足以回答「谁在什么时候改了哪一段、改前改后是什么」。

#### Scenario: 修改片段留下审计记录

- WHEN 管理员修改一个已启用的 JS 片段
- THEN 系统 MUST 写入一条审计记录，至少包含操作者、目标片段标识与变更摘要

### Requirement: AI 写入自定义配置必须分级生效

通过机器凭证（MCP / API Key）写入自定义配置时，生效策略 MUST 按风险分级：

- **CSS 片段与主题 token**：持写入权限即可直接生效。
- **JS / HTML 片段、导航项、首页模块、自定义页**：MUST 以草稿（未启用 / 未发布）状态写入，需人工在后台启用后才生效；仅当凭证额外持有「发布」权限时才允许直接生效。

系统 MUST 在写入响应中明确告知该次写入是「已生效」还是「已存为草稿」，MUST NOT 让调用方无法区分两者。

#### Scenario: AI 写 JS 默认落草稿

- WHEN 一个仅持自定义写入权限的机器凭证提交一段 JS 片段
- THEN 该片段 MUST 以未启用状态保存
- AND 前台 MUST NOT 注入该片段，直到管理员在后台启用

#### Scenario: AI 写 CSS 直接生效

- WHEN 一个持自定义写入权限的机器凭证提交一段 CSS 片段
- THEN 该片段 MUST 直接生效，无需人工启用

#### Scenario: 持发布权限的凭证可直接生效

- WHEN 一个同时持有「发布」权限的机器凭证提交一段 JS 片段
- THEN 该片段 MUST 直接以启用状态保存

#### Scenario: 写入响应区分草稿与生效

- WHEN 机器凭证写入任一自定义配置成功
- THEN 响应 MUST 明确标识本次写入结果是已生效还是已存为草稿
