# OrinForum 全面代码审查报告

> 审查日期：2026-06-20
> 审查范围：Go 后端、Web 前端（SolidJS）、Dashboard（React）、共享包、基础设施
> 审查方式：36 路并行 AI 审查代理
> 风险评分：10 分制（越高越危险）

---

## 全局风险总览

| 模块 | 风险评分 | CRITICAL | HIGH | MEDIUM | LOW |
|---|---|---|---|---|---|---|
| Go 后端 | **5/10** 🟡 | 0 | 5+ | 10+ | 15+ |
| Web 前端 | **3/10** 🟢 | 0 | 0 | 2+ | 3+ |
| Dashboard | **6/10** 🟡 | 6 | 8 | 10+ | 10+ |
| 共享包 | **5/10** 🟡 | 0 | 3+ | 5+ | 9 |
| 基础设施 | **6/10** 🟡 | 3+ | 7+ | 20+ | 15+ |

---

## 🚨 最高优先级修复项

### 安全漏洞



---

## 一、Go 后端 — 风险评分  7/10 🟠（CRITICAL 全部修复）

### 严重问题


### 中危问题（摘要）

### 低危问题（摘要）

- `router.go` 多处 — 尾部斜线不一致、路由注册顺序、参数名混用等 ⚠️ 部分修复：批量 commit 有修复，但尾部斜线（`"/"` vs `""`）仍不一致

---

## 二、Web 前端（SolidJS）— 风险评分 8/10

### 严重问题


### 低危问题（摘要）

---


### 中危问题（摘要）

| 问题 | 位置 | 描述 |
|---|---|---|
### 低危问题（摘要）

- `ProShell.tsx:141` — `languageMenuItems` 内联创建
- `posts/index.tsx:17` — `useRef<ActionType>(undefined!)` 断言
- `dashboard/index.tsx:29` — `fetchData` 与 mount useEffect 重复
- `api-client/src/client.ts:55, auth.ts:41` — `console.log(adapter)` 调试语句残留
- 多处 — 错误消息反映 API 技术细节
- `api-client/src/constants.ts:87` — 基础 URL 硬编码 `'/api/v1'`
- `router.tsx:220-243` — 缺少通配符路由
- `roles/detail/index.tsx:18` — `flattenSpaces` 参数 `items: any[]`
- `users/detail/index.tsx:23` — `fetchUser` 未用 useCallback 包裹

---

## 四、共享包 — 风险评分 7/10

### 严重问题

- **[CRITICAL] packages/api-client/src/auth.ts:136-173** — Token 刷新竞态条件：`isRefreshing=true` 与 `promise=null` 窗口间并发 401 创建第二个刷新请求。
- **[CRITICAL] openapi.json** — 所有 136 个端点缺少 `operationId`。

### 高危问题

- **[HIGH] packages/api-client/src/client.ts:18** — `DEFAULT_TIMEOUT_MS = 3000`，对仪表板操作过于激进。

### 中危问题

| 问题 | 位置 | 描述 |
|---|---|---|
| CSS 变量不一致 | `app/web/src/assets/css/global.css:79` | 18+ 处引用 `var(--themeColorMain)` 而非 `var(--color-primary)` |

### 低危问题

- `sse-client/client.ts:62-71,153-158` — `disconnected` 回调触发两次
- `sse-client/client.ts:86` — `abortController` 无条件重新赋值未中止之前 controller
- `sse-client/client.ts:96-101` — fetch 无超时机制
- `sse-client/client.ts:138-151` — SSE 多行载荷被静默截断
- `tokens/dist/tokens.css:29-45` — 17 个 `--color-seed-*` CSS 变量暴露但未被引用
- `tokens/dist/tokens.css:76-110` — 11 个 spacing/z-index/duration 变量未使用
- `tokens/README.md:3` — 声明已不存在的 `@orinforum/web` 包
- `tokens/README.md:8-18` — 架构图遗漏 `layout.js/d.ts` 和 `dashboard.js/d.ts`
- `tokens/scripts/build.ts:210` — `colorBgContainer` 映射到 `item` 而非 `card`

---

## 五、基础设施 — 风险评分 7/10

### 严重问题

