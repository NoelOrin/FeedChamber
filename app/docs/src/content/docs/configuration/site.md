---
title: 站点设置
description: 站点名称与 URL、SEO、注册开关、分页、维护模式与公告的配置项和生效方式
---

站点设置是一组住在 `settings` 表里的键，改完立即生效，不用碰环境变量，也不用重启
（分页有一项例外，下面单独标出）。后台入口 `/admin/settings`，按 Tab 分成
站点 / SEO、用户与认证、内容与通知、第三方登录、存储设置五段；这一页只讲其中的
站点身份、SEO、注册、分页、维护模式与公告。

## 前置条件

- 账号具备 `setting:manage` 权限，默认 `admin` 角色自带。
- 能登录 `/admin`，认证方式见 [认证与安全](/configuration/authentication/)。
- 想直接读写原始键值：`POST /api/v1/admin/settings` 返回全部设置，
  `POST /api/v1/admin/settings/{key}/update` 改单个键，`POST /api/v1/admin/settings/batch` 批量写入。

## 站点信息

`/admin/settings?tab=site` →「站点信息」。

| 键 | 默认值 | 作用 |
|----|--------|------|
| `site_name` | `Feed Chamber` | 站点名。导航、页面标题后缀、TOTP 发行方都用它 |
| `site_description` | `NoelOrin 的个人博客` | 一句话简介，`seo_description` 为空时作为兜底描述 |
| `site_logo` | 空 | Logo 图片 URL，`og_image` 为空时作为分享图兜底 |
| `site_url` | 空 | 站点基准地址，sitemap、robots.txt、canonical、og:url 都取自它 |

正式部署时把 `site_url` 填成完整地址（带协议，不带结尾斜杠）。留空不会阻止启动，
但 sitemap、robots.txt 与 canonical 会退化成 `http://localhost:8080`，启动日志里会有
一条对应告警。本地开发可以不管。

## SEO 与站长验证

同一 Tab 的「SEO 设置」分组。四个字段层层兜底，填得越少越依赖站点信息：

| 键 | 兜底来源 | 说明 |
|----|----------|------|
| `seo_title` | `site_name` | 站点默认标题，同时作为子页标题的后缀 |
| `seo_description` | `site_description` | 搜索结果与社交卡片描述 |
| `seo_keywords` | 种子值 | 逗号分隔，输出为 `keywords` meta |
| `og_image` | `site_logo` | `og:image` 与 Twitter 卡片图 |

`google_site_verification` 与 `baidu_site_verification` 也在这一组：把站长平台给出的
验证串原样粘进去，前台会输出对应的验证 meta 标签。这两个值和站点信息一样属于公开
设置，任何人可读——它们本来就要出现在页面源码里。

他人主页是否进搜索索引由 `user_profile_indexable` 控制（默认 `false`，输出 `noindex`）。
个人主页属于薄内容，多数站点保持关闭，只索引帖子和版面。

## 注册

`/admin/settings?tab=user` →「用户与注册」。

| 键 | 默认值 | 取值 |
|----|--------|------|
| `register_enabled` | `true` | 关掉后注册接口直接拒绝（错误码 1013） |
| `register_mode` | `all` | `all` / `email` / `username` / `invite` / `approve` |

五档模式的实际差别：

| 模式 | 后台标签 | 行为 |
|------|----------|------|
| `all` | 开放注册 | 用户名或邮箱至少填一项 |
| `email` | 仅邮箱注册 | 必须填邮箱；用户名留空时取邮箱 `@` 前一段 |
| `username` | 仅用户名注册 | 必须填用户名 |
| `invite` | 邀请码注册 | 必须带有效邀请码，邀请码在 `/admin/invites` 生成 |
| `approve` | 审批注册 | 注册成功但角色是 `pending_user`，审批通过才算正式用户 |

审批入口在 `/admin/users` 的行操作里（接口 `POST /api/v1/admin/users/{id}/approve_registration`）。
通过后角色改为 `user` 并吊销该用户全部会话，需要重新登录。

## 分页

`/admin/settings?tab=content` →「发帖与评论」分组里的分页三项。

| 键 | 默认值 | 说明 |
|----|--------|------|
| `page_default_size` | `20` | 默认每页条数 |
| `page_max_size` | `100` | 每页上限；超过它的 `page_size` 会被回落成默认每页数 |
| `article_page_size` | `20` | 前台列表每页文章数 |

:::caution[生效时机不同]
`article_page_size` 由前端每次读取公开设置，改完刷新页面即生效；
`page_default_size` 与 `page_max_size` 只在进程启动时载入服务端，改完要重启才完全生效。
:::

## 维护模式

同样在 `?tab=content`。

| 键 | 默认值 | 说明 |
|----|--------|------|
| `site_maintenance` | `false` | 打开后非管理员的写请求返回 5005（HTTP 503） |
| `site_maintenance_message` | `站点维护中，请稍后再试` | 返回给用户的具体文案 |

维护期不是「什么都挡」，放行规则是白名单：`admin` 角色不受影响；只读浏览（`GET` 与
`/list`、`/search`、`/detail` 这类读接口）继续可用；登录、注册、刷新会话、找回密码、
`/health`、`/swagger` 与 OAuth/OIDC 协议端点保持可达。只有非管理员的写操作会被拦。

## 公告

公告不在设置页，走独立模块 `/admin/announcements`。

| 字段 | 说明 |
|------|------|
| 标题 / 正文 | 正文是 Markdown |
| 显示方式 | `1` toast、`2` 顶部条幅、`3` 弹窗、`4` iframe 嵌入（正文填外部 URL） |
| 位置 | toast 的六向定位，默认 `top-right` |
| 置顶 / 排序 | 两者同时生效，置顶优先 |
| 过期时间 | 到期后不再展示 |

发布与下线分别走 `publish`、`unpublish` 两个动作，草稿与已发布分开管理。
`announcement_auth_required=true` 时公告列表要求登录后才可读。

## 验证

```bash
# 公开设置里能读到刚改的值
curl -s -X POST http://localhost:8080/api/v1/settings/public \
  | jq '.data.site_name, .data.register_mode, .data.site_url'

# 公开设置不应包含非公开键
curl -s -X POST http://localhost:8080/api/v1/settings/public | jq '.data | keys'
```

后台侧看三处：重新进入 `?tab=site` 值已回填、首页标题与分享图已变、`/robots.txt` 与
`/sitemap.xml` 里的域名是 `site_url` 而不是 localhost。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 改了 `site_url`，sitemap 还是 localhost | 批量保存没成功，或进程没重启 | 确认批量写入返回 `code: 0`；重启后看启动日志告警 |
| 站点名、描述不更新 | 浏览器缓存了服务端渲染结果 | 强制刷新；以公开设置接口的返回值为准 |
| 注册返回「注册功能已关闭」 | `register_enabled=false` | 打开开关，或换 `register_mode` 档位 |
| 邀请注册一直提示缺邀请码 | 模式是 `invite` 但没建码 | 先到 `/admin/invites` 创建邀请码 |
| 审批注册的用户登录后几乎什么都做不了 | 角色仍是 `pending_user` | 到 `/admin/users` 审批，通过后让用户重新登录 |
| 维护期自己也被挡 | 当前账号不是 `admin` 角色 | 换 `admin` 账号操作，或临时关掉 `site_maintenance` |
