---
title: 站点自定义
description: 主题 token 覆盖、自定义 CSS/JS/HTML 片段、导航项、首页模块与自定义页的配置方式与生效规则
---

站点自定义让站长不改代码就能调整外观与结构，共五个板块：**主题 token**、**片段**
（CSS / JS / HTML）、**导航项**、**首页模块**、**自定义页**。它们都从公开接口
`POST /api/v1/customization/public` 聚合输出，前台一次请求拿全。

它们不进 `settings` 的键值对，而是各自独立成表——逐条启停、逐条审计与按条件索引，
都是「允许注入任意内容」场景的硬需求，一个 JSON 大字段给不了。

## 前置条件

- 读权限 `customize:read`，写权限 `customize:write`，发布类动作 `customize:publish`。
- 后台入口：`/admin/customize/theme`、`/snippets`、`/nav`、`/home`、`/pages`。
- 前台页面：自定义页在 `/p/{slug}`。

## 主题 token

`/admin/customize/theme`。一条覆盖的身份是 **(token, mode)**：同一个 token 可以分别给
明亮和暗色各写一份值，互不覆盖。

| `mode` | 作用范围 |
|--------|----------|
| `all`（默认） | 同时写进 `:root` 与 `.dark` |
| `light` | 只写 `:root` |
| `dark` | 只写 `.dark` |

只接受 CSS 自定义属性名（`--xxx`）与不含 `; { } < >` 和换行、长度 ≤512 的值。渲染时
被拼成一段 `<style id="fc-theme-overrides">`，排在全局样式之后，所以能盖住默认值。这一层
刻意只做「改色改圆角」——想写复杂选择器就用下面的 CSS 片段。改错了可单条删除（恢复默认）
或一键清空全部覆盖。

## 片段：自定义 CSS / JS / HTML

`/admin/customize/snippets`。每条片段有类型、注入位置、内容与启用开关。

| 类型 | 默认位置 | 说明 |
|------|----------|------|
| `css` | `body_end` | 放进 `<style>` |
| `js` | `head` | 放进 `<script>` |
| `html` | `body_end`（**固定**） | 放进容器 `div`；裸 HTML 放 `head` 会在 `<body>` 之前被解析 |

`head` 段的 CSS/JS 在服务端内联渲染，首屏即生效；`body_end` 段由客户端在渲染后注入，
不阻塞首屏。两条路径都只注入前台，**`/admin` 段永不注入**——后台是纯客户端渲染且与前台
同源，把站长脚本放进后台等于放大风险面。启用 JS 或 HTML 片段需要 `customize:publish`，
不只是 `customize:write`：这等于让任意脚本对全站访客执行，风险级别与「发布」相同。CSS
片段只影响样式，最坏是站点变丑，可以随时关掉。

## 导航项

`/admin/customize/nav`。字段：`label`（文字）、`href`（链接）、`icon`、`sort`（排序）、
`enabled`，以及可见性 `visibility` + `min_level`。

| `visibility` | 谁能看到 |
|--------------|----------|
| `public` | 所有人，含游客 |
| `login` | 仅已登录用户 |
| `level` | 等级 ≥ `min_level` 的用户（此时 `min_level` 必须大于 0） |

导航项渲染在左侧栏的「导航」分组下。**表为空时前台不渲染任何自定义导航**，回退到内置
默认导航——清空配置不会让导航变空白。前台读的是同一份公开聚合，改完刷新即可见。

## 首页模块

`/admin/customize/home`。四种类型：

| `kind` | 内容 | `config` 示例 |
|--------|------|---------------|
| `latest` | 最新帖子列表 | `{"limit":5}` |
| `hot` | 热门帖子列表 | `{"limit":5}` |
| `spaces` | 版面入口宫格 | 空 |
| `html` | 站长自定义 HTML | `{"html":"<div>...</div>"}` |

字段还有 `title`、`sort`、`enabled` 与和导航项相同的 `visibility` / `min_level`。
`config` 必须是合法 JSON，解析失败时按该类型默认参数渲染、不会让首页报错；`limit` 取
1–20，缺省 5。`html` 模块的内容放在 `config.html` 里，空内容会被写入接口拒绝。模块按
`sort` 渲染在 `/timeline` 顶部，**没有配置时不渲染任何区块**，首页保持原时间线布局。

## 自定义页

