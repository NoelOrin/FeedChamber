---
title: 作为 MCP Server
description: 工具目录、鉴权、分级生效与工具启停
sidebar:
  order: 2
---

## 连接

```json
{
  "mcpServers": {
    "feedchamber": {
      "url": "https://你的站点/mcp",
      "headers": { "X-API-Key": "ak_<你的Key>" }
    }
  }
}
```

## 工具目录（15 个）

### 读工具（默认启用）

| 工具 | 权限码 | 说明 |
|------|--------|------|
| `search_posts` | `post:read` | 搜索帖子 |
| `get_post` | `post:read` | 读单帖（受可见性约束） |
| `list_spaces` | `post:read` | 版面列表 |
| `list_tags` | `post:read` | 标签列表 |
| `get_user_profile` | `user:read` | 用户资料 |
| `list_snippets` | `customize:read` | 站点自定义片段 |

### 内容写工具（默认**关闭**）

| 工具 | 权限码 |
|------|--------|
| `create_post` | `post:create` |
| `create_comment` | `comment:create` |
| `like_post` | `content:like` |
| `favorite_post` | `favorite:self` |

### 站点自定义写工具（默认**关闭**）

| 工具 | 权限码 |
|------|--------|
| `upsert_snippet` | `customize:write` |
| `set_theme_tokens` | `customize:write` |
| `upsert_nav_item` | `customize:write` |
| `upsert_home_module` | `customize:write` |
| `upsert_custom_page` | `customize:write` |

**不含**创建 / 提权 API Key 的工具——避免凭证自我扩权。

## 鉴权判据

API Key 的 scope 是**上限**：`scope` 与角色权限必须**同时**覆盖，才允许调用该工具。

`customize:publish` 更进一步：**只认凭证自身的 scope，不走角色继承**。
原因：admin 角色默认持有它；若接受继承，那么「admin 创建的任意 Key」都自动具备
让 JS 直接生效的能力，草稿保护对最需要它的那批凭证就完全失效了。

## 分级生效（机器凭证的草稿闸门）

| 写入内容 | 行为 |
|----------|------|
| CSS 片段 / 主题 token | 持 `customize:write` 即**直接生效**（最坏是站点变丑，可撤销） |
| JS / HTML 片段、导航、首页模块、自定义页 | **强制存草稿**，需人工在后台启用 |
| 凭证 scope 含 `customize:publish` | 可跳过草稿直接生效 |

## 工具启停

后台 `/admin/mcp` 可以逐项启停。写工具**默认全部关闭**——这是安全默认值。

启停结果会**落库**到设置 `mcp_disabled_tools`，重启与多实例下都保持一致。
编码规则（相对默认的覆盖）：

- 名单里出现读工具名 → 停用该读工具
- 名单里出现写工具名 → 该写工具保持停用
- 名单里出现 `-<写工具名>` → **显式启用**该写工具

用负号而不是「完整启用名单」的原因：完整名单一旦被误清空（例如后台保存了空表单），
所有写工具会一起打开；负号表达的是「显式开启某一项」，误清空只会回到全关。

## 结果体积

单次工具结果默认上限 8192 字节（设置 `mcp_max_result_bytes`），超限会截断正文并标注。
