## 背景

`docs/backend-overview.md` §13.2-30 把「30 处 GET 读接口」列为待拍板风险。本次拍板前的实测分类如下（来源：`git log -S` 回溯引入提交 + `app/server/internal/router/*.go` 逐行核对 + `packages/api-client/src/services/*.ts` 消费者统计）。

### 30 处 GET 的分类

**A. 有 GET 语义理由（13 条，MUST 保留 GET）**

| 端点 | 理由 |
|---|---|
| `/.well-known/openid-configuration`、`/.well-known/jwks.json` | OIDC Discovery / JWKS，RFC 8414 / 7517 |
| `GET /oauth/authorize`（Provider）、`GET /sso/authorize` | OAuth2 授权端点，浏览器 302 落地 |
| `GET /oauth/{provider}/callback` | provider 回调重定向，`code` 在 query（RFC 6749） |
| `GET /sso/userinfo` | OIDC UserInfo（RFC 5.3） |
| `GET /sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt`、`/latest.rss` | 爬虫/阅读器协议 |
| `GET /health` | 探活 |
| `GET /upload/file/{key}` | 流式文件字节，`<img src>` 直链 |
| `GET /upload/{id}` | 302 到文件 URL |

**B. 纯 JSON 只读查询（17 条，本次判为合规 GET）**

`/checkin/status`、`/checkin/history?year=&month=`、`/sessions`、`/security/events`、`/security/login-history`、`/posts/{id}`、`/posts/{id}/detail`、`/comments/{id}`（admin）、`/reports/{id}`、`/oauth/providers`、`/oauth/{provider}/auth`（返回 `{auth_url,state}` JSON，非 302）、`/oauth/{provider}/config`、`/oauth/accounts`、`/announcements/{id}`、`/user/profile`、`/user/{id}`、`/user/{id}/profile`。

其中 `postService.getById`、`oauthService.getAuthURL/getProviderConfig/getLinkedAccounts`、`announcementService.getById` 在仓内零消费者；其余只读接口的页面代码全部经 `api-client`，改方法会影响面很小——但本次决定不改造。

## 决策

### 1. 方法判据：有没有副作用

用一条判据替代「一律 POST」：

- 有副作用 → `POST`（且动词后缀）；
- 无副作用（只读、幂等、可安全重试）→ `GET` 或 `POST` 均可，按形态选：
  - 单资源读取 / 轻量状态查询 → `GET`（URL 是标识不是动作）；
  - 列表 / 分页 / 多条件筛选 → `POST` + `/list`（参数多、body 更稳）；
- **写操作不得以 `GET` 暴露**：防止 `GET /sessions/{id}/revoke` 这类会被预取/爬虫触发的形态。

### 2. 同一资源只能有一种读方法

禁止同一读取路径同时提供 GET 与 POST 版本。原因：双读入口意味着两份 handler 接线、两份 openapi、两份缓存语义，且无法回答「哪个是权威」。现状核对：30 条 GET 与现有 POST 路由均不共享同一「读取路径」，无需改造。

### 3. query string 规则按方法区分

- `GET`：允许 query 承载过滤/分页（`page`/`pageSize` 字段名不变），因为这是 GET 的唯一参数通道；
- `POST`：仍禁止 query 承载业务参数，保持现有约定；
- 这一条解决了 `GET /checkin/history?year=&month=` 的规则冲突。

### 4. 系统 / 协议 / 文件端点单列白名单类别

不是「豁免清单」（不枚举具体路由，避免随路由增减腐化），而是按类别：探活、爬虫协议、OAuth2/OIDC 协议端点、文件字节/下载、认证流程端点。评审新增端点时先归类。

### 5. 仍然禁止 RESTful

`PUT` / `PATCH` / `DELETE` 继续禁用；URL 不采用「层级嵌套 + 方法承载语义」的 REST 做法。允许 GET 不等于允许 REST：GET 在这里只承担「只读」这一种语义。

## 备选方案与放弃理由

| 方案 | 放弃理由 |
|---|---|
| **全量整改**：17 条业务读改 POST，30 条收敛到 13 条 | 需改 17 handler + 17 swag 注解 + 17 个 `api-client` 方法 + swag/`openapi.json` 重生成 + `feedchamber_test.sh` 11 行；收益是把 17 条合法的只读语义改成错的方法。而且 13 条协议端点仍要白名单，规则复杂度不变。 |
| **全量登记豁免**：spec 显式列出 30 条 GET 为豁免 | 路由清单会随增删腐化；且不解释「为什么允许」，下一个接口仍靠直觉。 |
| **只加 CI 断言，规范不动** | 规范继续与代码矛盾（MUST POST vs 30 条 GET），断言反而把所有现存接口判红。 |
| **本次方案**：改判据 + 类别白名单 | 规范与现实一致，新增端点有可执行判据，代码零改动。 |

## 边界情况

- **`GET /oauth/{provider}/auth` 会写 KV 的 `oauth_state`**：属认证流程起点的临时凭证，归「认证流程端点」类别；它不改变业务状态，重放只会产生新的 state，不构成业务副作用。
- **`GET /upload/{id}` 是 302**：语义是"取文件"，归文件字节类别。
- **`GET /sessions` 等服务端读取**：读取本身无副作用；会话活跃时间更新由网关 `Enforce()` 统一负责，与端点方法无关。
- **SSE**（`POST /notifications/stream`）：既有实现用 POST，方法不由本次规则约束变更。

## 验证方式

1. 规范一致性：全仓 grep `一律 POST` / `全部 HTTP 接口使用 POST` / `默认 method: 'POST'`，除归档 change 外不得残留与新规则矛盾的表述。
2. 代码零改动：`git status --porcelain app/ packages/*/src` 应只包含本次前后端无代码改动的结果（本次预期为空）。
3. 路由分类核对：`grep -rn '\.Get(' app/server/internal/router/*.go` 的 30 条逐条归入 A/B 类，无第三条类别。