`/admin/customize/pages`，前台地址 `/p/{slug}`，正文是 Markdown。

| 字段 | 说明 |
|------|------|
| `slug` | 唯一；不能包含 `/ ? # &` 与空格，也不能与既有顶层路由同名（如 `admin`、`api`、`login`、`spaces`） |
| `title` | 页面标题 |
| `content` | Markdown 正文 |
| `status` | `draft` / `published` |

草稿对访客表现为 **404 而不是 403**——否则等于告诉别人「这里有个未发布页」。
发布与下线走 `POST /api/v1/admin/customization/pages/{id}/publish`，`publish=false` 即撤下。

## 分级生效：机器凭证强制草稿

写入来源决定内容能不能直接生效：

| 来源 | 行为 |
|------|------|
| 站长在后台点击（会话凭证） | 按请求的 `enabled` / `status` 原样写入 |
| 机器凭证（API Key、OAuth 客户端、MCP 工具） | 高风险内容**强制存为草稿或未启用**，需人工在后台开启 |

高风险指的是：`js` 与 `html` 片段、导航项、`html` 类型的首页模块、自定义页。导航项也在
其中，因为它的 `href` 可以写成 `javascript:`，效果等同执行脚本。低风险写入（CSS 片段、
主题 token、`latest`/`hot`/`spaces` 模块）不受此限制。

机器凭证额外持有 `customize:publish` 时可跳过强制草稿。判断只看凭证自身 scope，不看
角色继承——避免「给 Key 授了 admin scope 就顺带获得注入 JS 的能力」。

写入响应里的 `applied` 与 `reason` 直接说明这次改动有没有落地：

```json
{ "snippet": { "...": "..." }, "applied": false, "reason": "机器凭证未持 customize:publish 权限，已保存为草稿，需在后台启用" }
```

## 审计与回滚

每次写入都会记一条审计（后台 `/admin/audit-logs` 可查），动作名形如
`customize.snippet.update`、`customize.tokens.set`、`customize.page.publish`。
详情里记录的是**变更摘要**而不是全文：长度从多少变成多少，加上改后内容的前 120 个字符。
整段 JS 逐字入库会让审计表迅速膨胀，而排查时真正需要的是「哪一段被换掉了」。

回滚没有「一键还原上一版」按钮，按板块各自处理：

| 板块 | 回滚方式 |
|------|----------|
| 片段 | 关掉 `enabled`（最快），或把内容改回去 |
| 主题 token | 删除单条覆盖，或清空全部 |
| 导航项 / 首页模块 | 改回原字段，或删除该项 |
| 自定义页 | 改回内容，或 `publish=false` 撤下 |

顺序建议：先止血（关片段 / 撤发布），再从审计日志确认改动时间与内容，最后才改回去。

## 验证

```bash
# 1. 公开聚合：五个分区都在（未登录也能读）
curl -s -X POST http://localhost:8080/api/v1/customization/public -d '{}' \
  | jq '{nav: (.data.nav|length), modules: (.data.home_modules|length), body: (.data.body_snippets|length)}'

# 2. 自定义页：已发布能取到，草稿应 404
curl -s http://localhost:8080/api/v1/pages/about | jq '.code'
```

浏览器侧：主题 token 改动后 `<style id="fc-theme-overrides">` 的内容会变；`head` 片段出现在
`<head>` 里，`body_end` 片段出现在页面源码末尾，容器带 `fc-custom-*` 形式的 id。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 加了 JS 片段但没执行 | 片段没启用，或是机器写入被存成草稿 | 到 `/admin/customize/snippets` 打开开关 |
| 机器调用返回 `applied: false` | 缺 `customize:publish` scope | 属预期；人工在后台启用，或给 Key 补 scope |
| `/admin` 里样式也变了 | 不可能是注入——后台不注入任何片段 | 检查是不是浏览器扩展或全局 CSS |
| token 保存报错 | 名字不是 `--xxx`，或值里有 `; { } < >` | 换成合法变量名与值；复杂样式用 CSS 片段 |
| 自定义页 404 | 还是 `draft` | 调 publish 接口发布 |
| slug 报冲突 | 撞上了保留字或已存在的页面 | 换 slug；保留字列表包含 `admin`、`api`、`login` 等 |
| 导航项不显示 | `enabled=false`，或 `visibility=level` 而访客等级不够 | 检查开关与可见性设定 |