- **[CRITICAL] app/web (SolidJS)** — 整个前端零测试，50+ 组件/hooks/utils 未经覆盖。
- **[CRITICAL] app/dashboard** — 管理后台零测试框架配置。
- **[CRITICAL] handler/ (27 源文件)** — 所有 handler 文件均无测试。
- **[CRITICAL] repository/ (49 源文件)** — 仅 2 个测试文件，核心数据访问层未测试。
- **[CRITICAL] openapi.json** — 所有 136 个端点缺少 `operationId`。

### 高危问题

- **[HIGH] service/auth** — 最关键服务（Register/Login/token）完全无测试。
- **[HIGH] service/post** — CRUD/搜索/置顶/锁定无测试。
- **[HIGH] service/space** — 板块树/CRUD 无测试。
- **[HIGH] service/banned** — 封禁逻辑与 auth 中间件集成无测试。
- **[HIGH] Dockerfile:27** — 无 USER 指令，容器以 root 运行。
- **[HIGH] @orinforum/tokens:package.json** — exports map 缺少 `"."` entry，创建双重解析路径。
- **[HIGH] @orinforum/dashboard-vite:package.json** — axios 和 lodash 未使用。
- **[HIGH] dashboard:package.json** — 缺少 `@types/lodash`。
- **[HIGH] openapi:/categories/** — 4 个路径 OpenAPI 存在但 Go 后端完全不存在。
- **[HIGH] openapi:9 个管理端端点** — response 架构为空对象 `{}`。
- **[HIGH] mobile-build.yml** — lint/check 步骤缺失。
- **[HIGH] mobile-build.yml** — 标签推送触发器缺失（README 记录但 YAML 未实现）。
- **[HIGH] 整个仓库** — 无任何安全扫描（SAST/CodeQL/密钥扫描/依赖审计）。

### 中危问题（摘要）

#### Docker

- 多阶段构建：`apk del` 未在阶段结束前执行
- `COPY --from=go-builder /build/.env\*` 复制空内容（`.env*` 在 `.dockerignore` 中）
- `COPY <<'EOF'` heredoc 需 BuildKit，失败于 legacy builder
- `.dockerignore` 缺少 `.turbo/`、`*.log`、`.DS_Store` 等
- `pnpm@latest` 未锁定版本

#### Turbo

- `start`/`preview` 缺少 `persistent: true` + `cache: false`
- `preview` 缺少 `dependsOn: ["^build"]`
- `lint:fix`/`format:check` 为孤儿 pipeline 任务
- `build`/`test` 无 `inputs` glob

#### Biome

- `noUnusedVariables: "warn"` 应为 `error`
- `useExhaustiveDependencies: "warn"` 应为 `error`
- `useHookAtTopLevel: "warn"` 应为 `error`
- `experimentalScannerIgnores` 缺少 `.turru`、`.next`、`coverage`
- `noExplicitAny: "off"` 关闭 any 检查
- `noNonNullAssertion: "off"` 绕过空安全
- `vcs.useIgnoreFile: false` 屏蔽 `.gitignore` 集成

#### OpenAPI

- 23+ Go 路由未记录（OAuth、SSO、follow/status、like/status 等）
- 公开端点不正确要求 BearerAuth（`/spaces/tree`、`/posts/list` 等）
- 108 个端点缺少 500 响应（占 80%）
- 15 个 BearerAuth 端点缺少 401 响应
- `admin/batch/*` 等端点缺少 403 响应
- 4 个端点摘要与 Go 路由器 handler 不匹配
- POST 而非 DELETE 用于删除操作


#### 未使用依赖

- `@orinforum/web` — `@tanstack/solid-form`、`zod`、`@tanstack/solid-query-devtools`、`@tanstack/solid-router-devtools`
- `@orinforum/dashboard-vite` — `axios`、`lodash`
- `@ant-design/pro-layout` — 被 `@ant-design/pro-components` 重新导出

### 低危问题（摘要）

- `shared-utils`、`sse-client`、`api-client` — 缺少 lint/format/test 脚本
- `tokens` — `tokens:build` 与 `build` 脚本冗余
- `@tanstack/solid-query` / `@tanstack/react-query` — 落后最新版本 11 minor
- `rehype-sanitize` + `isomorphic-dompurify` 净化依赖重叠
- `lodash` — 处于维护冻结状态
- 测试 assertion lib 不一致（混用 raw `t.Errorf`/`t.Fatal`）
- mock 广度和碎片化不足
- CI/CD：无测试执行、pmpm 缓存缺少 restore-keys 回退、secrets 以明文写入 gradle.properties、无部署门禁与环境隔离
- README 与 YAML 不一致

---

## 六、建议修复顺序

### 第一阶段：安全与权限（立即修复）

1. ~~Gateway 权限映射表与实际 router 路径对齐（`permission.go:117-197`）~~ ✅ 已修复：路径对齐 + 补全 pin/lock/batch/activity/banned + 空间级权限统一到 Gateway
2. ~~封禁中间件注册到路由（`gateway.go:78`）~~ ✅ 已修复（batch 2 + batch 3）
3. ~~XSS 攻击链修复：ProfileBio + MarkdownRenderer URI 校验 + TipTap image validate + sanitize 补全~~ ✅ 已修复（safeUrl + rehype-sanitize + isomorphic-dompurify）
4. ~~CORS 策略修复：显式 Origin + `Vary: Origin` 或拆分无 cookie 场景~~ 🟢 设计决策：使用 Bearer token 认证，`*` 适配无 cookie 场景
5. ~~S3 预签名添加 `content-length-range`（`s3.go:92`）~~ ✅ 已修复（`532c10d` 添加了 content-length-range 条件）
6. ~~SMTP 头部注入修复~~ ✅
7. ~~MongoDB `$regex` 注入修复~~ ✅
8. ~~邮件正文 HTML 消毒~~ ✅

### 第二阶段：架构与正确性（短期修复）

11. ~~分类路由完整实现~~ ✅ 已修复（spaces 替代 categories）
12. ~~Post 变更端点添加 `RequirePermission` 中间件~~ ✅ 已修复：Gateway 统一权限，不依赖 router 级中间件
13. ~~**handler 层错误信息脱敏（统一返回通用错误码）**~~ ✅ 已修复
14. 批量操作端点添加输入大小限制
15. ~~配置启动时验证必填字段~~ ✅ 已修复
16. 邮件发送器添加超时和速率限制
17. 临时存储失败清理逻辑
18. 并发 goroutine 纳入生命周期管理（WaitGroup + 速率限制）

### 第三阶段：代码质量（中期修复）

19. Web 前端认证守卫统一化（路由组默认 `beforeLoad`）
20. 异步 effect 模式规范化（消除 `createEffect(async () =>`）
21. 表单提交防重（全局 `loading/submitting` 守卫）
22. Biome 启用严格规则（noExplicitAny、noNonNullAssertion、exhaustive-deps）
23. TypeScript 类型安全：消除 `{} as T` 反模式、类型化 loader
24. SSR hydration 安全：`new Date()` 包装为 hook
25. 模型字段约束补全（索引、默认值、json 标签、时间字段类型统一）
26. OpenAPI 规范与路由注册同步

### 第四阶段：测试与基础设施（长期修复）

27. 前端 Web/Dashboard 配置 vitest + 核心组件测试
28. Go handler 层集成测试
29. Go service 层 mock 化测试
30. repository 层测试覆盖核心 CRUD
31. 依赖清理：未使用的 axios/lodash/TanStack 依赖
32. 版本统一：Biome、tailwindcss、TanStack 工具链
33. Docker 安全加固（USER 指令、pnpm 版本锁定）
34. CI/CD 增强（lint 步骤、安全扫描、缓存策略、部署门禁）
35. 代码分割：Dashboard 迁移至 `React.lazy()` 按需加载
36. 清理残留 `console.log` 调试语句

---

## 修复后二次审查（2026-06-22）

> 审查范围：针对 CRITICAL/HIGH 修复 diff 的二次审查
> 审查方式：caveman-review 逐行 diff 审查

### 🟡 risk（建议修复）

3. **service/auth.go L317/375/408/441 — 四处 `RevokeAllForUser` 吞错误**：`_ = s.bl.RevokeAllForUser(...)` 吊销失败 = 旧 token 仍有效 = 安全漏洞。Fix: 至少 `slog.Error("revoke all tokens failed", ...)` 记录。❌ 未修复

4. **handler/auth.go L182-197 — KV 故障放行已吊销 token**：`IsRevoked`/`IsUserRevoked` 的 error 被 `_, _` 忽略。KV 不可用时返回 `false, err` → `revoked=false` → 放行已吊销 token。Fix: `if err != nil { pkg.Fail(ctx, pkg.TOKEN_WRONG, "令牌验证失败"); return }`。❌ 未修复

5. **handler/auth.go L202-205 — 旧 refresh token 吊销失败被吞**：`_ = h.bl.Revoke(...)` 失败 = 已泄露的 refresh token 仍可用。Fix: 记录错误日志。❌ 未修复

6. **gateway/gateway.go L86 — 硬编码 `role == "ban"` 字符串**：若封禁角色名配置化或大小写变化则绕过。Fix: 用 `model.HasBanRole(role)` 与 Login/RefreshToken 保持一致。❌ 未修复

7. **gateway/gateway.go L53-56 — 手写 404 响应**：`ctx.StatusCode(404)` + `ctx.WriteString("")` 跳过 Iris 的 `ctx.StopExecution()`。若未来代码在 return 前插入逻辑可能泄漏。Fix: 用 `ctx.StopWithStatus(404)` 或 `pkg.Fail(ctx, pkg.NOT_FOUND, "")`。❌ 未修复

8. ~~**ProfileBio.tsx L45 — SSR 中 `isomorphic-dompurify` 可用性**：SSR 场景下 `onMount` 不执行 → 走 `DOMPurify.sanitize(props.bio)`。`isomorphic-dompurify` 在 Node 端依赖 `jsdom`，若未正确初始化会抛异常。Fix: 确认 SSR 中 `isomorphic-dompurify` 可用，或对 SSR 场景做 fallback（返回纯文本）。~~ ✅ 已在 `b1e113d` 修复

### 🔵 nit（可选）

9. ~~**MarkdownEditor/index.tsx L99 — `safeSrc` unused import**：`mdToHtml` 已走 `rehypeSanitize` 管道，不再需要前端额外校验。Fix: 删除无用 `import { safeSrc }`。~~ ✅ 已清理

10. ~~**safe-url.ts L5 — `//` 协议相对 URL 放行**：`data:` 被拦截但 `//evil.com` 放行，行为不对称。低风险（浏览器不执行协议相对 URL 的 JS）。如需严格可加白名单。~~ ✅ 已修复（`isSafeRelativeUrl` 开头加 `if (url.startsWith('//')) return false`）

11. **extensions.ts L25-38 — `SafeImage` 空 src 渲染**：`renderHTML` 返回 `{}` 不渲染 src，但 TipTap 仍渲染 `<img>` 标签（无 src = 空图占位）。Fix: 返回 `class: 'hidden'` 或不渲染整个节点。❌ 未修复

12. **kvstore/sqlite.go — GC DELETE 仍触发 `dispatchKVChange`**：GC 不预填 `pendingDeleteKeys`，C 回调触发时 `delete(s.pendingDeleteKeys, rowID)` 操作不存在的 key，无害但浪费。Fix: 可加早期返回或 GC 中也预填 key 并发送通知。❌ 未修复

---

## 三批审查（2026-06-24）— Gateway 权限统一 + 安全修复 batch 2

> 审查范围：`2550a99` (fix: 代码评审高危漏洞全量修复 batch 2), `8c8cde3` (refactor: 移除冗余 RequirePermission), `99c6267` (fix: 统一权限检查 + 空间级权限)
> 审查方式：10 路并行 AI 审查代理（5 个正确性角度 + 3 个代码质量角度 + 海拔 + 约定）→ 1 票验证 → 扫尾
> 严重性：🔴 CRITICAL / 🟠 HIGH / 🟡 MEDIUM / 🔵 LOW

### 🔴 CRITICAL

1. ~~**`/api/v1/admin/settings` 绕过权限检查** — `permission.go:229` vs `router.go:326`。前缀映射注册为 `/api/v1/admin/settings/`（带尾部斜杠），路由注册为 `as.Post("", h.Setting.GetAll)` 产生路径 `/api/v1/admin/settings`（无斜杠）。`strings.HasPrefix("/api/v1/admin/settings", "/api/v1/admin/settings/")` 为 false，精确/模式/空间模式均未命中，回退 `return true`。
   - **后果**：任何认证用户可调用 `POST /api/v1/admin/settings` 获取全部系统设置。
   - **修复**：在 `initRouteMap()` 添加 `p.addExact("/api/v1/admin/settings", "setting:manage")`。~~ ✅ 已修复

### 🟠 HIGH

2. ~~**协议相对 URL 绕过 safeUrl**~~ ✅ 已修复（`safe-url.ts` 增加 `//` 检查 + `extensions.ts` 对应修复）
3. ~~**Moderator 角色缺少 PermCommentUpdate — 功能性回归**~~ ✅ 已修复（`model/permission.go` moderator 角色已添加 `PermCommentUpdate`）
4. **RefreshToken 撤销-再签发顺序导致用户卡死** — `handler/auth.go:195-207`。`GetRefreshToken` 先撤销旧 refresh token（line 197, `_ = h.bl.Revoke(...)` 吞错误），再签发新 token pair（line 200）。若 `RefreshTokenPair` 失败（DB 超时、连接池耗尽），旧令牌已销毁、新令牌未签发。错误吞没同时使防重放保护失效（KV 写入失败时旧令牌仍可重放）。
   - **后果**：短暂 DB 故障期间所有 refresh 请求均导致用户被迫重新登录。
   - **修复**：成功生成新 token pair 后再撤销旧令牌，或使用原子交换（单 KV 事务）。
5. **LogoutAll 吞没 KV 错误，令牌静默保持有效** — `session.go:248-271`。`LogoutAll` 删除 DB 会话记录后对 KV 黑名单写入全部使用 `_ = kv.Set(...)` 吞错误。KV 不可用时 `user_bl:{uuid}` 和 `bl:{jti}` 均未写入。`AuthChecker.Middleware` 查询 KV 黑名单返回 `(false, nil)`，旧 JWT 仍通过认证。
   - **后果**：用户看到"All sessions revoked"但令牌实际未被撤销。`TrackingMiddleware` 在下一次认证请求时重新创建会话行。完全静默的失效。
   - **修复**：在删除 DB 记录前执行 KV 写入（失败时回滚 DB 操作），或至少记录 KV 失败。

### 🟡 MEDIUM

1.  **OAuth 撤销端点缺少 `return` 后 `StopWithJSON`** — `oauth_provider.go:162`。`switch grantType` 的 `default` 分支调用 `ctx.StopWithJSON()` 后无 `return`。Iris 的 `StopWithJSON` 仅设置停止标志不阻止后续代码执行。若之后添加代码，会写入已响应后的状态码。
    - **后果**：未来维护者在 `default` 后添加代码可能导致 `superfluous response.WriteHeader` 日志。
    - **修复**：添加 `return`。

### 🔵 LOW

14. **SSO 声称支持 PKCE 但未实现** — `sso_oidc.go:48`。OIDC Discovery 公布 `code_challenge_methods_supported: ['S256', 'plain']`，但 `Authorize` 端点不读 `code_challenge`，`Token` 端点不验证 `code_verifier`。授权码未绑定到质询。
    - **后果**：拦截授权码的攻击者可绕过 code_verifier 证明直接兑换。
    - **修复**：移除 PKCE 声明或实现验证。

15. **头像 `<img src>` 缺少 safeSrc（信息泄露）** — `ContentItem.tsx:27`、`UserCard.tsx:76`。用户控制头像 URL 直接传入 `<img src>` 未过 safeSrc。现代浏览器阻止 `<img src>` 中 `javascript:` 方案（XSS 不可行），但指向攻击者服务器的 `https://` URL 泄露 IP/UA/Referer。
    - **后果**：用户活动可被追踪。
    - **修复**：应用 `safeSrc` + 添加 CSP `img-src` 指令。

16. **受保护路由静默穿透** — `permission.go:148`。`/api/v1/posts/{postId}/favorite`、`/favorites/list`、`/checkin/*`、`/upload/*`、`/dashboard`、`/auth/logout`、`/auth/refresh`、`/users/{userId}/*` 等路由无权限映射条目，回退 `return true`。
    - **后果**：任何新增敏感路由如忘记注册权限映射将自动放行所有认证用户。无强制/林特机制。
    - **修复**：考虑拒绝默认回退或林特规则。

17. **安全事件/查询错误被吞没** — `security.go:127,133,165,179`。`_ = kv.Set(...)`、`_ = db.Create(...)`、`_ = db.Count(&count)` 吞没错误。暴力破解检测的 KV 写入失败 = 不封禁 IP；安全事件创建失败 = 丢失审计日志；登录历史查询失败 = `count=0` → 误报异常地点。
    - **后果**：攻击者可利用 DB/KV 瞬态故障绕过安全检测。
    - **修复**：至少 `slog.Error(...)` 记录失败，关键路径考虑重试。

### ❌ 已驳回（REFUTED）

- **`RefreshToken` 用户级撤销绕过** — 原候选认为 `/auth/refresh`（protected party）的 `RefreshToken` handler 不检查 `IsUserRevoked`。**已驳回**：网关 `AuthChecker.Middleware`（`gateway/auth.go:61-66`）在 handler 前拦截所有 protected 路由的 `IsUserRevoked`。`RefreshToken` handler 无需重复检查。

### 验证状态

| 候选 | 状态 |
|------|------|
| `comment:delete` 映射错误 | CONFIRMED → ✅ 已修复 |
| `/admin/settings` 尾部斜杠绕过 | CONFIRMED → ✅ 已修复 |
| `//evil.com` 绕过 safeUrl | CONFIRMED → ✅ 已修复 |
| APIKey 中间件死代码 | CONFIRMED → ✅ 已修复 |
| Moderator `comment:update` 回归 | CONFIRMED → ✅ 已修复 |
| banCheck 不可达 | CONFIRMED → ✅ 已修复 |
| 头像 safeSrc 缺失 | CONFIRMED（信息泄露 / XSS 不可行） |
| LogoutAll KV 错误吞没 | CONFIRMED |
| RefreshToken 撤销顺序卡死用户 | CONFIRMED |
| OAuth 回调绕过封禁 | CONFIRMED → ✅ 已修复 |
| SSO UserInfo 缺少黑名单检查 | CONFIRMED → ✅ 已修复 |
| OAuth Provider Revoke 缺少 KV 写入 | CONFIRMED → ✅ 已修复 |
| 测试引用不存在的旧路径 | CONFIRMED → ✅ 已修复 |

---

## 四批审查（2026-06-24）— 一期缺陷修复全量审查

> 审查范围：`b1e113d` (chore: 完成一期代码缺陷修复与优化) — 1651 行 diff，30+ 文件
> 审查方式：8 路并行 AI 审查代理（line-by-line / removed-behavior / cross-file / reuse / simplification / efficiency / altitude / conventions）→ 1-vote verify
> 严重性：🔴 CRITICAL / 🟠 HIGH / 🟡 MEDIUM / 🔵 LOW

### 🟠 HIGH

*(该批次审查中的 HIGH 项已在 b1e113d 及后续 commit 中逐步修复，具体见下方验证状态)*

### 验证状态

| 候选 | 状态 | 说明 |
|------|------|------|
| notification SSE 推送跳过 | CONFIRMED → ⚠️ 待确认 | 需检查 532c10d 后状态 |
| 旧 refresh token 全量拒绝 | CONFIRMED → ⚠️ 待确认 | 需检查 532c10d 后状态 |
| AuthGate fail-closed 全站 500 | CONFIRMED → ⚠️ 待确认 | 需检查 532c10d 后状态 |
| prometheus_test 断言失败 | CONFIRMED → ⚠️ 待确认 | 需检查 532c10d 后状态 |
| /metrics 无权限限制 | CONFIRMED → ⚠️ 待确认 | 需检查 532c10d 后状态 |
| report/content currentUser 遗漏 | CONFIRMED → ⚠️ 待确认 | 需检查 532c10d 后状态 |
| post.Update/Delete currentUserUUID | CONFIRMED → ⚠️ 待确认 | 需检查 532c10d 后状态 |
| OAuth LinkAccount state 破坏 | PLAUSIBLE（需确认前端状态） |
| notification Delete 双倍开销 | CONFIRMED（低影响） |
| validateModerationRule 层级错误 | CONFIRMED（设计问题） |
| CreateRule 客户端 ID 注入 | PLAUSIBLE（取决于 service 层是否覆盖） |
